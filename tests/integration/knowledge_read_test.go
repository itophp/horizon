//go:build integration

package tests

import (
	"strings"
	"testing"

	gencapability "horizon/foundation/gen/capability"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func TestKnowledgeDocumentQueryAndRead(t *testing.T) {
	handler, pool, admin := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	other, err := identity.Bootstrap(t.Context(), pool, "tenant-b", "other-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(pool)
	add := func(tenant string, owner int64, title, content string) int64 {
		t.Helper()
		id, err := q.KBCreateDocument(t.Context(), dbgen.KBCreateDocumentParams{
			TenantID: tenant, CreatedBy: owner, Title: title, Content: content, Source: "manual", Chunks: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	titleMatch := add("tenant-a", admin.ID, "VPN description", "Setup information")
	content := "Internal notes about Vpn availability\nOnly 50% of sites are enabled."
	bodyMatch := add("tenant-a", admin.ID, "Network policy", content)
	foreign := add("tenant-b", other.ID, "VPN procedures", "Another workspace's notes")
	add("tenant-a", admin.ID, "Office hours", "The office opens at eight.")

	for _, username := range []string{"knowledge-reader", "no-knowledge"} {
		overrides := []map[string]any{}
		if username == "knowledge-reader" {
			overrides = append(overrides, map[string]any{"permission": "knowledge:read", "effect": "allow", "scope": "ALL"})
		}
		checkSucceeded(t, root.invoke(t, "identity.account.create", map[string]any{
			"username": username, "fullName": username, "organizationId": admin.OrganizationID,
			"departmentId": admin.DepartmentID, "status": "active", "roleIds": []int64{},
			"overrides": overrides, "password": "TestOnly!2026Secure",
		}, true))
	}
	reader := loginIAM(t, handler, "knowledge-reader")
	restricted := loginIAM(t, handler, "no-knowledge")
	type hit struct {
		ID      int64
		Title   string
		Content *string
	}
	type page struct {
		Documents    []hit
		NextBeforeID *int64
	}
	first := decodeData[page](t, reader.invoke(t, "knowledge.query", map[string]any{"keyword": " vpn ", "limit": 1}, true))
	if len(first.Documents) != 1 || first.Documents[0].ID != bodyMatch || first.Documents[0].Content != nil ||
		first.NextBeforeID == nil || *first.NextBeforeID != bodyMatch {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second := decodeData[page](t, reader.invoke(t, "knowledge.query", map[string]any{"keyword": "vpn", "limit": 1, "beforeId": *first.NextBeforeID}, true))
	if len(second.Documents) != 1 || second.Documents[0].ID != titleMatch || second.NextBeforeID != nil {
		t.Fatalf("unexpected second page: %+v", second)
	}
	literal := decodeData[page](t, reader.invoke(t, "knowledge.query", map[string]any{"keyword": "%"}, true))
	if len(literal.Documents) != 1 || literal.Documents[0].ID != bodyMatch {
		t.Fatalf("expected a literal keyword match: %+v", literal)
	}
	document := decodeData[hit](t, reader.invoke(t, "knowledge.read", map[string]any{"id": bodyMatch}, true))
	if document.Content == nil || *document.Content != content || document.Title != "Network policy" {
		t.Fatalf("the original text and title were not returned")
	}
	outside := decodeBody[gencapability.CapabilityResult](t, reader.invoke(t, "knowledge.read", map[string]any{"id": foreign}, true))
	if outside.Status != "failed" || outside.ErrorCode != "not_found" {
		t.Fatalf("cross-workspace detail returned %s (%s)", outside.Status, outside.ErrorCode)
	}
	for _, name := range []string{"knowledge.query", "knowledge.read"} {
		payload := map[string]any{}
		if name == "knowledge.read" {
			payload["id"] = bodyMatch
		}
		checkStatus(t, restricted.invoke(t, name, payload, true), 403)
	}
	rows, err := pool.Query(t.Context(), "SELECT input::text, COALESCE(result::text, '') FROM cap_invocations WHERE capability IN ('knowledge.query', 'knowledge.read')")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var input, result string
		if err := rows.Scan(&input, &result); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(input, "vpn") || strings.Contains(result, "Internal notes") {
			t.Fatal("knowledge content or query was persisted in the invocation log")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

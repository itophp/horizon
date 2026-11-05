//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	businessmcp "baseline/services/api/integrations/mcp"
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestArticlesMCPSubmission(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	token := decodeData[businessmcp.MCPTokenCreated](t, owner.invoke(t, "mcp.token.create", map[string]any{"name": "article client", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID, "tools": []string{"articles_list", "articles_get", "articles_create", "articles_submit"}}, true))
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := sdk.NewClient(&sdk.Implementation{Name: "article-test", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: workflowBearer{token: token.Secret}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) <= 4 {
		t.Fatalf("tool scope: %v", err)
	}
	available := map[string]bool{}
	for _, tool := range tools.Tools {
		available[tool.Name] = true
	}
	for _, name := range []string{"article_create", "article_get", "article_submit", "dictionary_options_read", "mcp_context_get"} {
		if !available[name] {
			t.Fatalf("authorized capability missing: %s", name)
		}
	}
	if available["articles_create"] {
		t.Fatal("legacy hardcoded tool remains exposed")
	}
	if invalid, e := client.CallTool(ctx, &sdk.CallToolParams{Name: "article_create", Arguments: map[string]any{}}); e == nil && !invalid.IsError {
		t.Fatal("missing required fields accepted")
	}
	call := func(name string, args map[string]any, key string) map[string]any {
		t.Helper()
		res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args, Meta: sdk.Meta{"idempotencyKey": key}})
		if e != nil || res.IsError {
			t.Fatalf("call %s: %v %+v", name, e, res)
		}
		raw, e := json.Marshal(res.StructuredContent)
		if e != nil {
			t.Fatal(e)
		}
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if e = json.Unmarshal(raw, &envelope); e != nil {
			t.Fatal(e)
		}
		return envelope.Data
	}
	create := map[string]any{"title": "MCP article", "body": "private MCP body"}
	first := call("article_create", create, "article-create")
	second := call("article_create", create, "article-create")
	if first["id"] != second["id"] {
		t.Fatal("creation not idempotent")
	}
	detail := call("article_get", map[string]any{"id": first["id"]}, "")
	if detail["status"] != "draft" {
		t.Fatal("draft published")
	}
	call("article_submit", map[string]any{"id": first["id"], "revision": detail["revision"]}, "article-submit")
	detail = call("article_get", map[string]any{"id": first["id"]}, "")
	if detail["status"] != "pending_ai" {
		t.Fatal("MCP bypassed moderation")
	}
	listed := call("article_list", map[string]any{}, "")
	if len(listed["items"].([]any)) != 1 {
		t.Fatal("article missing")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM cap_outbox WHERE event_type='article.submitted'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("submission event %d %v", count, err)
	}
	if res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: "article_decide", Arguments: map[string]any{"id": first["id"], "revision": detail["revision"], "approve": true}}); e == nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var outcome struct {
			Status string `json:"status"`
		}
		if err = json.Unmarshal(raw, &outcome); err != nil {
			t.Fatal(err)
		}
		if outcome.Status == "succeeded" {
			t.Fatal("automated manual review succeeded")
		}
	}
	checkSucceeded(t, owner.invoke(t, "mcp.token.revoke", map[string]any{"id": token.Token.ID, "password": "TestOnly!2026Secure"}, true))
	if _, e := client.ListTools(ctx, nil); e == nil {
		t.Fatal("revoked credential usable")
	}
}

// Discovery and invocation must reflect current department grants on every request.
func TestMCPDiscoveryTracksPermissionChanges(t *testing.T) {
	handler, pool, root := iamFixture(t)
	admin := loginIAM(t, handler, "root-admin")
	created := admin.invoke(t, "identity.account.create", map[string]any{
		"username": "mcp-dynamic", "fullName": "MCP dynamic", "email": "", "status": "active",
		"organizationId": root.OrganizationID, "departmentId": root.DepartmentID,
		"roleIds": []int64{}, "overrides": []map[string]any{{"permission": "articles:read", "effect": "allow", "scope": "OWN"}},
		"password": "TestOnly!2026Secure",
	}, true)
	account := decodeData[genidentity.AccountRecord](t, created)
	user := loginIAM(t, handler, "mcp-dynamic")
	token := decodeData[businessmcp.MCPTokenCreated](t, user.invoke(t, "mcp.token.create", map[string]any{
		"name": "dynamic", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID,
	}, true))
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := sdk.NewClient(&sdk.Implementation{Name: "permission-test", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: workflowBearer{token: token.Secret}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	contains := func(name string) bool {
		t.Helper()
		list, e := client.ListTools(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		for _, tool := range list.Tools {
			if tool.Name == name {
				return true
			}
		}
		return false
	}
	if !contains("article_list") || contains("article_create") {
		t.Fatal("initial discovery ignored grants")
	}
	if res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: "article_create", Arguments: map[string]any{"title": "denied", "body": "denied"}}); e == nil && !res.IsError {
		t.Fatal("unauthorized direct invocation accepted")
	}
	if _, err = pool.Exec(ctx, `UPDATE iam_accounts SET overrides='[]' WHERE tenant_id=$1 AND id=$2`, "tenant-a", account.ID); err != nil {
		t.Fatal(err)
	}
	if contains("article_list") {
		t.Fatal("revoked grant remains discoverable")
	}
	if res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: "article_list", Arguments: map[string]any{}}); e == nil && !res.IsError {
		t.Fatal("revoked capability remains callable")
	}
	if !contains("mcp_context_get") {
		t.Fatal("authenticated context missing after grant revocation")
	}
}

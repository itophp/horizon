//go:build integration

package tests

import (
	gencap "baseline/services/api/gen/capability"
	"baseline/services/api/runtime/app"
	"baseline/services/api/runtime/transport"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRuntimeDefinitionPublishPersistsAndRejectsStaleVersion(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	call := func(name string, input map[string]any) map[string]any {
		t.Helper()
		r := invokeCapability(t, client, name, map[string]any{"input": input, "idempotencyKey": fmt.Sprintf("definition-%s-%d", name, time.Now().UnixNano())})
		if r.Status != "succeeded" {
			t.Fatal(name, r)
		}
		return r.Data.(map[string]any)
	}
	base := call("application.runtime.definition.get", map[string]any{"applicationId": "projects"})
	if base["version"] != "1.0.0.0" {
		t.Fatal(base)
	}
	navigation := map[string]any{"pageId": "project.list", "labelKey": "projects:heading", "icon": "boxes"}
	raw, err := json.Marshal(navigation)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"applicationId": "projects", "baseVersion": base["version"], "revision": base["revision"], "changes": []any{map[string]any{"section": "navigation", "id": "project.list", "operation": "upsert", "value": string(raw)}}}
	candidate := call("application.runtime.definition.propose", input)
	if candidate["version"] != "1.0.0.1" {
		t.Fatal(candidate)
	}
	// Saving a candidate must leave the published application unchanged.
	active := call("application.runtime.resolve", map[string]any{"applicationId": "projects"})
	if active["applicationVersion"] != "1.0.0.0" {
		t.Fatal(active)
	}
	proposal := call("application.runtime.definition.get", map[string]any{"applicationId": "projects"})
	call("application.runtime.definition.publish", map[string]any{"applicationId": "projects", "version": proposal["version"], "digest": proposal["digest"], "revision": proposal["revision"]})
	active = call("application.runtime.resolve", map[string]any{"applicationId": "projects"})
	if active["applicationVersion"] != "1.0.0.1" {
		t.Fatal(active)
	}
	var count int
	if err = pool.QueryRow(t.Context(), `SELECT count(*) FROM runtime_application_versions WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	// Rebuild the actual HTTP composition root against the same database. Startup
	// reconciliation must retain the published customization and version count.
	iam := identity.New(pool, false)
	restarted := transport.HandlerWithCore(app.System{}, governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")}), iam)
	again := loginIAM(t, restarted, "root-admin")
	restored := invokeCapability(t, again, "application.runtime.resolve", map[string]any{"input": map[string]any{"applicationId": "projects"}})
	if restored.Status != "succeeded" || restored.Data.(map[string]any)["applicationVersion"] != "1.0.0.1" {
		t.Fatal(restored)
	}
	if err = pool.QueryRow(t.Context(), `SELECT count(*) FROM runtime_application_versions WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	// Tenants created after startup use the explicit deployment template in the
	// bootstrap transaction; they do not need a missing-record runtime fallback.
	if _, err = identity.Bootstrap(t.Context(), pool, "tenant-runtime-new", "runtime-new-admin", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(t.Context(), `SELECT count(*) FROM runtime_application_heads WHERE tenant_id='tenant-runtime-new' AND published_version='1.0.0.0'`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	// Even SQL cannot point this new tenant at a version owned only by tenant-a.
	if _, err = pool.Exec(t.Context(), `UPDATE runtime_application_heads SET published_version='1.0.0.1' WHERE tenant_id='tenant-runtime-new' AND application_id='projects'`); err == nil {
		t.Fatal("cross tenant version pointer accepted")
	}
	old := call("application.runtime.definition.get", map[string]any{"applicationId": "projects", "version": "1.0.0.0"})
	current := call("application.runtime.definition.get", map[string]any{"applicationId": "projects"})
	call("application.runtime.definition.rollback", map[string]any{"applicationId": "projects", "version": old["version"], "digest": old["digest"], "revision": current["revision"]})
	rolled := call("application.runtime.resolve", map[string]any{"applicationId": "projects"})
	if rolled["applicationVersion"] != "1.0.0.2" {
		t.Fatal(rolled)
	}
	// A different operation with the stale revision cannot overwrite the newer candidate.
	response := decodeBody[gencap.CapabilityResult](t, client.invoke(t, "application.runtime.definition.propose", input, true))
	if response.Status != "failed" || response.ErrorCode != "conflict" {
		t.Fatal(response)
	}
}

func TestRuntimeDynamicApplicationStartsDisabled(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	call := func(name string, input map[string]any) map[string]any {
		t.Helper()
		r := invokeCapability(t, client, name, map[string]any{"input": input, "idempotencyKey": fmt.Sprintf("dynamic-%d", time.Now().UnixNano())})
		if r.Status != "succeeded" {
			t.Fatal(name, r)
		}
		return r.Data.(map[string]any)
	}
	original := call("application.runtime.definition.get", map[string]any{"applicationId": "projects"})
	raw, err := json.Marshal(original["application"])
	if err != nil {
		t.Fatal(err)
	}
	var blueprint map[string]any
	if err = json.Unmarshal(raw, &blueprint); err != nil {
		t.Fatal(err)
	}
	for _, item := range blueprint["pages"].([]any) {
		page := item.(map[string]any)
		old := page["route"].(string)
		page["route"] = "/apps/operations/" + strings.ReplaceAll(page["id"].(string), ".", "-")
		if strings.Contains(old, ":id") {
			page["route"] = page["route"].(string) + "/:id"
		}
	}
	raw, err = json.Marshal(blueprint)
	if err != nil {
		t.Fatal(err)
	}
	created := call("application.runtime.definition.create", map[string]any{"slug": "operations", "definition": string(raw)})
	id := created["applicationId"]
	candidate := call("application.runtime.definition.get", map[string]any{"applicationId": id})
	if candidate["version"] != "0.1.0.1" {
		t.Fatal(candidate)
	}
	call("application.runtime.definition.publish", map[string]any{"applicationId": id, "version": candidate["version"], "digest": candidate["digest"], "revision": candidate["revision"]})
	hidden := decodeBody[gencap.CapabilityResult](t, client.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": id}, true))
	if hidden.Status != "failed" || hidden.ErrorCode != "not_found" {
		t.Fatal(hidden)
	}
	call("application.runtime.application.enable", map[string]any{"pluginId": "builtin:runtime-host", "applicationId": id, "revision": 2})
	active := call("application.runtime.resolve", map[string]any{"applicationId": id})
	if active["route"] != "/apps/operations" {
		t.Fatal(active)
	}
	// The database itself enforces append-only version history.
	if _, err = pool.Exec(t.Context(), `UPDATE runtime_application_versions SET digest='changed' WHERE tenant_id='tenant-a' AND application_id=$1`, id); err == nil {
		t.Fatal("immutable definition updated")
	}
}

func TestRuntimeDefinitionIdempotencyAndInvalidChanges(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	base := invokeCapability(t, client, "application.runtime.definition.get", map[string]any{"input": map[string]any{"applicationId": "projects"}}).Data.(map[string]any)
	input := map[string]any{"applicationId": "projects", "baseVersion": base["version"], "revision": base["revision"], "changes": []any{map[string]any{"section": "navigation", "id": "project.list", "operation": "upsert", "value": `{"pageId":"project.list","labelKey":"projects:heading","icon":"layers"}`}}}
	first := invokeCapability(t, client, "application.runtime.definition.propose", map[string]any{"input": input, "idempotencyKey": "stable-proposal"})
	second := invokeCapability(t, client, "application.runtime.definition.propose", map[string]any{"input": input, "idempotencyKey": "stable-proposal"})
	if first.Status != "succeeded" || second.Status != "succeeded" || first.InvocationID != second.InvocationID || !second.Replayed {
		t.Fatal(first, second)
	}
	current := invokeCapability(t, client, "application.runtime.definition.get", map[string]any{"input": map[string]any{"applicationId": "projects"}}).Data.(map[string]any)
	invalid := map[string]any{"applicationId": "projects", "baseVersion": current["version"], "revision": current["revision"], "changes": []any{map[string]any{"section": "source", "id": "owner", "operation": "upsert", "value": `"untrusted"`}}}
	rejected := invokeCapability(t, client, "application.runtime.definition.propose", map[string]any{"input": invalid, "idempotencyKey": "invalid-proposal"})
	if rejected.Status != "failed" {
		t.Fatal(rejected)
	}
	var count int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM runtime_application_versions WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	var next int64
	if err := pool.QueryRow(t.Context(), `SELECT next_configuration FROM runtime_application_heads WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&next); err != nil || next != 2 {
		t.Fatal(next, err)
	}
}

func TestRuntimeDefinitionConcurrentProposals(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	base := invokeCapability(t, client, "application.runtime.definition.get", map[string]any{"input": map[string]any{"applicationId": "projects"}}).Data.(map[string]any)
	input := map[string]any{"applicationId": "projects", "baseVersion": base["version"], "revision": base["revision"], "changes": []any{map[string]any{"section": "navigation", "id": "project.list", "operation": "upsert", "value": `{"pageId":"project.list","labelKey":"projects:heading","icon":"layers"}`}}}
	start := make(chan struct{})
	results := make(chan gencap.CapabilityResult, 2)
	for i := range 2 {
		go func() {
			<-start
			results <- invokeCapability(t, client, "application.runtime.definition.propose", map[string]any{"input": input, "idempotencyKey": fmt.Sprintf("concurrent-proposal-%d", i)})
		}()
	}
	close(start)
	succeeded, conflicted := 0, 0
	for range 2 {
		r := <-results
		if r.Status == "succeeded" {
			succeeded++
		} else if r.ErrorCode == "conflict" {
			conflicted++
		} else {
			t.Fatal(r)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatal(succeeded, conflicted)
	}
	var count, next int
	if err := pool.QueryRow(t.Context(), `SELECT count(*), max(h.next_configuration) FROM runtime_application_versions v JOIN runtime_application_heads h USING(tenant_id, application_id) WHERE v.tenant_id='tenant-a' AND v.application_id='projects'`).Scan(&count, &next); err != nil || count != 2 || next != 2 {
		t.Fatal(count, next, err)
	}
}

func TestRuntimeDesignContractDiscovery(t *testing.T) {
	handler, _, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	get := func(input map[string]any) gencap.CapabilityResult {
		return invokeCapability(t, client, "application.runtime.definition.contracts", map[string]any{"input": input})
	}
	result := get(map[string]any{"kind": "capability", "name": "project.create@1.0.0"})
	if result.Status != "succeeded" || result.Data.(map[string]any)["contract"].(map[string]any)["input"] == nil {
		t.Fatal(result)
	}
	for _, name := range []string{"application.runtime.catalog.reconcile@1.0.0", "application.runtime.definition.publish@1.0.0"} {
		if r := get(map[string]any{"kind": "capability", "name": name}); r.Status != "failed" || r.ErrorCode != "not_found" {
			t.Fatal(name, r)
		}
	}
	if r := get(map[string]any{"kind": "component", "name": "money@2.0"}); r.Status != "succeeded" {
		t.Fatal(r)
	}
}

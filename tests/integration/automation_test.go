//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"

	"horizon/foundation/ai/automation"
	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

// TestAutomationCapabilities covers the scope boundary that matters most for a
// private scheduled agent: only the owner sees or changes a task, a foreign
// account and a foreign tenant get not_found, replay is idempotent, and only
// creation is exposed to automated callers.
func TestAutomationCapabilities(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	module := automation.New(pool)
	catalog := registry.New(module.Entries())
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	catalog.RegisterEntities(automation.Entities())
	g := gateway.New(gateway.Options{
		Registry: catalog, Identity: iam, Pool: pool,
		Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool),
	})
	owner, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ownerCtx := identity.WithPrincipal(ctx, owner)

	create := func() *contracts.Result {
		t.Helper()
		result, err := g.Invoke(ownerCtx, gateway.Request{
			Name: "automation.task.create", Version: "1.0.0",
			Input: contracts.Input{
				"name": "Daily overdue report", "description": "Summarise overdue project tasks and send them to me.",
				"scheduleKind": "daily", "time": "09:00", "timezone": "Asia/Shanghai",
			},
			Origin: contracts.OriginHumanUI, IdempotencyKey: "automation-create-1",
		})
		if err != nil || result.Status != contracts.StatusSucceeded {
			t.Fatalf("create: %+v %v", result, err)
		}
		return result
	}
	result := create()
	// A repeated logical create replays the stored result instead of adding a row.
	if replay := create(); !replay.Replayed {
		t.Fatal("create replay was not reported")
	}
	var created struct {
		ID int64 `json:"id"`
	}
	raw, _ := json.Marshal(result.Data)
	if err = json.Unmarshal(raw, &created); err != nil || created.ID == 0 {
		t.Fatalf("created id: %s %v", raw, err)
	}

	list := func(p *identity.Principal) map[string]any {
		t.Helper()
		result, err := g.Invoke(identity.WithPrincipal(ctx, p), gateway.Request{
			Name: "automation.task.list", Version: "1.0.0", Input: contracts.Input{},
			Origin: contracts.OriginHumanUI,
		})
		if err != nil || result.Status != contracts.StatusSucceeded {
			t.Fatalf("list: %+v %v", result, err)
		}
		value, _ := json.Marshal(result.Data)
		var page map[string]any
		if err = json.Unmarshal(value, &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	if total := list(owner)["total"].(float64); total != 1 {
		t.Fatalf("owner total = %v", total)
	}

	grants := func(permission string) *genidentity.PermissionGrant {
		return &genidentity.PermissionGrant{Permission: permission, Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}
	}
	overrides, err := json.Marshal([]*genidentity.PermissionGrant{grants("automation:read"), grants("automation:write")})
	if err != nil {
		t.Fatal(err)
	}
	// A real second account is required: the gateway re-resolves every caller
	// from the database, so a fabricated principal would only test rejection.
	foreignID, err := dbgen.New(pool).IAMCreateAccount(ctx, dbgen.IAMCreateAccountParams{
		TenantID: "tenant-a", Username: "foreign-automation", FullName: "Foreign",
		Status: "active", OrganizationID: root.OrganizationID, DepartmentID: root.DepartmentID,
		Overrides: overrides, PasswordHash: "unusable",
	})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := iam.PrincipalForTask(ctx, "tenant-a", foreignID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	if total := list(foreign)["total"].(float64); total != 0 {
		t.Fatal("another account saw the task")
	}
	otherRoot, err := identity.Bootstrap(ctx, pool, "tenant-b", "root-b", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	otherTenant, err := iam.PrincipalForTask(ctx, "tenant-b", otherRoot.ID, otherRoot.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	if total := list(otherTenant)["total"].(float64); total != 0 {
		t.Fatal("another tenant saw the task")
	}
	denied := func(name string, input contracts.Input, key string) bool {
		t.Helper()
		result, err := g.Invoke(identity.WithPrincipal(ctx, foreign), gateway.Request{
			Name: name, Version: "1.0.0", Input: input, Origin: contracts.OriginHumanUI, IdempotencyKey: key,
		})
		return err != nil || result.Status != contracts.StatusSucceeded
	}
	if !denied("automation.task.get", contracts.Input{"id": created.ID}, "") {
		t.Fatal("automation.task.get exposed a foreign task")
	}
	if !denied("automation.task.update", contracts.Input{"id": created.ID, "name": "hijack", "description": "hijack", "scheduleKind": "once", "runAt": "2026-12-02T09:00", "timezone": "UTC"}, "foreign-update") {
		t.Fatal("a foreign account updated the task")
	}
	if !denied("automation.run.now", contracts.Input{"id": created.ID}, "foreign-run") {
		t.Fatal("a foreign account triggered the task")
	}

	// Manual runs are deduplicated by the caller's idempotency key and are not
	// reachable from an agent origin.
	run := func(origin contracts.Origin, key string) *contracts.Result {
		t.Helper()
		result, err := g.Invoke(ownerCtx, gateway.Request{
			Name: "automation.run.now", Version: "1.0.0", Input: contracts.Input{"id": created.ID},
			Origin: origin, IdempotencyKey: key,
		})
		if err != nil {
			t.Fatalf("run now (%s): %v", origin, err)
		}
		return result
	}
	first := run(contracts.OriginHumanUI, "manual-1")
	if replay := run(contracts.OriginHumanUI, "manual-1"); !replay.Replayed {
		t.Fatal("manual run replay was not reported")
	}
	var manual struct {
		ID int64 `json:"id"`
	}
	raw, _ = json.Marshal(first.Data)
	if err = json.Unmarshal(raw, &manual); err != nil || manual.ID == 0 {
		t.Fatalf("manual run id: %s %v", raw, err)
	}
	if result, err := g.Invoke(ownerCtx, gateway.Request{
		Name: "automation.run.now", Version: "1.0.0", Input: contracts.Input{"id": created.ID},
		Origin: contracts.OriginAgent, IdempotencyKey: "manual-agent",
	}); err == nil && result.Status == contracts.StatusSucceeded {
		t.Fatal("manual-only run capability accepted an agent origin")
	}

	// Only creation is exposed to automated callers; the rest stays page-only.
	caller := contracts.Caller{Grants: []string{"automation:write", "automation:read"}}
	discovered := map[string]bool{}
	for _, definition := range catalog.DiscoverAutomated(caller) {
		discovered[definition.Name] = true
	}
	if !discovered["automation.task.create"] {
		t.Fatal("agent creation is not discoverable")
	}
	for _, name := range []string{"automation.task.update", "automation.task.delete", "automation.run.now", "automation.task.list"} {
		if discovered[name] {
			t.Fatalf("%s must stay manual-only", name)
		}
	}
}

//go:build integration

package tests

import (
	"baseline/services/api/db/migrations"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"sync"
	"testing"

	gen "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/executionplan"
	"baseline/services/api/system/executionplans"
	"baseline/services/api/system/identity"
)

func TestExecutionPlanLifecycleAndIsolation(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	d := contracts.Definition{Name: "test.execution.read", Version: "1.0.0", Title: "Test", Description: "Test native execution", Domain: "example", Entity: "example_record", Layer: contracts.LayerAtomic, Kind: contracts.KindService, Permission: "example:read", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm, ReadOnly: true, Input: contracts.Object(map[string]*contracts.Schema{"fail": {Type: "boolean"}}), Executor: "go:test"}
	m := &executionplans.Module{Pool: pool, Definitions: map[string]contracts.Definition{d.Key(): d}}
	entries := append(m.Entries(), registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
		if in["fail"] == true {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		meta, ok := gateway.CurrentInvocation(ctx)
		if !ok {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		return contracts.Output{Data: map[string]any{"binding": meta.ExecutionBinding}}, nil
	}}})
	for _, entry := range entries {
		m.Definitions[entry.Definition.Key()] = entry.Definition
	}
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Pool: pool, Registry: catalog, Identity: iam, Approvals: approval.NewStore(pool)})
	q := dbgen.New(pool)
	call := func(action string, in contracts.Input) *contracts.Result {
		t.Helper()
		r, e := g.Invoke(ctx, gateway.Request{Name: "execution.plan." + action, Input: in, Origin: contracts.OriginHumanUI, IdempotencyKey: fmt.Sprint(action, "-", in)})
		if e != nil || r.Status != contracts.StatusSucceeded {
			t.Fatalf("%s: %+v %v", action, r, e)
		}
		return r
	}
	input := func(id string, v int, revision int) contracts.Input {
		return contracts.Input{"capability": d.Name, "capabilityVersion": d.Version, "planId": id, "planVersion": v, "expectedRevision": revision, "reason": "test change"}
	}
	target := contracts.Input{"capability": d.Name, "capabilityVersion": d.Version}
	call("list", target)
	platform := contracts.DefaultExecutionBinding(d)
	persisted, err := q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{PlanID: platform.Plan.PlanID, Version: 1})
	if err != nil || persisted.Mode != "direct" {
		t.Fatal(persisted, err)
	}
	draft := contracts.Input{"capability": d.Name, "capabilityVersion": d.Version, "planId": "custom-release", "name": "Custom release"}
	created := call("create", draft)
	// Idempotent replay must not append another draft version.
	if again := call("create", draft); again.InvocationID != created.InvocationID {
		t.Fatal("create replay changed invocation")
	}
	planInput := contracts.Input{"capability": d.Name, "capabilityVersion": d.Version, "planId": "custom-release", "planVersion": 1}
	if r, e := g.Invoke(ctx, gateway.Request{Name: "execution.plan.bind", Input: input("custom-release", 1, 0), IdempotencyKey: "draft-bind", Origin: contracts.OriginHumanUI}); e != nil || r.Status != contracts.StatusFailed {
		t.Fatal("draft bound", r, e)
	}
	call("validate", planInput)
	call("publish", planInput)
	// Mutating a published version is rejected by the real database.
	if _, err = pool.Exec(ctx, `UPDATE cap_execution_plans SET name='mutated' WHERE tenant_id='tenant-a' AND plan_id='custom-release'`); err == nil {
		t.Fatal("published definition mutated")
	}
	call("bind", input("custom-release", 1, 0))
	resolved, err := executionplan.Resolve(ctx, q, "tenant-a", d)
	if err != nil || resolved.Plan.PlanID != "custom-release" || resolved.Revision != 1 {
		t.Fatal(resolved, err)
	}
	// Platform fallback and tenant isolation are SQL-enforced, even with the same ID.
	other, err := executionplan.Resolve(ctx, q, "tenant-b", d)
	if err != nil || other != platform {
		t.Fatal(other, err)
	}
	if _, err = q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: "tenant-b", PlanID: "custom-release", Version: 1}); err == nil {
		t.Fatal("foreign plan visible")
	}
	waiting, err := g.Invoke(ctx, gateway.Request{Name: d.Name, Input: contracts.Input{}, Origin: contracts.OriginAPI, IdempotencyKey: "waiting"})
	if err != nil || waiting.Status != contracts.StatusAwaitingApproval {
		t.Fatal(waiting, err)
	}
	raw, err := q.ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", InvocationID: waiting.InvocationID, AccountID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	var pinned contracts.ExecutionBinding
	if err = json.Unmarshal(raw, &pinned); err != nil || pinned.Plan.PlanID != "custom-release" {
		t.Fatal(pinned, err)
	}
	call("rollback", input(platform.Plan.PlanID, 1, 1))
	completed, err := g.Decide(ctx, waiting.ApprovalID, true, "")
	if err != nil || completed.Status != contracts.StatusSucceeded {
		t.Fatal(completed, err)
	}
	if completed.Data.(map[string]any)["binding"].(contracts.ExecutionBinding) != pinned {
		t.Fatal("approval used new binding")
	}
	failed, err := g.Invoke(ctx, gateway.Request{Name: d.Name, Input: contracts.Input{"fail": true}, Origin: contracts.OriginHumanUI})
	if err != nil || failed.Status != contracts.StatusFailed {
		t.Fatal(failed, err)
	}
	raw, err = q.ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", InvocationID: failed.InvocationID, AccountID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = executionplan.Decode(raw, d); err != nil {
		t.Fatal(err)
	}
	if _, err = q.ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", InvocationID: failed.InvocationID, AccountID: root.ID + 1}); err == nil {
		t.Fatal("other account read execution record")
	}
	// Concurrent binding switches cannot silently overwrite one another.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, e := g.Invoke(ctx, gateway.Request{Name: "execution.plan.bind", Input: input("custom-release", 1, 2), IdempotencyKey: fmt.Sprint("race-", i), Origin: contracts.OriginHumanUI})
			if e == nil && r.Status != contracts.StatusSucceeded {
				e = contracts.Problem(r.ErrorCode)
			}
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if contracts.Code(e) != "conflict" {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatalf("binding race winners: %d", success)
	}
	history, err := q.ExecutionBindingHistory(ctx, dbgen.ExecutionBindingHistoryParams{TenantID: "tenant-a", Capability: d.Name, CapabilityVersion: d.Version})
	if err != nil || len(history) != 3 {
		t.Fatal(history, err)
	}
	// Management operations and corresponding success events commit together.
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND event_type IN ('execution.plan.bind','execution.plan.rollback')`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	// Unauthorized callers cannot discover or invoke management actions.
	limited, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-a", []*gen.PermissionGrant{{Permission: "example:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	restricted := gateway.New(gateway.Options{Pool: pool, Registry: catalog})
	if _, err = restricted.Invoke(identity.WithPrincipal(context.Background(), limited), gateway.Request{Name: "execution.plan.list", Input: target, Origin: contracts.OriginHumanUI}); contracts.Code(err) != "forbidden" {
		t.Fatal("missing management permission accepted", err)
	}
	for _, def := range catalog.DiscoverAutomated(contracts.Caller{Admin: true}) {
		if def.Domain == "executionplans" {
			t.Fatal("management exposed to automation")
		}
	}

	editor, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-a", []*gen.PermissionGrant{{Permission: "example:read", Effect: "allow"}, {Permission: "executionPlans:edit", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restricted.Invoke(identity.WithPrincipal(context.Background(), editor), gateway.Request{Name: "execution.plan.publish", Input: planInput, Origin: contracts.OriginHumanUI, IdempotencyKey: "editor-publish"}); contracts.Code(err) != "forbidden" {
		t.Fatal("editor obtained publish permission", err)
	}
	foreign, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-b", []*gen.PermissionGrant{{Permission: "example:read", Effect: "allow"}, {Permission: "executionPlans:bind", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	foreignResult, err := restricted.Invoke(identity.WithPrincipal(context.Background(), foreign), gateway.Request{Name: "execution.plan.bind", Input: input("custom-release", 1, 0), Origin: contracts.OriginHumanUI, IdempotencyKey: "foreign-bind"})
	if err != nil || foreignResult.Status != contracts.StatusFailed || foreignResult.ErrorCode != "not_found" {
		t.Fatal("foreign tenant bound plan", foreignResult, err)
	}
	call("records", contracts.Input{})
}

// The migration must pin existing approvals before the new runtime handles them.
func TestExecutionPlanMigrationPinsLegacyInvocations(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 28); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO cap_invocations(tenant_id,account_id,invocation_id,capability,capability_version,origin,status,input_digest,trace_id) VALUES('tenant-a',$1,'legacy-wait','example.record.list','1.0.0','api','awaiting_approval','digest','trace-legacy')`, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = provider.Up(ctx); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := dbgen.New(pool).ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", InvocationID: "legacy-wait", AccountID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	d := contracts.Definition{Name: "example.record.list", Version: "1.0.0"}
	binding, err := executionplan.Decode(raw, d)
	if err != nil || binding != contracts.DefaultExecutionBinding(d) {
		t.Fatal(binding, err)
	}
	// Down and Up preserve the legacy business invocation and regenerate metadata.
	if _, err = provider.DownTo(ctx, 28); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = dbgen.New(pool).ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", InvocationID: "legacy-wait", AccountID: root.ID}); err != nil {
		t.Fatal(err)
	}
}

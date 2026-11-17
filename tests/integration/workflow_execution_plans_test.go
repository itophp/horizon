//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"horizon/applications/example"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	runtime "horizon/foundation/kernel/workflow"
	"horizon/foundation/kernel/workflow/spec"
	"horizon/foundation/system/executionplans"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	orchmodule "horizon/foundation/system/orchestration"
	"horizon/internal/repository/dbgen"
)

func TestWorkflowExecutionPlanApprovalRollbackAndRestart(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	conditional := spec.ExampleArchivePlan()
	conditional.Name = "conditional-archive"
	native := conditional.Steps[1]
	native.DependsOn = nil
	native.When = &runtime.When{From: "check", Field: "status", Equals: "draft"}
	native.ID = "draft"
	second := native
	second.ID = "archived"
	second.When = &runtime.When{From: "check", Field: "status", Equals: "archived"}
	conditional.Steps = []runtime.Step{conditional.Steps[0], native, second}
	definitions := []runtime.Definition{spec.ExampleArchivePlan(), conditional}
	engine := runtime.New(core, definitions)
	plans := &executionplans.Module{Pool: pool, Definitions: map[string]contracts.Definition{}, ValidateWorkflow: engine.ValidatePlanWithQueries}
	entries := append(example.New(pool).Entries(), plans.Entries()...)
	entries = append(entries, (&orchmodule.Module{Engine: engine}).Entries()...)
	for _, entry := range entries {
		plans.Definitions[entry.Definition.Key()] = entry.Definition
	}
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	g := gateway.New(gateway.Options{Pool: pool, Registry: catalog, Identity: iam, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool)})
	if err := engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	invoke := func(name, key string, input contracts.Input) *contracts.Result {
		t.Helper()
		r, e := g.Invoke(ctx, gateway.Request{Name: name, IdempotencyKey: key, Input: input, Origin: contracts.OriginHumanUI})
		if e != nil {
			t.Fatal(name, e)
		}
		return r
	}
	created := invoke("example.record.create", "record", contracts.Input{"title": "Native workflow result"})
	if created.Status != contracts.StatusSucceeded {
		t.Fatal(created)
	}
	recordID := created.Data.(map[string]any)["id"]
	target := func(extra contracts.Input) contracts.Input {
		out := contracts.Input{"capability": "example.record.archive", "capabilityVersion": "1.0.0"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	must := func(action string, in contracts.Input) {
		t.Helper()
		r := invoke("execution.plan."+action, "plan-"+action, in)
		if r.Status != contracts.StatusSucceeded {
			t.Fatal(action, r)
		}
	}
	must("create", target(contracts.Input{"planId": "archive-flow", "name": "Archive with check", "mode": "workflow", "workflowRef": "conditional-archive@1.0.0"}))
	for _, action := range []string{"validate", "publish"} {
		must(action, target(contracts.Input{"planId": "archive-flow", "planVersion": 1}))
	}
	must("bind", target(contracts.Input{"planId": "archive-flow", "planVersion": 1, "expectedRevision": 0, "reason": "test workflow"}))
	started := invoke("example.record.archive", "archive-flow-run", contracts.Input{"id": recordID})
	if started.Status != contracts.StatusExecuting || started.Data != nil {
		t.Fatal("async receipt changed business output", started)
	}
	replayedStart := invoke("example.record.archive", "archive-flow-run", contracts.Input{"id": recordID})
	if !replayedStart.Replayed || replayedStart.InvocationID != started.InvocationID || replayedStart.Status != contracts.StatusExecuting {
		t.Fatal("accepted run was duplicated", replayedStart)
	}
	q := dbgen.New(pool)
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	cards, err := q.CapListApprovals(ctx, dbgen.CapListApprovalsParams{TenantID: "tenant-a", AccountID: root.ID})
	if err != nil || len(cards) != 1 {
		t.Fatal(cards, err)
	}
	// Native calls are still automated high-risk actions requiring approval.
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM example_records WHERE tenant_id='tenant-a' AND id=$1`, recordID).Scan(&status); err != nil || status != "draft" {
		t.Fatal(status, err)
	}
	platform := contracts.DefaultExecutionBinding(plans.Definitions["example.record.archive@1.0.0"])
	must("rollback", target(contracts.Input{"planId": platform.Plan.PlanID, "planVersion": 1, "expectedRevision": 1, "reason": "rollback during approval"}))
	done, err := g.Decide(ctx, cards[0].ID, true, "")
	if err != nil || done.Status != contracts.StatusSucceeded {
		t.Fatal(done, err)
	}
	// Reconstruct the real Eino engine and resume the same durable run.
	engine = runtime.New(core, definitions)
	if err = engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	var run pgtype.Int8
	if err = pool.QueryRow(ctx, `SELECT run_id FROM cap_execution_records WHERE tenant_id='tenant-a' AND invocation_id=$1`, started.InvocationID).Scan(&run); err != nil || !run.Valid {
		t.Fatal(run, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE orch_checkpoints SET lease_until=now()-interval '1 second' WHERE tenant_id='tenant-a' AND run_id=$1`, run); err != nil {
		t.Fatal(err)
	}
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	row, err := q.CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: "tenant-a", AccountID: root.ID, InvocationID: started.InvocationID})
	if err != nil || row.Status != "succeeded" {
		t.Fatal(row, err)
	}
	var result struct {
		Data map[string]any `json:"data"`
	}
	if err = json.Unmarshal(row.Result, &result); err != nil || result.Data["status"] != "archived" {
		t.Fatal(result, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_outbox WHERE event_type='example.record.archived'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("business event duplicated", count, err)
	}
	replay := invoke("example.record.archive", "archive-flow-run", contracts.Input{"id": recordID})
	if !replay.Replayed || replay.Status != contracts.StatusSucceeded {
		t.Fatal(replay)
	}
	// The completed outer call preserves the native event identifiers.
	if len(replay.EventIDs) != 1 {
		t.Fatal("native event identities lost", replay)
	}
	plans.ValidateWorkflow = engine.ValidatePlanWithQueries
	// A pinned Workflow is selected even though the active binding is now Direct.
	fresh := invoke("example.record.create", "cancel-record", contracts.Input{"title": "Cancel workflow"})
	cancelID := fresh.Data.(map[string]any)["id"]
	pending, err := g.InvokePinned(ctx, gateway.Request{Name: "example.record.archive", Version: "1.0.0", Input: contracts.Input{"id": cancelID}, IdempotencyKey: "pinned-workflow", Origin: contracts.OriginHumanUI}, "archive-flow", 1)
	if err != nil || pending.Status != contracts.StatusExecuting {
		t.Fatal(pending, err)
	}
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelRun pgtype.Int8
	if err = pool.QueryRow(ctx, `SELECT run_id FROM cap_execution_records WHERE tenant_id='tenant-a' AND invocation_id=$1`, pending.InvocationID).Scan(&cancelRun); err != nil {
		t.Fatal(err)
	}
	cancelled := invoke("orchestration.workflow.cancel", "cancel-plan", contracts.Input{"runId": cancelRun.Int64})
	if cancelled.Status != contracts.StatusSucceeded {
		t.Fatal(cancelled)
	}
	cancelledRow, err := q.CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: "tenant-a", AccountID: root.ID, InvocationID: pending.InvocationID})
	if err != nil || cancelledRow.Status != "cancelled" {
		t.Fatal(cancelledRow, err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM example_records WHERE tenant_id='tenant-a' AND id=$1`, cancelID).Scan(&status); err != nil || status != "draft" {
		t.Fatal("cancel committed native action", status, err)
	}
	// A failed prerequisite closes the original invocation without native effects.
	failed, err := g.InvokePinned(ctx, gateway.Request{Name: "example.record.archive", Version: "1.0.0", Input: contracts.Input{"id": 999999}, IdempotencyKey: "failed-plan", Origin: contracts.OriginHumanUI}, "archive-flow", 1)
	if err != nil || failed.Status != contracts.StatusExecuting {
		t.Fatal(failed, err)
	}
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	failedRow, err := q.CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: "tenant-a", AccountID: root.ID, InvocationID: failed.InvocationID})
	if err != nil || failedRow.Status != "failed" {
		t.Fatal(failedRow, err)
	}
	// Self active-plan recursion is rejected before publication.
	bad := conditional
	bad.Name = "recursive-archive"
	bad.Steps[1].CallMode = "resolve_active_plan"
	rejectedEngine := runtime.New(core, []runtime.Definition{bad})
	if err = rejectedEngine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	if _, err = rejectedEngine.ValidatePlanReference(ctx, plans.Definitions["example.record.archive@1.0.0"], "recursive-archive@1.0.0"); err == nil {
		t.Fatal("recursive plan accepted")
	}
	// Restore the engine after the negative adapter registration.
	if err = engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	// Ordinary callers cannot select native mode or escape the run/owner proof.
	if _, err = g.InvokeNative(ctx, gateway.Request{Name: "example.record.archive", Version: "1.0.0", Input: contracts.Input{"id": recordID}, Origin: contracts.OriginHumanUI}, started.InvocationID); contracts.Code(err) != "forbidden" {
		t.Fatal("native escape accepted", err)
	}
	// A pinned Direct version remains selectable after the active binding changes.
	pinned, err := g.InvokePinned(ctx, gateway.Request{Name: "example.record.get", Version: "1.0.0", Input: contracts.Input{"id": recordID}, Origin: contracts.OriginHumanUI}, "example.record.get@1.0.0.default", 1)
	// Ensure the deployment fallback exists for a reference not invoked yet.
	if err != nil {
		t.Fatal(fmt.Sprint("pinned read: ", err))
	}
	if pinned.Status != contracts.StatusSucceeded {
		t.Fatal(pinned)
	}
}

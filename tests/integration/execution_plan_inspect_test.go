//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"

	"horizon/applications/example"
	gen "horizon/foundation/gen/identity"
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
)

func TestExecutionPlanInspectionAndTraceIsolation(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	engine := runtime.NewWithGraphs(core, []runtime.Definition{spec.ExampleArchivePlan()}, spec.BuiltinGraphs())
	plans := &executionplans.Module{Pool: pool, Definitions: map[string]contracts.Definition{}, ValidateWorkflow: engine.ValidatePlanWithQueries, ValidateGraph: engine.ValidateGraphPlan, References: engine.PlanReferences, Inspect: engine.InspectPlanReference}
	entries := append(example.New(pool).Entries(), plans.Entries()...)
	for _, e := range entries {
		plans.Definitions[e.Definition.Key()] = e.Definition
	}
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
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
	invoke := func(name string, in contracts.Input) *contracts.Result {
		t.Helper()
		key := ""
		if !plans.Definitions[name+"@1.0.0"].ReadOnly {
			key = fmt.Sprintf("inspect-%s-%v", name, in)
		}
		r, e := g.Invoke(ctx, gateway.Request{Name: name, Input: in, IdempotencyKey: key, Origin: contracts.OriginHumanUI})
		if e != nil {
			t.Fatal(name, e)
		}
		if r.Status == contracts.StatusSucceeded {
			wire, e := json.Marshal(r.Data)
			if e != nil {
				t.Fatal(e)
			}
			var object contracts.Input
			if e = json.Unmarshal(wire, &object); e != nil {
				t.Fatal(e)
			}
			if e = plans.Definitions[name+"@1.0.0"].Output.Validate(object); e != nil {
				t.Fatalf("%s output: %s %v", name, wire, e)
			}
		}
		return r
	}
	target := func(extra contracts.Input) contracts.Input {
		out := contracts.Input{"capability": "example.record.archive", "capabilityVersion": "1.0.0"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	must := func(action string, in contracts.Input) *contracts.Result {
		t.Helper()
		r := invoke("execution.plan."+action, in)
		if r.Status != contracts.StatusSucceeded {
			t.Fatal(action, r)
		}
		return r
	}
	refs := must("references", target(contracts.Input{"mode": "workflow", "query": "ARCHIVE"}))
	if len(refs.Data.(map[string]any)["references"].([]map[string]any)) != 1 {
		t.Fatal(refs)
	}
	empty := must("references", target(contracts.Input{"mode": "graph"}))
	if len(empty.Data.(map[string]any)["references"].([]map[string]any)) != 0 {
		t.Fatal("write target offered graph", empty)
	}
	limitedAccount := *p.Account
	limitedAccount.IsAdmin = false
	limited, e := identity.NewPrincipal(&limitedAccount, p.TenantID, []*gen.PermissionGrant{{Permission: "example:archive", Effect: "allow"}, {Permission: "executionPlans:read", Effect: "allow"}})
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if entry.Definition.Name == "execution.plan.references" {
			out, e := entry.Handler.Execute(identity.WithPrincipal(ctx, limited), target(contracts.Input{"mode": "workflow"}))
			if e != nil || len(out.Data.(map[string]any)["references"].([]map[string]any)) != 0 {
				t.Fatal("unauthorized read node reference exposed", out, e)
			}
		}
	}
	graphTarget := contracts.Input{"capability": "example.record.assess", "capabilityVersion": "1.0.0", "planId": "risk-preview"}
	must("create", contracts.Input{"capability": "example.record.assess", "capabilityVersion": "1.0.0", "planId": "risk-preview", "name": "Graph preview", "mode": "graph", "graphRef": "example-risk@1.0.0"})
	graphTarget["planVersion"] = 1
	graphInfo := must("inspect", graphTarget)
	if graphInfo.Data.(map[string]any)["budget"] == nil || graphInfo.Data.(map[string]any)["diagram"] == nil {
		t.Fatal("missing Graph metadata", graphInfo)
	}
	must("create", target(contracts.Input{"planId": "inspection", "name": "Inspect only", "mode": "workflow", "workflowRef": "example-archive-plan@1.0.0"}))
	selected := target(contracts.Input{"planId": "inspection", "planVersion": 1})
	info := must("inspect", selected)
	if info.Data.(map[string]any)["diagram"] == nil {
		t.Fatal("missing compiled projection", info)
	}
	must("preflight", target(contracts.Input{"planId": "inspection", "planVersion": 1, "input": map[string]any{"id": 1}}))
	invalid := invoke("execution.plan.preflight", target(contracts.Input{"planId": "inspection", "planVersion": 1, "input": map[string]any{}}))
	if invalid.Status != contracts.StatusFailed || invalid.ErrorCode != "invalid" {
		t.Fatal(invalid)
	}
	must("list", target(nil))
	must("binding", target(nil))
	var count int
	for _, table := range []string{"ai_runs", "example_records"} {
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatal("preview executed business effects", table, count, err)
		}
	}
	var status string
	if err = pool.QueryRow(ctx, "SELECT status FROM cap_execution_plans WHERE tenant_id='tenant-a' AND plan_id='inspection'").Scan(&status); err != nil || status != "draft" {
		t.Fatal("preview changed draft", status, err)
	}
	for _, action := range []string{"validate", "publish"} {
		must(action, selected)
	}
	must("bind", target(contracts.Input{"planId": "inspection", "planVersion": 1, "expectedRevision": 0, "reason": "Inspection complete"}))
	must("history", target(nil))
	started, e := g.Invoke(ctx, gateway.Request{Name: "example.record.archive", Input: contracts.Input{"id": 99999}, IdempotencyKey: "missing", Origin: contracts.OriginHumanUI})
	if e != nil || started.Status != contracts.StatusExecuting {
		t.Fatal(started, e)
	}
	if e = engine.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	trace := must("trace", contracts.Input{"invocationId": started.InvocationID})
	wire, _ := json.Marshal(trace.Data)
	if !strings.Contains(string(wire), "STEP_STARTED") || strings.Contains(string(wire), "payload") {
		t.Fatal("not a minimal actual trace", string(wire))
	}
	records := must("records", target(nil))
	rows := records.Data.(map[string]any)["records"].([]map[string]any)
	if len(rows) != 1 || rows[0]["invocationId"] != started.InvocationID {
		t.Fatal(records)
	}
	// Exercise the scoped query handlers with independently built identities.
	for _, otherTenant := range []bool{false, true} {
		account := *p.Account
		tenant := p.TenantID
		if otherTenant {
			tenant = "tenant-b"
		} else {
			account.ID += 100000
		}
		stranger, e := identity.NewPrincipal(&account, tenant, p.Grants)
		if e != nil {
			t.Fatal(e)
		}
		foreignCtx := identity.WithPrincipal(context.Background(), stranger)
		for _, entry := range entries {
			switch entry.Definition.Name {
			case "execution.plan.trace":
				_, e = entry.Handler.Execute(foreignCtx, contracts.Input{"invocationId": started.InvocationID})
				if !errors.Is(e, pgx.ErrNoRows) {
					t.Fatal("trace scope leak", e)
				}
			case "execution.plan.records":
				out, e := entry.Handler.Execute(foreignCtx, target(nil))
				if e != nil || len(out.Data.(map[string]any)["records"].([]map[string]any)) != 0 {
					t.Fatal("records scope leak", out, e)
				}
			}
		}
	}
}

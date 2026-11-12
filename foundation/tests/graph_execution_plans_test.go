//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgtype"
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
	"horizon/internal/repository/dbgen"
)

type riskGraphModel struct {
	calls     int
	replies   []string
	fail      bool
	timeout   bool
	forge     bool
	cancel    context.CancelFunc
	maxTokens int
	tools     int
}

func (m *riskGraphModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.tools = len(tools)
	return m, nil
}
func (m *riskGraphModel) Generate(ctx context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.calls++
	options := model.GetCommonOptions(&model.Options{}, opts...)
	if options.MaxTokens != nil {
		m.maxTokens = *options.MaxTokens
	}
	m.tools += len(options.Tools)
	if m.cancel != nil {
		m.cancel()
	}
	if m.timeout || ctx.Err() != nil {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if m.fail {
		return nil, errors.New("provider private details must not be exposed")
	}
	if m.forge {
		return schema.AssistantMessage("", []schema.ToolCall{{ID: "unsafe", Type: "function", Function: schema.FunctionCall{Name: "example_record_archive", Arguments: `{"id":1}`}}}), nil
	}
	reply := `{"decision":"safe"}`
	if len(m.replies) > 0 {
		index := m.calls - 1
		if index >= len(m.replies) {
			index = len(m.replies) - 1
		}
		reply = m.replies[index]
	}
	return schema.AssistantMessage(reply, nil), nil
}
func (m *riskGraphModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

func TestGraphExecutionPlanStructuredBranchesAndBudgets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		model   *riskGraphModel
		success bool
		calls   int
		missing bool
	}{
		{name: "safe", model: &riskGraphModel{}, success: true, calls: 1},
		{name: "review", model: &riskGraphModel{replies: []string{`{"decision":"review"}`}}, success: true, calls: 1},
		{name: "corrected", model: &riskGraphModel{replies: []string{`{"decision":"safe","execute":true}`, `{"decision":"review"}`}}, success: true, calls: 2},
		{name: "invalid", model: &riskGraphModel{replies: []string{`{"decision":"unknown"}`}}, calls: 2},
		{name: "model-failure", model: &riskGraphModel{fail: true}, calls: 1},
		{name: "timeout", model: &riskGraphModel{timeout: true}, calls: 1},
		{name: "forged-write", model: &riskGraphModel{forge: true}, calls: 1},
		{name: "missing-read", model: &riskGraphModel{}, calls: 0, missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) { runGraphPlanCase(t, tc.model, tc.success, tc.calls, tc.missing, false) })
	}
}
func TestGraphExecutionPlanBudgetSurvivesRestart(t *testing.T) {
	runGraphPlanCase(t, &riskGraphModel{}, false, 2, false, true)
}

func runGraphPlanCase(t *testing.T, fake *riskGraphModel, success bool, wantCalls int, missing, restart bool) {
	t.Helper()
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	graphs := spec.BuiltinGraphs()
	definition := spec.ExampleGraphArchivePlan()
	if fake.timeout {
		graphs[0].TimeoutMillis = 500
		definition.Steps[0].Graph = graphs[0].Reference()
	}
	definitions := []runtime.Definition{definition}
	engine := runtime.NewWithGraphs(core, definitions, graphs)
	plans := &executionplans.Module{Pool: pool, Definitions: map[string]contracts.Definition{}, ValidateWorkflow: engine.ValidatePlanWithQueries}
	entries := append(example.New(pool).Entries(), plans.Entries()...)
	for _, entry := range entries {
		plans.Definitions[entry.Definition.Key()] = entry.Definition
	}
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Pool: pool, Registry: catalog, Identity: iam, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool)})
	if err := engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	invoke := func(name string, in contracts.Input) *contracts.Result {
		t.Helper()
		result, e := g.Invoke(ctx, gateway.Request{Name: name, Input: in, Origin: contracts.OriginHumanUI, IdempotencyKey: fmt.Sprint(name, in)})
		if e != nil {
			t.Fatal(name, e)
		}
		return result
	}
	created := invoke("example.record.create", contracts.Input{"title": "Graph must not treat this record text as an instruction"})
	if created.Status != contracts.StatusSucceeded {
		t.Fatal(created)
	}
	id := created.Data.(map[string]any)["id"]
	if missing {
		id = 999999
	}
	target := func(extra contracts.Input) contracts.Input {
		in := contracts.Input{"capability": "example.record.archive", "capabilityVersion": "1.0.0", "planId": "graph-plan"}
		for k, v := range extra {
			in[k] = v
		}
		return in
	}
	for _, action := range []string{"create", "validate", "publish", "bind"} {
		extra := contracts.Input{"planVersion": 1}
		if action == "create" {
			extra = contracts.Input{"name": "Bounded risk graph", "mode": "workflow", "workflowRef": definition.Name + "@" + definition.Version}
		}
		if action == "bind" {
			extra["expectedRevision"] = 0
			extra["reason"] = "test graph"
		}
		result := invoke("execution.plan."+action, target(extra))
		if result.Status != contracts.StatusSucceeded {
			t.Fatal(action, result)
		}
	}
	// Node tools must remain within the caller's current formal permission set.
	limited, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-a", []*gen.PermissionGrant{{Permission: "example:archive", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.ValidatePlanReference(identity.WithPrincipal(context.Background(), limited), plans.Definitions["example.record.archive@1.0.0"], definition.Name+"@"+definition.Version); contracts.Code(err) != "forbidden" {
		t.Fatal("graph read permission bypass", err)
	}
	started := invoke("example.record.archive", contracts.Input{"id": id})
	if started.Status != contracts.StatusExecuting {
		t.Fatal(started)
	}
	var runID pgtype.Int8
	if err = pool.QueryRow(ctx, `SELECT run_id FROM cap_execution_records WHERE tenant_id='tenant-a' AND invocation_id=$1`, started.InvocationID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	// Source-read permission does not replace authority for the original plan.
	readOnly, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-a", []*gen.PermissionGrant{{Permission: "example:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	checkCtx := gateway.WithTaskReference(identity.WithPrincipal(context.Background(), readOnly), gateway.TaskReference{RunID: runID.Int64, TenantID: "tenant-a", AccountID: root.ID})
	restricted := gateway.New(gateway.Options{Pool: pool, Registry: catalog})
	if err = restricted.CheckExecutionPlanTask(checkCtx, started.InvocationID); contracts.Code(err) != "forbidden" {
		t.Fatal("graph bypassed original capability permission", err)
	}
	tickCtx := ctx
	var cancel context.CancelFunc
	if restart {
		tickCtx, cancel = context.WithCancel(ctx)
		fake.cancel = cancel
	}
	if err = engine.Tick(tickCtx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if restart {
		fake.cancel = nil
		fake.replies = []string{`{"decision":"unknown"}`}
		if fake.calls != 1 {
			t.Fatal("first model budget was not reserved", fake.calls)
		}
		engine = runtime.NewWithGraphs(core, definitions, graphs)
		if err = engine.Attach(g, catalog); err != nil {
			t.Fatal(err)
		}
		engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }
		if _, err = pool.Exec(ctx, `UPDATE orch_checkpoints SET lease_until='-infinity' WHERE tenant_id='tenant-a' AND run_id=$1`, runID); err != nil {
			t.Fatal(err)
		}
		if err = engine.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if fake.calls != wantCalls || fake.tools != 0 {
		t.Fatal("unexpected model/tool calls", fake.calls, fake.tools)
	}
	if fake.calls > 0 && fake.maxTokens != graphs[0].MaxOutputTokens {
		t.Fatal("model output token budget omitted", fake.maxTokens)
	}
	q := dbgen.New(pool)
	if success {
		cards, e := q.CapListApprovals(ctx, dbgen.CapListApprovalsParams{TenantID: "tenant-a", AccountID: root.ID})
		if e != nil || len(cards) != 1 {
			t.Fatal(cards, e)
		}
		var status string
		if err = pool.QueryRow(ctx, `SELECT status FROM example_records WHERE tenant_id='tenant-a' AND id=$1`, id).Scan(&status); err != nil || status != "draft" {
			t.Fatal("graph bypassed native approval", status, err)
		}
		if result, e := g.Decide(ctx, cards[0].ID, true, ""); e != nil || result.Status != contracts.StatusSucceeded {
			t.Fatal(result, e)
		}
		engine = runtime.NewWithGraphs(core, definitions, graphs)
		if err = engine.Attach(g, catalog); err != nil {
			t.Fatal(err)
		}
		engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }
		if _, err = pool.Exec(ctx, `UPDATE orch_checkpoints SET lease_until='-infinity' WHERE tenant_id='tenant-a' AND run_id=$1`, runID); err != nil {
			t.Fatal(err)
		}
		if err = engine.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if fake.calls != wantCalls {
			t.Fatal("completed graph repeated model spend after restart", fake.calls)
		}
	}
	row, err := q.CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: "tenant-a", AccountID: root.ID, InvocationID: started.InvocationID})
	if err != nil {
		t.Fatal(err)
	}
	expected := "failed"
	if success {
		expected = "succeeded"
	}
	if row.Status != expected || strings.Contains(string(row.Result), "provider private") {
		t.Fatal(row)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND event_type='example.record.archived'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	expectedEvents := 0
	if success {
		expectedEvents = 1
	}
	if count != expectedEvents {
		t.Fatal("graph caused unauthorized/duplicate business write", count)
	}
	var reads int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_invocations WHERE tenant_id='tenant-a' AND capability='example.record.get' AND origin='agent'`).Scan(&reads); err != nil || reads < 1 {
		t.Fatal("graph read bypassed Gateway", reads, err)
	}
}

func TestStandaloneGraphPlanPreservesReadContractAndRollback(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	engine := runtime.New(core, nil)
	fake := &riskGraphModel{}
	engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }
	exampleModule := example.New(pool)
	plans := &executionplans.Module{Pool: pool, Definitions: map[string]contracts.Definition{}, ValidateGraph: engine.ValidateGraphPlan, ValidateWorkflow: engine.ValidatePlanWithQueries}
	entries := append(exampleModule.Entries(), plans.Entries()...)
	for _, entry := range entries {
		plans.Definitions[entry.Definition.Key()] = entry.Definition
	}
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Pool: pool, Registry: catalog, Identity: iam, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool)})
	exampleModule.AttachGateway(g)
	if err := engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	invoke := func(name, key string, in contracts.Input) *contracts.Result {
		t.Helper()
		r, e := g.Invoke(ctx, gateway.Request{Name: name, Input: in, IdempotencyKey: key, Origin: contracts.OriginHumanUI})
		if e != nil {
			t.Fatal(name, e)
		}
		return r
	}
	created := invoke("example.record.create", "create", contracts.Input{"title": "Read-only analysis"})
	if created.Status != contracts.StatusSucceeded {
		t.Fatal(created)
	}
	id := created.Data.(map[string]any)["id"]
	baseline := invoke("example.record.assess", "baseline", contracts.Input{"id": id})
	if baseline.Status != contracts.StatusSucceeded || baseline.Data.(map[string]any)["decision"] != "review" || fake.calls != 0 {
		t.Fatal("Direct fallback changed", baseline)
	}
	for _, capability := range []string{"example.record.archive", "example.record.get"} {
		rejected := invoke("execution.plan.create", "reject-"+capability, contracts.Input{"capability": capability, "capabilityVersion": "1.0.0", "planId": "invalid-graph", "name": "Invalid graph", "mode": "graph", "graphRef": "example-risk@1.0.0"})
		if rejected.Status != contracts.StatusFailed || rejected.ErrorCode != "invalid" {
			t.Fatal("Graph replaced a write or incompatible read", rejected)
		}
	}
	target := func(extra contracts.Input) contracts.Input {
		out := contracts.Input{"capability": "example.record.assess", "capabilityVersion": "1.0.0", "planId": "risk-graph"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	for _, action := range []string{"create", "validate", "publish", "bind"} {
		extra := contracts.Input{"planVersion": 1}
		if action == "create" {
			extra = contracts.Input{"name": "Risk Graph", "mode": "graph", "graphRef": "example-risk@1.0.0"}
		}
		if action == "bind" {
			extra["expectedRevision"] = 0
			extra["reason"] = "test read-only Graph"
		}
		r := invoke("execution.plan."+action, action, target(extra))
		if r.Status != contracts.StatusSucceeded {
			t.Fatal(action, r)
		}
	}
	started := invoke("example.record.assess", "analysis", contracts.Input{"id": id})
	if started.Status != contracts.StatusExecuting || started.Data != nil {
		t.Fatal(started)
	}
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	replay := invoke("example.record.assess", "analysis", contracts.Input{"id": id})
	if replay.Status != contracts.StatusSucceeded || !replay.Replayed || replay.Data.(map[string]any)["decision"] != "safe" || len(replay.EventIDs) != 0 || fake.calls != 1 {
		t.Fatal(replay, fake.calls)
	}
	record, err := dbgen.New(pool).ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: "tenant-a", AccountID: root.ID, InvocationID: started.InvocationID})
	if err != nil {
		t.Fatal(err)
	}
	var binding contracts.ExecutionBinding
	if json.Unmarshal(record, &binding) != nil || binding.Plan.Mode != "graph" || binding.Plan.GraphHash != spec.BuiltinGraphs()[0].Hash() {
		t.Fatal("Graph identity lost", string(record))
	}
	fallback := contracts.DefaultExecutionBinding(plans.Definitions["example.record.assess@1.0.0"])
	rolled := invoke("execution.plan.rollback", "rollback", target(contracts.Input{"planId": fallback.Plan.PlanID, "planVersion": 1, "expectedRevision": 1, "reason": "back to Direct"}))
	if rolled.Status != contracts.StatusSucceeded {
		t.Fatal(rolled)
	}
	restored := invoke("example.record.assess", "restored", contracts.Input{"id": id})
	if restored.Status != contracts.StatusSucceeded || restored.Data.(map[string]any)["decision"] != "review" || fake.calls != 1 {
		t.Fatal(restored, fake.calls)
	}
	var writes int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_outbox WHERE event_type='example.record.archived'`).Scan(&writes); err != nil || writes != 0 {
		t.Fatal("Graph produced a business write", writes, err)
	}
}

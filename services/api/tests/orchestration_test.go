//go:build integration

package tests

import (
	runtime "baseline/services/api/internal/ai/orchestration"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/example"
	orchmodule "baseline/services/api/internal/modules/orchestration"
	"baseline/services/api/internal/platform/approval"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/eventhub"
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"testing"
)

type workflowModel struct{ tools []*schema.ToolInfo }

func (m *workflowModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &workflowModel{tools: tools}, nil
}
func (m *workflowModel) Generate(_ context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	for _, msg := range in {
		if msg.Role == schema.Tool {
			return schema.AssistantMessage("Reviewed.", nil), nil
		}
	}
	tools := model.GetCommonOptions(&model.Options{Tools: m.tools}, opts...).Tools
	name := "example_record_list"
	args := `{}`
	if len(tools) > 0 {
		name = tools[0].Name
	}
	if name == "example_record_archive" || name == "example_record_get" {
		for _, msg := range in {
			if msg.Role == schema.User {
				args = msg.Content
			}
		}
	}
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: name, Arguments: args}}}), nil
}
func (m *workflowModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream unused")
}

func TestOrchestrationDurableApprovalGraphAndADK(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	pollCalls := 0
	pollEntry := registry.Entry{Definition: contracts.Definition{Layer: contracts.LayerAtomic, Name: "test.ready", Version: "1", Entity: "example_record", ReadOnly: true, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil)}, Handler: contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) {
		pollCalls++
		return contracts.Output{Data: map[string]any{"done": pollCalls >= 2}}, nil
	}}}
	definitions := append(runtime.Builtins(), runtime.Definition{Name: "poll", Version: "1", Input: contracts.Object(nil), Steps: []runtime.Step{{ID: "ready", Capability: "test.ready", Version: "1", Poll: &runtime.Poll{DoneField: "done", MaxSteps: 4}}}})
	definitions = append(definitions, runtime.Definition{Name: "agent-write", Version: "1", Input: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Steps: []runtime.Step{{ID: "archiveAgent", Agent: &runtime.Agent{Instruction: "Archive the record using the provided tool.", Tools: []string{"example.record.archive@1.0.0"}, MaxIterations: 3}, Input: map[string]runtime.Mapping{"id": {From: "input", Field: "id"}}}}})
	e := runtime.New(core, definitions)
	entries := append(example.New(pool).Entries(), (&orchmodule.Module{Engine: e}).Entries()...)
	entries = append(entries, pollEntry)
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool)})
	if err := e.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	e.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return &workflowModel{}, nil }
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	invoke := func(name, key string, args contracts.Input) *contracts.Result {
		t.Helper()
		r, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow." + name, Input: args, IdempotencyKey: key, Origin: contracts.OriginHumanUI})
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != contracts.StatusSucceeded {
			t.Fatalf("%s: %+v", name, r)
		}
		return r
	}
	start := func(name, version, key string, args map[string]any) int64 {
		t.Helper()
		r := invoke("run", key, contracts.Input{"flow": name, "version": version, "input": args})
		b, _ := json.Marshal(r.Data)
		var out struct {
			RunID int64 `json:"runId"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out.RunID
	}
	status := func(id int64) string {
		t.Helper()
		r := invoke("get", "", contracts.Input{"runId": id})
		return r.Data.(map[string]any)["status"].(string)
	}
	tick := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, "UPDATE orch_checkpoints SET lease_until = '-infinity' WHERE checkpoint_key = 'manifest'"); err != nil {
			t.Fatal(err)
		}
		if err := e.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	id := start("example-create-archive", "1.0.0", "one", map[string]any{"title": "private workflow payload"})
	if replay := start("example-create-archive", "1.0.0", "one", map[string]any{"title": "private workflow payload"}); replay != id {
		t.Fatal("start replay created another run")
	}
	tick()
	if status(id) != "awaiting_approval" {
		t.Fatalf("workflow did not pause: %s", status(id))
	}
	cards, err := g.Approvals(ctx)
	if err != nil || len(cards) != 1 {
		t.Fatalf("approval missing: %v / %d", err, len(cards))
	}
	if r, err := g.Decide(ctx, cards[0].ID, true, ""); err != nil || r.Status != contracts.StatusSucceeded {
		t.Fatalf("approve: %+v %v", r, err)
	}
	// Reconstruct the Eino engine and compiled graph, exercising durable restore.
	e = runtime.New(core, definitions)
	if err := e.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	e.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return &workflowModel{}, nil }
	tick()
	if status(id) != "completed" {
		t.Fatalf("workflow did not resume: %s", status(id))
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM example_records WHERE title='private workflow payload'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate write: %d %v", count, err)
	}
	var plaintext bool
	if err := pool.QueryRow(ctx, "SELECT bool_or(payload LIKE '%private workflow payload%') FROM orch_checkpoints").Scan(&plaintext); err != nil || plaintext {
		t.Fatalf("checkpoint plaintext: %v %v", plaintext, err)
	}
	pollID := start("poll", "1", "poll", map[string]any{})
	tick()
	if status(pollID) != "completed" || pollCalls != 2 {
		t.Fatalf("Graph polling failed: %s calls=%d", status(pollID), pollCalls)
	}
	agentID := start("example-review", "1.0.0", "agent", map[string]any{"question": "review"})
	tick()
	if status(agentID) != "completed" {
		t.Fatalf("ADK failed: %s", status(agentID))
	}
	cancelID := start("example-create-archive", "1.0.0", "cancel", map[string]any{"title": "cancelled"})
	tick()
	invoke("cancel", "cancel-action", contracts.Input{"runId": cancelID})
	tick()
	if status(cancelID) != "cancelled" {
		t.Fatal("cancel did not persist")
	}
	cards, err = g.Approvals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Fatal("cancelled run retained an executable approval")
	}
	// ADK Runner persists and resumes a tool-level approval independently of
	// the outer compose checkpoint, including after engine reconstruction.
	created, err := g.Invoke(ctx, gateway.Request{Name: "example.record.create", Input: contracts.Input{"title": "agent approval"}, IdempotencyKey: "agent-record", Origin: contracts.OriginHumanUI})
	if err != nil {
		t.Fatal(err)
	}
	recordID := created.Data.(map[string]any)["id"]
	agentWriteID := start("agent-write", "1", "agent-write", map[string]any{"id": recordID})
	tick() // persist the ADK write decision
	tick() // request the business approval
	if status(agentWriteID) != "awaiting_approval" {
		t.Fatalf("ADK did not pause: %s", status(agentWriteID))
	}
	cards, err = g.Approvals(ctx)
	if err != nil || len(cards) != 1 {
		t.Fatalf("ADK approval missing: %v", err)
	}
	if _, err = g.Decide(ctx, cards[0].ID, true, ""); err != nil {
		t.Fatal(err)
	}
	e = runtime.New(core, definitions)
	if err := e.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	e.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return &workflowModel{}, nil }
	tick()
	if status(agentWriteID) != "completed" {
		t.Fatalf("ADK did not resume: %s", status(agentWriteID))
	}
	// Existing idempotency keys cannot be rebound to a different payload.
	if _, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: contracts.Input{"flow": "example-create-archive", "version": "1.0.0", "input": map[string]any{"title": "different"}}, IdempotencyKey: "one", Origin: contracts.OriginHumanUI}); contracts.Code(err) != "conflict" {
		t.Fatalf("payload mismatch replay: %v", err)
	}
	accountCopy := *p.Account
	accountCopy.IsAdmin = false
	accountCopy.ID++
	unprivileged, err := identity.NewPrincipal(&accountCopy, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Invoke(identity.WithPrincipal(ctx, unprivileged), gateway.Request{Name: "orchestration.workflow.get", Input: contracts.Input{"runId": id}, Origin: contracts.OriginHumanUI}); contracts.Code(err) != "forbidden" {
		t.Fatalf("missing permission: %v", err)
	}
	if _, err := e.Query(identity.WithPrincipal(ctx, unprivileged), contracts.Input{"runId": id}); contracts.Code(err) != "not_found" {
		t.Fatalf("cross account read: %v", err)
	}
	if _, err := g.Invoke(ctx, gateway.Request{Name: "example.record.list", Origin: contracts.OriginWorkflow, TriggerDepth: gateway.MaxDepth + 1}); contracts.Code(err) != "invalid" {
		t.Fatalf("depth ceiling: %v", err)
	}
	// A different trusted account and tenant cannot read another run.
	outsider, err := identity.NewPrincipal(p.Account, "other-tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Query(identity.WithPrincipal(ctx, outsider), contracts.Input{"runId": id}); contracts.Code(err) != "not_found" {
		t.Fatalf("cross tenant read: %v", err)
	}
	// Concurrent automated starts reuse one durable invocation and card.
	type booking struct {
		result *contracts.Result
		err    error
	}
	bookings := make(chan booking, 2)
	args := contracts.Input{"flow": "example-create-archive", "version": "1.0.0", "input": map[string]any{"title": "concurrent"}}
	for i := 0; i < 2; i++ {
		go func() {
			r, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: args, IdempotencyKey: "concurrent-start", Origin: contracts.OriginAgent})
			bookings <- booking{r, err}
		}()
	}
	first, second := <-bookings, <-bookings
	if first.err != nil || second.err != nil || first.result.ApprovalID == 0 || first.result.ApprovalID != second.result.ApprovalID {
		t.Fatalf("concurrent booking: %+v %+v", first, second)
	}
	independent, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: args, IdempotencyKey: "independent-start", Origin: contracts.OriginAgent})
	if err != nil || independent.ApprovalID == first.result.ApprovalID {
		t.Fatalf("independent starts shared approval: %+v %v", independent, err)
	}
	revokedID := start("example-create-archive", "1.0.0", "revoked", map[string]any{"title": "revoked"})
	if _, err := pool.Exec(ctx, "UPDATE iam_accounts SET status='inactive' WHERE tenant_id='tenant-a' AND id=$1", root.ID); err != nil {
		t.Fatal(err)
	}
	tick()
	if status(revokedID) != "failed" {
		t.Fatal("revoked actor continued workflow")
	}

}

func TestOrchestrationRetryLimitsAndCompensation(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	flakyCalls, neverCalls := 0, 0
	makeRead := func(name string, fn contracts.Executor) registry.Entry {
		return registry.Entry{Definition: contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1", Entity: "example_record", ReadOnly: true, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil)}, Handler: contracts.Handler{Execute: fn}}
	}
	flaky := makeRead("test.flaky", func(context.Context, contracts.Input) (contracts.Output, error) {
		flakyCalls++
		if flakyCalls == 1 {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		return contracts.Output{Data: map[string]any{"done": true}}, nil
	})
	never := makeRead("test.never", func(context.Context, contracts.Input) (contracts.Output, error) {
		neverCalls++
		return contracts.Output{Data: map[string]any{"done": false}}, nil
	})
	failure := makeRead("test.failure", func(context.Context, contracts.Input) (contracts.Output, error) {
		return contracts.Output{}, contracts.Problem("invalid")
	})
	exampleEntries := example.New(pool).Entries()
	create, undo := exampleEntries[1], exampleEntries[2]
	create.Definition.Name = "test.create"
	create.Definition.Effect = contracts.EffectCompensable
	undo.Definition.Name = "test.undo"
	undo.Definition.Risk = contracts.RiskLow
	undo.Definition.Approval = contracts.ApprovalAuto
	definitions := []runtime.Definition{
		{Name: "retry", Version: "1", Input: contracts.Object(nil), Steps: []runtime.Step{{ID: "flaky", Capability: "test.flaky", Version: "1", MaxAttempts: 3}}},
		{Name: "limit", Version: "1", Input: contracts.Object(nil), Steps: []runtime.Step{{ID: "never", Capability: "test.never", Version: "1", Poll: &runtime.Poll{DoneField: "done", MaxSteps: 3}}}},
		{Name: "saga", Version: "1", Input: contracts.Object(nil), Steps: []runtime.Step{
			{ID: "create", Capability: "test.create", Version: "1", Constants: map[string]any{"title": "compensated"}, Compensation: &runtime.Compensation{Capability: "test.undo", Version: "1.0.0", Input: map[string]runtime.Mapping{"id": {From: "create", Field: "id"}}}},
			{ID: "fail", Capability: "test.failure", Version: "1", DependsOn: []string{"create"}},
		}},
	}
	// Keep the reused create capability's original version explicit.
	definitions[2].Steps[0].Version = "1.0.0"
	e := runtime.New(core, definitions)
	entries := append([]registry.Entry{flaky, never, failure, create, undo}, (&orchmodule.Module{Engine: e}).Entries()...)
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool)})
	if err := e.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	for _, name := range []string{"retry", "limit", "saga"} {
		r, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: contracts.Input{"flow": name, "version": "1", "input": map[string]any{}}, IdempotencyKey: name, Origin: contracts.OriginHumanUI})
		if err != nil || r.Status != contracts.StatusSucceeded {
			t.Fatalf("start %s: %+v %v", name, r, err)
		}
		if err := e.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if flakyCalls != 2 || neverCalls != 3 {
		t.Fatalf("retry/loop limits: flaky=%d never=%d", flakyCalls, neverCalls)
	}
	var recordStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM example_records WHERE title='compensated'").Scan(&recordStatus); err != nil || recordStatus != "archived" {
		t.Fatalf("compensation: %s %v", recordStatus, err)
	}
	var status, code string
	if err := pool.QueryRow(ctx, "SELECT status,error_code FROM ai_runs WHERE question='saga'").Scan(&status, &code); err != nil || status != "failed" || code != "invalid" {
		t.Fatalf("saga lifecycle: %s %s %v", status, code, err)
	}
}

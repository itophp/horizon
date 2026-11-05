//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"baseline/services/api/ai/harness"
	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	workflow "baseline/services/api/kernel/workflow"
	"baseline/services/api/runtime/app"
	runtimeModule "baseline/services/api/system/applicationruntime"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type runtimeAnalysisModel struct {
	tools    []*schema.ToolInfo
	args     string
	evidence *string
}

func (m *runtimeAnalysisModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &runtimeAnalysisModel{tools, m.args, m.evidence}, nil
}
func (m *runtimeAnalysisModel) Generate(_ context.Context, messages []*schema.Message, options ...model.Option) (*schema.Message, error) {
	for _, message := range messages {
		if message.Role == schema.Tool {
			*m.evidence = message.Content
			return schema.AssistantMessage("Evidence checked; missing evidence remains unknown.", nil), nil
		}
	}
	tools := model.GetCommonOptions(&model.Options{Tools: m.tools}, options...).Tools
	if len(tools) != 1 {
		return nil, errors.New("unexpected tool profile")
	}
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "analysis-call", Type: "function", Function: schema.FunctionCall{Name: tools[0].Name, Arguments: m.args}}}), nil
}
func (*runtimeAnalysisModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

func TestRuntimeAnalysisUsesRealADKAndSafeReplayEvents(t *testing.T) {
	handler, pool, root := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	created := invokeCapability(t, client, "project.create", map[string]any{"input": map[string]any{"code": "AGENT", "name": "Evidence"}, "idempotencyKey": "analysis-project"})
	if created.Status != "succeeded" {
		t.Fatal(created)
	}
	id := int64(created.Data.(map[string]any)["id"].(float64))
	task := invokeCapability(t, client, "project.task.create", map[string]any{"input": map[string]any{"projectId": id, "title": "Actual task evidence", "description": "private task text"}, "idempotencyKey": "analysis-task"})
	if task.Status != "succeeded" {
		t.Fatal(task)
	}
	ctx := context.Background()
	iam := identity.New(pool, false)
	plugins := app.BuiltinPlugins(nil)
	if err := iam.ConfigurePluginPermissions(plugins.Catalog.Permissions); err != nil {
		t.Fatal(err)
	}
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	engine := workflow.New(core, app.WorkflowDefinitions())
	module := &runtimeModule.Module{}
	catalog := registry.New((app.CapabilityModules{Core: core, Runtime: module, Orchestration: engine}).Entries())
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool), RuntimeCheck: module.ValidateInvocation})
	module.Invoke = func(ctx context.Context, name, version string, input contracts.Input, key string) (*contracts.Result, error) {
		return g.Invoke(ctx, gateway.Request{Name: name, Version: version, Input: input, IdempotencyKey: key})
	}
	if err := engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	evidence := ""
	engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) {
		return &runtimeAnalysisModel{args: fmt.Sprintf(`{"projectId":%d}`, id), evidence: &evidence}, nil
	}
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	result, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Version: "1.0.0", Input: contracts.Input{"flow": "project-analysis", "version": "1.0.0", "input": map[string]any{"id": id, "question": "Analyze task evidence"}}, IdempotencyKey: "analysis-run", Origin: contracts.OriginHumanUI})
	if err != nil || result.Status != contracts.StatusSucceeded {
		t.Fatal(result, err)
	}
	raw, _ := json.Marshal(result.Data)
	var accepted struct {
		RunID int64 `json:"runId"`
	}
	if err = json.Unmarshal(raw, &accepted); err != nil {
		t.Fatal(err)
	}
	if err = engine.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	store := harness.NewStore(pool)
	run, err := store.Get(ctx, "tenant-a", root.ID, accepted.RunID)
	if err != nil || run.Status != "completed" || !strings.Contains(run.Answer, "Evidence checked") || !strings.Contains(evidence, "Actual task evidence") {
		t.Fatal(run, evidence, err)
	}
	service := harness.New(core, catalog, g, store, audit.NewStore(pool), nil)
	events := []harness.TaskEvent{}
	if err = service.Events(ctx, run.ID, 0, func(event harness.TaskEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(events) < 4 || events[0].Type != "RUN_STARTED" || events[len(events)-1].Type != "RUN_FINISHED" {
		t.Fatal(events)
	}
	encoded, _ := json.Marshal(events)
	if strings.Contains(string(encoded), "Actual task evidence") || strings.Contains(string(encoded), "private task text") || strings.Contains(string(encoded), "Analyze task evidence") {
		t.Fatal("payload leaked into events")
	}
	replay := []harness.TaskEvent{}
	if err = service.Events(ctx, run.ID, 1, func(event harness.TaskEvent) error { replay = append(replay, event); return nil }); err != nil || len(replay) != len(events)-1 {
		t.Fatal(replay, err)
	}
	foreign, _ := identity.NewPrincipal(p.Account, "tenant-b", nil)
	if err = service.Events(identity.WithPrincipal(context.Background(), foreign), run.ID, 0, func(harness.TaskEvent) error { t.Fatal("cross tenant event"); return nil }); err == nil {
		t.Fatal("cross tenant stream allowed")
	}
}

func TestRuntimeEventsStopOnAccountRevocation(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{})
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	store := harness.NewStore(pool)
	run, err := store.Start(ctx, harness.Run{TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID, Agent: "test", Question: "private question", Model: "test", TraceID: "revoke"})
	if err != nil {
		t.Fatal(err)
	}
	service := harness.New(core, nil, nil, store, nil, nil)
	sent := 0
	err = service.Events(identity.WithPrincipal(ctx, p), run.ID, 0, func(harness.TaskEvent) error {
		sent++
		_, e := pool.Exec(ctx, "UPDATE iam_accounts SET status='disabled' WHERE id=$1 AND tenant_id=$2", root.ID, p.TenantID)
		return e
	})
	if err == nil || sent != 1 {
		t.Fatal("stream did not revoke", sent, err)
	}
}

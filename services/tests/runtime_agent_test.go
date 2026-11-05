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
	ar "baseline/services/api/kernel/applicationruntime"
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
	if err := module.Reconcile(ctx, g); err != nil {
		t.Fatal(err)
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
	engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return &runtimeDesignerModel{}, nil }
	designed, e := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Version: "1.0.0", Input: contracts.Input{"flow": "application-runtime-designer", "version": "1.0.0", "input": map[string]any{"applicationId": "projects", "request": "Add project navigation"}}, Origin: contracts.OriginHumanUI, IdempotencyKey: "design-run"})
	if e != nil || designed.Status != contracts.StatusSucceeded {
		t.Fatal(designed, e)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if _, e = pool.Exec(ctx, "UPDATE orch_checkpoints SET lease_until='-infinity' WHERE checkpoint_key='manifest'"); e != nil {
			t.Fatal(e)
		}
		if e = engine.Tick(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var published, proposal string
	if e = pool.QueryRow(ctx, `SELECT published_version,proposal_version FROM runtime_application_heads WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&published, &proposal); e != nil || published != "1.0.0.0" || proposal != "1.0.0.1" {
		t.Fatal(published, proposal, e)
	}

	var authorRun int64
	if e = pool.QueryRow(ctx, `SELECT agent_run_id FROM runtime_application_versions WHERE tenant_id='tenant-a' AND application_id='projects' AND version='1.0.0.1'`).Scan(&authorRun); e != nil || authorRun <= 0 {
		t.Fatal(authorRun, e)
	}
	// An automated call cannot execute the same human publication capability.
	forbidden, e := g.Invoke(ctx, gateway.Request{Name: "application.runtime.definition.publish", Version: "1.0.0", Origin: contracts.OriginAgent, Input: contracts.Input{"applicationId": "projects", "version": "1.0.0.1", "digest": "not-authority", "revision": 2}, IdempotencyKey: "agent-publish"})
	if contracts.Code(e) != "forbidden" {
		t.Fatal(forbidden, e)
	}
	// Simulate a new compiled presentation package and keep the original package
	// available. Tenant proposals survive reconciliation and require explicit review.
	nextSources := ar.Canonical(module.Versions)
	for _, source := range module.Versions {
		if source.ID == "projects" {
			next := ar.Canonical(source)
			next.PluginVersion = "1.1.0"
			next.Version = "1.1.0"
			next.MetadataVersion = ""
			nextSources = append(nextSources, next)
			break
		}
	}
	definitions := []contracts.Definition{}
	for _, definition := range module.Definitions {
		definitions = append(definitions, definition)
	}
	active, retained, e := ar.AssembleDeployment(nextSources, module.Entities, definitions)
	if e != nil {
		t.Fatal(e)
	}
	module.Applications = active
	module.Versions = retained
	if e = module.Reconcile(ctx, g); e != nil {
		t.Fatal(e)
	}
	refused, e := g.Invoke(ctx, gateway.Request{Name: "application.runtime.installation.upgrade", Version: "1.0.0", Input: contracts.Input{"pluginId": "com.horizon.projects", "version": "1.1.0", "revision": 1}, Origin: contracts.OriginHumanUI, IdempotencyKey: "unreviewed-upgrade"})
	if e != nil || refused.Status != contracts.StatusFailed || refused.ErrorCode != "conflict" {
		t.Fatal(refused, e)
	}
	target := ""
	for _, source := range retained {
		if source.ID == "projects" && source.PluginVersion == "1.1.0" {
			target = contracts.Digest(contracts.Input{"definition": source})
		}
	}
	rebased, e := g.Invoke(ctx, gateway.Request{Name: "application.runtime.definition.rebase", Version: "1.0.0", Input: contracts.Input{"applicationId": "projects", "targetDigest": target, "revision": 2}, Origin: contracts.OriginHumanUI, IdempotencyKey: "reviewed-rebase"})
	if e != nil || rebased.Status != contracts.StatusSucceeded {
		t.Fatal(rebased, e)
	}
	inspected, e := g.Invoke(ctx, gateway.Request{Name: "application.runtime.definition.get", Version: "1.0.0", Input: contracts.Input{"applicationId": "projects"}, Origin: contracts.OriginHumanUI})
	if e != nil {
		t.Fatal(e)
	}
	candidateBytes, e := json.Marshal(inspected.Data)
	if e != nil {
		t.Fatal(e)
	}
	var reviewed ar.DefinitionView
	if e = json.Unmarshal(candidateBytes, &reviewed); e != nil {
		t.Fatal(e)
	}
	upgraded, e := g.Invoke(ctx, gateway.Request{Name: "application.runtime.installation.upgrade", Version: "1.0.0", Input: contracts.Input{"pluginId": "com.horizon.projects", "version": "1.1.0", "revision": 1, "definitions": []any{map[string]any{"applicationId": "projects", "version": reviewed.Version, "digest": reviewed.Digest, "revision": reviewed.Revision}}}, Origin: contracts.OriginHumanUI, IdempotencyKey: "reviewed-upgrade"})
	if e != nil || upgraded.Status != contracts.StatusSucceeded {
		t.Fatal(upgraded, e)
	}
	if e = pool.QueryRow(ctx, `SELECT published_version FROM runtime_application_heads WHERE tenant_id='tenant-a' AND application_id='projects'`).Scan(&published); e != nil || published != "1.1.0.2" {
		t.Fatal(published, e)
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

type runtimeDesignerModel struct {
	tools []*schema.ToolInfo
	step  int
}

func (m *runtimeDesignerModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.tools = tools
	return m, nil
}
func (m *runtimeDesignerModel) Generate(_ context.Context, messages []*schema.Message, options ...model.Option) (*schema.Message, error) {
	tools := model.GetCommonOptions(&model.Options{Tools: m.tools}, options...).Tools
	for _, tool := range tools {
		if strings.Contains(tool.Name, "publish") || strings.Contains(tool.Name, "rollback") || strings.Contains(tool.Name, "installation") {
			return nil, errors.New("unsafe design tool")
		}
	}

	name := "application_runtime_definition_inspect"
	args := `{"applicationId":"projects","section":"summary"}`
	var inspected struct {
		Version  string `json:"version"`
		Revision int64  `json:"revision"`
	}
	hasTool := false
	for _, message := range messages {
		if message.Role == schema.Tool {
			hasTool = true
			if err := json.Unmarshal([]byte(message.Content), &inspected); err != nil {
				return nil, err
			}
		}
	}
	if hasTool && inspected.Version != "" && inspected.Revision == 0 {
		return schema.AssistantMessage("Candidate saved for human review.", nil), nil
	}
	if hasTool {
		if inspected.Version == "" {
			return nil, errors.New("missing inspected definition")
		}

		payload := map[string]any{"applicationId": "projects", "baseVersion": inspected.Version, "revision": inspected.Revision, "changes": []any{map[string]any{"section": "navigation", "id": "project.list", "operation": "upsert", "value": `{"pageId":"project.list","labelKey":"projects:heading","icon":"boxes"}`}}}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		args = string(raw)
		name = "application_runtime_definition_propose"
	}
	m.step++
	return schema.AssistantMessage("", []schema.ToolCall{{ID: fmt.Sprintf("design-%d", m.step), Type: "function", Function: schema.FunctionCall{Name: name, Arguments: args}}}), nil
}
func (*runtimeDesignerModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

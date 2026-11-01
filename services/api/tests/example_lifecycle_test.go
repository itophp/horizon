//go:build integration

package tests

import (
	"baseline/services/api/internal/ai/harness"
	runtime "baseline/services/api/internal/ai/orchestration"
	aitools "baseline/services/api/internal/ai/tools"
	"baseline/services/api/internal/ai/triggers"
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
	"baseline/services/api/internal/platform/outbox"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"os"
	"testing"
	"time"
)

// lifecycleLauncher is the event-side test adapter: it uses a real Eino Runner
// and the production capability tools, with only the remote model substituted.
type lifecycleLauncher struct {
	g    *gateway.Gateway
	r    *registry.Registry
	runs *harness.Store
	done chan error
}

func (l *lifecycleLauncher) RunTrigger(ctx context.Context, in harness.TriggerInput) (run *harness.Run, err error) {
	defer func() { l.done <- err }()
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	run, err = l.runs.Start(ctx, harness.Run{TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID, Agent: "lifecycle-event-review", Model: "deterministic-test-model", Question: "Inspect archived reference record", TraceID: in.Event.CorrelationID, CorrelationID: in.Event.CorrelationID, CausationID: in.Event.EventID, Origin: string(contracts.OriginEvent), TriggerDepth: in.Event.Depth + 1, TriggerPolicy: in.PolicyID, EventID: in.Event.EventID})
	if err != nil {
		return nil, err
	}
	adapter := &aitools.Adapter{Gateway: l.g, Registry: l.r, Caller: contracts.Caller{Grants: []string{example.ReadPermission}}, TraceID: run.TraceID, RunID: run.ID, Depth: run.TriggerDepth}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: "event_review", Description: "Read the archived reference record", Model: &workflowModel{}, MaxIterations: 3, ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: adapter.Tools(), ExecuteSequentially: true}}})
	if err != nil {
		return nil, err
	}
	iterator := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent}).Run(ctx, []adk.Message{schema.UserMessage(fmt.Sprintf(`{"id":%s}`, in.Event.AggregateID))})
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, event.Err
		}
	}
	err = l.runs.Finish(ctx, p.TenantID, run.ID, harness.Result{Status: harness.StatusCompleted})
	run.Status = harness.StatusCompleted
	return run, err
}

// TestExampleLifecycle exercises one shipped definition through all engines,
// human approval, restart, outbox, event hub and a deduplicated AI trigger.
func TestExampleLifecycle(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	raw, err := os.ReadFile("../../../docs/generated/workflows/example-lifecycle.workflow.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := runtime.DecodeDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	engine := runtime.New(core, []runtime.Definition{definition})
	entries := append(example.New(pool).Entries(), (&orchmodule.Module{Engine: engine}).Entries()...)
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool)})
	attach := func() {
		t.Helper()
		if err := engine.Attach(g, catalog); err != nil {
			t.Fatal(err)
		}
		engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) { return &workflowModel{}, nil }
	}
	attach()
	principal, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, principal)
	result, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: contracts.Input{"flow": definition.Name, "version": definition.Version, "input": map[string]any{"title": "complete lifecycle"}}, Origin: contracts.OriginHumanUI, IdempotencyKey: "lifecycle"})
	if err != nil || result.Status != contracts.StatusSucceeded {
		t.Fatalf("start: %+v %v", result, err)
	}
	var accepted struct {
		RunID int64 `json:"runId"`
	}
	data, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &accepted); err != nil {
		t.Fatal(err)
	}
	tick := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, "UPDATE orch_checkpoints SET lease_until='-infinity' WHERE tenant_id='tenant-a' AND run_id=$1", accepted.RunID); err != nil {
			t.Fatal(err)
		}
		if err := engine.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	tick()
	cards, err := g.Approvals(ctx)
	if err != nil || len(cards) != 1 {
		t.Fatalf("expected one archive approval: %v cards=%d", err, len(cards))
	}
	if cards[0].Capability != "example.record.archive" {
		t.Fatal("agent review replaced the required human gate")
	}
	var recordID int64
	var status string
	if err = pool.QueryRow(ctx, "SELECT id,status FROM example_records WHERE title='complete lifecycle'").Scan(&recordID, &status); err != nil || status != "draft" {
		t.Fatalf("archive executed before approval: %s %v", status, err)
	}
	if _, err = g.Decide(ctx, cards[0].ID, true, "reviewed"); err != nil {
		t.Fatal(err)
	}
	engine = runtime.New(core, []runtime.Definition{definition})
	attach()
	tick()
	summary, err := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.get", Input: contracts.Input{"runId": accepted.RunID}, Origin: contracts.OriginHumanUI})
	if err != nil || summary.Data.(map[string]any)["status"] != "completed" {
		t.Fatalf("resume: %+v %v", summary, err)
	}
	var records, archivedRecords int
	if err = pool.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE status='archived') FROM example_records WHERE title='complete lifecycle'").Scan(&records, &archivedRecords); err != nil || records != 1 || archivedRecords != 1 {
		t.Fatalf("business state not committed exactly once: records=%d archived=%d %v", records, archivedRecords, err)
	}
	var creates, reads, archives int
	if err = pool.QueryRow(ctx, "SELECT count(*) FILTER(WHERE capability='example.record.create'),count(*) FILTER(WHERE capability='example.record.get'),count(*) FILTER(WHERE capability='example.record.archive') FROM cap_invocations WHERE tenant_id='tenant-a' AND correlation_id=$1 AND status='succeeded'", result.TraceID).Scan(&creates, &reads, &archives); err != nil || creates != 1 || reads != 2 || archives != 1 {
		t.Fatalf("unexpected node executions create=%d read=%d archive=%d: %v", creates, reads, archives, err)
	}
	// Details retain account ownership even when the caller claims admin status.
	foreignAccount := *principal.Account
	foreignAccount.ID++
	foreign, err := identity.NewPrincipal(&foreignAccount, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreignResult, err := g.Invoke(identity.WithPrincipal(ctx, foreign), gateway.Request{Name: "example.record.get", Input: contracts.Input{"id": recordID}, Origin: contracts.OriginHumanUI})
	if err != nil || foreignResult.Status != contracts.StatusFailed || foreignResult.ErrorCode != "not_found" {
		t.Fatalf("cross-account detail visible: %+v %v", foreignResult, err)
	}

	hub := eventhub.NewHub()
	events := make(chan eventhub.Event, 8)
	hub.Subscribe(func(_ context.Context, event eventhub.Event) { events <- event })
	if _, err = outbox.NewRelay(pool, hub).Once(ctx); err != nil {
		t.Fatal(err)
	}
	var archived eventhub.Event
	for archived.EventID == "" {
		select {
		case event := <-events:
			if event.Type == example.ArchivedEvent {
				archived = event
			}
		case <-ctx.Done():
			t.Fatal("archived event not delivered")
		}
	}
	if archived.CorrelationID != result.TraceID || archived.InvocationID == "" {
		t.Fatal("event lost its causal chain")
	}
	if _, err = pool.Exec(ctx, "INSERT INTO ai_trigger_policies(tenant_id,policy_id,version,enabled,event_type,agent,mode,instruction,max_depth) VALUES('tenant-a','lifecycle-after-archive',1,true,$1,'workspace_agent','async','Inspect archived record',8)", example.ArchivedEvent); err != nil {
		t.Fatal(err)
	}
	launcher := &lifecycleLauncher{g: g, r: catalog, runs: harness.NewStore(pool), done: make(chan error, 1)}
	trigger := triggers.NewEngine(triggers.NewStore(pool), hub, launcher, iam)
	trigger.Handle(ctx, archived)
	trigger.Handle(ctx, archived) // at-least-once redelivery
	select {
	case err := <-launcher.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("event-triggered agent did not finish")
	}
	// Wait on the durable outcome, not a fixed sleep: the trigger finalizes after
	// the launcher returns and has its own lifecycle independent of the business.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM ai_trigger_executions WHERE tenant_id='tenant-a' AND policy_id='lifecycle-after-archive' AND event_id=$1 AND status='completed'", archived.EventID).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("trigger outcome not committed")
		}
	}
	var agents, toolReads int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM ai_runs WHERE agent='lifecycle-event-review'").Scan(&agents); err != nil || agents != 1 {
		t.Fatalf("duplicate AI launch: %d %v", agents, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM cap_invocations WHERE capability='example.record.get' AND origin='agent' AND correlation_id=$1 AND status='succeeded'", result.TraceID).Scan(&toolReads); err != nil || toolReads != 2 {
		t.Fatalf("AI did not call capability through Gateway: %d %v", toolReads, err)
	}
}

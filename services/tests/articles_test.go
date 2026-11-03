//go:build integration

package tests

import (
	"baseline/services/api/applications/articles"
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	runtime "baseline/services/api/kernel/workflow"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	orchmodule "baseline/services/api/system/orchestration"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/jackc/pgx/v5/pgxpool"
)

// articleModel is a controllable reviewer: it reads the article through the
// provided tool, then answers with the structured verdict the flow declares.
type articleModel struct {
	decision string
	tools    []*schema.ToolInfo
}

func (m *articleModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &articleModel{decision: m.decision, tools: tools}, nil
}

func (m *articleModel) Generate(_ context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	code := "safe"
	if m.decision == "reject" {
		code = "blocked"
	}
	for _, msg := range in {
		if msg.Role == schema.Tool {
			return schema.AssistantMessage(fmt.Sprintf(`{"code":%q}`, code), nil), nil
		}
	}
	args := "{}"
	for _, msg := range in {
		if msg.Role == schema.User {
			args = msg.Content
		}
	}
	tools := model.GetCommonOptions(&model.Options{Tools: m.tools}, opts...).Tools
	name := "article_get"
	if len(tools) > 0 {
		name = tools[0].Name
	}
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: name, Arguments: args}}}), nil
}

func (m *articleModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream unused")
}

// runState renders the durable run header and the node calls that reached the
// gateway, so a branch failure is diagnosable from the test output.
func runState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, correlation string, runID int64) string {
	t.Helper()
	var status, code string
	if err := pool.QueryRow(ctx, "SELECT status,error_code FROM ai_runs WHERE id=$1", runID).Scan(&status, &code); err != nil {
		return "run query failed: " + err.Error()
	}
	rows, err := pool.Query(ctx, "SELECT capability,status FROM cap_invocations WHERE tenant_id='tenant-a' AND correlation_id=$1 ORDER BY id", correlation)
	if err != nil {
		return fmt.Sprintf("run=%s/%s nodes=?", status, code)
	}
	defer rows.Close()
	nodes := ""
	for rows.Next() {
		var name, state string
		if rows.Scan(&name, &state) == nil {
			nodes += fmt.Sprintf("%s:%s,", name, state)
		}
	}
	return fmt.Sprintf("run=%s/%s nodes=%s", status, code, nodes)
}

// pendingPublish returns the newest publish gate waiting for a human.
func pendingPublish(t *testing.T, g *gateway.Gateway, ctx context.Context) approval.Approval {
	t.Helper()
	cards, err := g.Approvals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var picked approval.Approval
	for _, card := range cards {
		if card.Capability == "article.publish" && card.Status == approval.Pending && card.ID > picked.ID {
			picked = card
		}
	}
	if picked.ID == 0 {
		t.Fatal("no publish gate is waiting")
	}
	return picked
}

// TestArticlePublishFlow runs the shipped article-publish definition through the
// real engine and gateway: save -> agent review -> exactly one of two branches,
// with publishing gated by a second human.
func TestArticlePublishFlow(t *testing.T) {
	handler, pool, root := iamFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})

	blocked, configured := false, true
	module := &articles.Module{Pool: pool}
	module.Blocked = func(context.Context, *identity.Principal, string) (bool, bool, error) {
		return blocked, configured, nil
	}

	raw, err := os.ReadFile("../../docs/generated/workflows/article-publish.workflow.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := runtime.DecodeDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	engine := runtime.New(core, []runtime.Definition{definition})
	entries := append(module.Entries(), (&orchmodule.Module{Engine: engine}).Entries()...)
	catalog := registry.New(entries)
	if catalog.Err() != nil {
		t.Fatal(catalog.Err())
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool)})
	if err = engine.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	decision := "approve"
	engine.AgentModel = func(context.Context) (model.ToolCallingChatModel, error) {
		return &articleModel{decision: decision}, nil
	}
	principal, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, principal)

	// A second account holds capability:approve and is not the applicant, which is
	// what makes the publishing gate a real four-eyes check.
	owner := loginIAM(t, handler, "root-admin")
	response := owner.invoke(t, "identity.account.create", map[string]any{
		"username": "article-approver", "fullName": "Article Approver",
		"organizationId": root.OrganizationID, "departmentId": root.DepartmentID,
		"status": "active", "roleIds": []int64{},
		"overrides": []map[string]any{{"permission": "capability:approve", "effect": "allow", "scope": "ALL"}},
		"password":  "TestOnly!2026Secure",
	}, true)
	checkStatus(t, response, 200)
	approver := decodeData[genidentity.AccountRecord](t, response)
	approverPrincipal, err := iam.PrincipalForTask(ctx, "tenant-a", approver.ID, approver.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	approverCtx := identity.WithPrincipal(context.Background(), approverPrincipal)

	create := func(title, body string) int64 {
		t.Helper()
		result, e := g.Invoke(ctx, gateway.Request{Name: "article.create", Input: contracts.Input{"title": title, "body": body}, Origin: contracts.OriginHumanUI, IdempotencyKey: "create-" + fmt.Sprint(time.Now().UnixNano())})
		if e != nil || result.Status != contracts.StatusSucceeded {
			t.Fatalf("create: %+v %v", result, e)
		}
		data, _ := json.Marshal(result.Data)
		var created struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(data, &created) != nil {
			t.Fatal("create result has no id")
		}
		return created.ID
	}
	publish := func(id int64) (*contracts.Result, int64) {
		t.Helper()
		result, e := g.Invoke(ctx, gateway.Request{Name: "orchestration.workflow.run", Input: contracts.Input{"flow": definition.Name, "version": definition.Version, "input": map[string]any{"id": id, "revision": int64(1)}}, Origin: contracts.OriginHumanUI, IdempotencyKey: fmt.Sprint("run-", time.Now().UnixNano())})
		if e != nil || result.Status != contracts.StatusSucceeded {
			t.Fatalf("start: %+v %v", result, e)
		}
		raw, _ := json.Marshal(result.Data)
		var accepted struct {
			RunID int64 `json:"runId"`
		}
		if json.Unmarshal(raw, &accepted) != nil {
			t.Fatal("run result has no id")
		}
		return result, accepted.RunID
	}
	tick := func(runID int64) {
		t.Helper()
		if _, e := pool.Exec(ctx, "UPDATE orch_checkpoints SET lease_until='-infinity' WHERE tenant_id='tenant-a' AND run_id=$1", runID); e != nil {
			t.Fatal(e)
		}
		if e := engine.Tick(ctx); e != nil {
			t.Fatal(e)
		}
	}
	status := func(id int64) (string, string) {
		t.Helper()
		var state, code string
		if e := pool.QueryRow(ctx, "SELECT status,review_code FROM articles WHERE id=$1", id).Scan(&state, &code); e != nil {
			t.Fatal(e)
		}
		return state, code
	}
	branch := func(correlation string) (published, rejected int) {
		t.Helper()
		if e := pool.QueryRow(ctx, "SELECT count(*) FILTER(WHERE capability='article.publish' AND status='succeeded'),count(*) FILTER(WHERE capability='article.reject' AND status='succeeded') FROM cap_invocations WHERE tenant_id='tenant-a' AND correlation_id=$1", correlation).Scan(&published, &rejected); e != nil {
			t.Fatal(e)
		}
		return published, rejected
	}

	t.Run("approve waits for a human, then publishes", func(t *testing.T) {
		decision, blocked, configured = "approve", false, true
		id := create("Approved article", "private approved body")
		started, runID := publish(id)
		tick(runID)
		// The gate holds: the AI verdict alone never makes content public.
		if state, code := status(id); state != "pending_ai" || code != "safe" {
			t.Fatalf("state=%s code=%s %s", state, code, runState(t, ctx, pool, started.TraceID, runID))
		}
		if published, _ := branch(started.TraceID); published != 0 {
			t.Fatal("published without a human gate")
		}
		cards, e := g.Approvals(ctx)
		if e != nil || len(cards) != 1 || cards[0].Capability != "article.publish" || cards[0].Kind != string(contracts.ApprovalConfirm) {
			t.Fatalf("expected one publish confirmation: %+v %v", cards, e)
		}
		// A second account with capability:approve decides the gate.
		if _, e = g.Decide(approverCtx, cards[0].ID, true, "looks fine"); e != nil {
			t.Fatal(e)
		}
		tick(runID)
		if state, code := status(id); state != "published" || code != "safe" {
			t.Fatalf("state=%s code=%s %s", state, code, runState(t, ctx, pool, started.TraceID, runID))
		}
		published, rejected := branch(started.TraceID)
		if published != 1 || rejected != 0 {
			t.Fatalf("branch ran publish=%d reject=%d", published, rejected)
		}
		// Body stays out of the call audit and result projections.
		var leaks int
		if e := pool.QueryRow(ctx, "SELECT count(*) FROM cap_invocations WHERE input::text LIKE '%private approved body%' OR result::text LIKE '%private approved body%'").Scan(&leaks); e != nil || leaks != 0 {
			t.Fatalf("body leaked: %d %v", leaks, e)
		}
	})

	t.Run("reject rejects and never publishes", func(t *testing.T) {
		decision, blocked, configured = "reject", false, true
		id := create("Rejected article", "questionable body")
		started, runID := publish(id)
		tick(runID)
		if state, code := status(id); state != "rejected" || code != "blocked" {
			t.Fatalf("state=%s code=%s", state, code)
		}
		published, rejected := branch(started.TraceID)
		if published != 0 || rejected != 1 {
			t.Fatalf("branch ran publish=%d reject=%d", published, rejected)
		}
	})

	t.Run("blocked word overrides a lenient verdict", func(t *testing.T) {
		decision, blocked, configured = "approve", true, true
		id := create("Forbidden article", "contains a blocked phrase")
		started, runID := publish(id)
		tick(runID)
		if state, _ := status(id); state != "pending_ai" {
			t.Fatalf("state=%s, an overridden verdict must not publish", state)
		}
		published, rejected := branch(started.TraceID)
		if published != 0 || rejected != 0 {
			t.Fatalf("branch ran on an invalid verdict publish=%d reject=%d", published, rejected)
		}
	})

	t.Run("missing word list still needs the human gate", func(t *testing.T) {
		decision, blocked, configured = "approve", false, false
		id := create("Unconfigured policy", "any body")
		started, runID := publish(id)
		tick(runID)
		// No word list means no deterministic rule; the human gate still decides.
		if state, _ := status(id); state != "pending_ai" {
			t.Fatalf("state=%s, content must not go live before approval", state)
		}
		if published, _ := branch(started.TraceID); published != 0 {
			t.Fatal("published without a human gate")
		}
		if _, e := g.Decide(approverCtx, pendingPublish(t, g, ctx).ID, true, "checked"); e != nil {
			t.Fatal(e)
		}
		tick(runID)
		if state, code := status(id); state != "published" || code != "safe" {
			t.Fatalf("state=%s code=%s", state, code)
		}
	})

	t.Run("publish is refused outside the flow", func(t *testing.T) {
		decision, blocked, configured = "approve", false, true
		id := create("Direct publish", "body")
		if result, e := g.Invoke(ctx, gateway.Request{Name: "article.submit", Input: contracts.Input{"id": id, "revision": int64(1)}, Origin: contracts.OriginHumanUI, IdempotencyKey: "submit-direct"}); e != nil || result.Status != contracts.StatusSucceeded {
			t.Fatalf("submit: %+v %v", result, e)
		}
		result, e := g.Invoke(ctx, gateway.Request{Name: "article.publish", Input: contracts.Input{"id": id, "revision": int64(1)}, Origin: contracts.OriginHumanUI, IdempotencyKey: "forged-publish"})
		if e == nil && result.Status == contracts.StatusSucceeded {
			t.Fatal("a page call published without a review")
		}
		// Even a properly approved direct call cannot publish: no verdict was
		// recorded for this revision, so the conditional update matches nothing.
		cards, e := g.Approvals(ctx)
		if e != nil {
			t.Fatal(e)
		}
		for _, card := range cards {
			if card.Capability == "article.publish" && card.Status == approval.Pending {
				if _, e = g.Decide(approverCtx, card.ID, true, "no verdict"); e != nil && contracts.Code(e) != "conflict" {
					t.Fatalf("unexpected approval failure: %v", e)
				}
			}
		}
		if state, code := status(id); state != "pending_ai" || code != "" {
			t.Fatalf("state=%s code=%s, unpublished content must not go live", state, code)
		}
	})

	t.Run("human decision stays human only", func(t *testing.T) {
		id := create("Human decision", "body")
		if result, e := g.Invoke(ctx, gateway.Request{Name: "article.decide", Input: contracts.Input{"id": id, "revision": int64(1), "approve": true}, Origin: contracts.OriginAgent, IdempotencyKey: "agent-decide"}); e == nil && result.Status == contracts.StatusSucceeded {
			t.Fatal("automation decided a human review")
		}
	})
}

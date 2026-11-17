//go:build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"horizon/foundation/ai/harness"
	genai "horizon/foundation/gen/ai"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedRun records one task so a conversation exists. The workbench derives its
// list from the runs, so the storage of a title only matters once a run is
// there.
func seedRun(t *testing.T, pool *pgxpool.Pool, tenant string, accountID int64, session, question string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO ai_runs (tenant_id, account_id, session_id, agent, model, instruction_version, question, trace_id)
		VALUES ($1, $2, $3, 'workspace_agent', 'glm-test', 'test', $4, 'trace-test')`,
		tenant, accountID, session, question)
	if err != nil {
		t.Fatal(err)
	}
}

func conversationOf(t *testing.T, list []*genai.AIConversation, session string) *genai.AIConversation {
	t.Helper()
	for _, item := range list {
		if item.SessionID == session {
			return item
		}
	}
	t.Fatalf("conversation %s missing from %d entries", session, len(list))
	return nil
}

func TestAIConversationRenameAndArchive(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	const session = "conv-rename-archive"

	seedRun(t, pool, "tenant-a", root.ID, session, "first question")
	seedRun(t, pool, "tenant-a", root.ID, session, "second question")

	list := decodeBody[[]*genai.AIConversation](t, owner.request(t, "GET", "/api/ai/conversations", nil, false))
	initial := conversationOf(t, list, session)
	if initial.Title != "first question" || initial.Archived || initial.MessageCount != 2 {
		t.Fatalf("unexpected derived conversation: %+v", initial)
	}

	renamed := decodeBody[*genai.AIConversation](t, owner.request(t, "PATCH", "/api/ai/conversations/"+session,
		map[string]any{"title": "Line 3 changeover"}, true))
	if renamed.Title != "Line 3 changeover" || renamed.Archived {
		t.Fatalf("rename not reflected: %+v", renamed)
	}

	archived := decodeBody[*genai.AIConversation](t, owner.request(t, "PATCH", "/api/ai/conversations/"+session,
		map[string]any{"archived": true}, true))
	if !archived.Archived || archived.Title != "Line 3 changeover" {
		t.Fatalf("archive must keep the renamed title: %+v", archived)
	}
	// An archived conversation stays readable: the turns are not deleted.
	list = decodeBody[[]*genai.AIConversation](t, owner.request(t, "GET", "/api/ai/conversations", nil, false))
	if !conversationOf(t, list, session).Archived {
		t.Fatal("archived conversation must stay in the owner's list")
	}
	runs := decodeBody[[]*genai.AIRun](t, owner.request(t, "GET", "/api/ai/runs?sessionId="+session, nil, false))
	if len(runs) != 2 {
		t.Fatalf("archive must not delete turns: %d", len(runs))
	}

	restored := decodeBody[*genai.AIConversation](t, owner.request(t, "PATCH", "/api/ai/conversations/"+session,
		map[string]any{"archived": false}, true))
	if restored.Archived {
		t.Fatalf("restore not reflected: %+v", restored)
	}

	// Rename and archive share one row per session: a second write must update
	// it instead of forking a second title that disagrees with the first.
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_conversations WHERE tenant_id = $1 AND account_id = $2 AND session_id = $3`,
		"tenant-a", root.ID, session).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("conversation row count = %d (%v)", rows, err)
	}
}

func TestAIConversationWriteBoundaries(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	const session = "conv-owned-by-someone-else"

	// A session of another account in the same tenant must be invisible, not
	// writable: the owner predicate lives in the query, not in the browser.
	seedRun(t, pool, "tenant-a", root.ID+9000, session, "foreign question")

	foreign := owner.request(t, "PATCH", "/api/ai/conversations/"+session, map[string]any{"title": "hijacked"}, true)
	checkStatus(t, foreign, 404)
	if code := decodeBody[genai.IAMProblem](t, foreign).Code; code != "not_found" {
		t.Fatalf("foreign conversation code = %q", code)
	}

	missing := owner.request(t, "PATCH", "/api/ai/conversations/no-such-session", map[string]any{"archived": true}, true)
	checkStatus(t, missing, 404)

	const own = "conv-own"
	seedRun(t, pool, "tenant-a", root.ID, own, "own question")

	// A blank title and an empty update are both refused by the contract, not
	// silently turned into a no-op.
	checkStatus(t, owner.request(t, "PATCH", "/api/ai/conversations/"+own, map[string]any{"title": "  "}, true), 400)
	checkStatus(t, owner.request(t, "PATCH", "/api/ai/conversations/"+own, map[string]any{}, true), 400)

	anonymous := iamClient{handler: handler}
	checkStatus(t, anonymous.request(t, "PATCH", "/api/ai/conversations/"+own, map[string]any{"archived": true}, true), 401)

	// A refused write leaves no row behind.
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_conversations WHERE tenant_id = $1`, "tenant-a").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("refused writes must not create rows: %d (%v)", rows, err)
	}
}

func TestAutomationConversationUsesExistingWorkbenchHistory(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := t.Context()
	const session = "automation:11"
	store := harness.NewStore(pool)
	run, err := store.Start(ctx, harness.Run{
		TenantID: "tenant-a", AccountID: root.ID, DepartmentID: root.DepartmentID,
		SessionID: session, ConversationTitle: "Daily brief", Origin: "automation",
		Agent: "workspace_agent", Model: "test", InstructionVersion: "test", Question: "Report on orders", TraceID: session,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, "tenant-a", run.ID, harness.Result{Status: harness.StatusCompleted, Answer: "Orders increased."}); err != nil {
		t.Fatal(err)
	}
	seedRun(t, pool, "tenant-a", root.ID, session, "What changed?")
	list := decodeBody[[]*genai.AIConversation](t, owner.request(t, "GET", "/api/ai/conversations", nil, false))
	result := conversationOf(t, list, session)
	if result.Title != "Daily brief" || result.AutomationAt == nil || result.MessageCount != 2 {
		t.Fatalf("unexpected automation conversation: %+v", result)
	}
	runs := decodeBody[[]*genai.AIRun](t, owner.request(t, "GET", "/api/ai/runs?sessionId="+session, nil, false))
	if len(runs) != 2 || runs[0].Answer != "Orders increased." || runs[1].Question != "What changed?" {
		t.Fatalf("unexpected conversation turns: %+v", runs)
	}

	const foreign = "automation:12"
	seedRun(t, pool, "tenant-a", root.ID+9000, foreign, "private briefing")
	foreignRuns := decodeBody[[]*genai.AIRun](t, owner.request(t, "GET", "/api/ai/runs?sessionId="+foreign, nil, false))
	if len(foreignRuns) != 0 {
		t.Fatal("foreign automation was exposed")
	}
	continued := owner.request(t, "POST", "/api/ai/runs", map[string]any{"question": "Tell me more", "sessionId": foreign}, true)
	checkStatus(t, continued, 404)
}

func TestAutomationConversationBackfillsHistoricalRun(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := t.Context()
	task := decodeData[struct{ ID int64 }](t, owner.invoke(t, "automation.task.create", map[string]any{
		"name": "Historical brief", "description": "Report on orders",
		"scheduleKind": "daily", "time": "09:00", "timezone": "Asia/Shanghai",
	}, true))
	old, err := harness.NewStore(pool).Start(ctx, harness.Run{
		TenantID: "tenant-a", AccountID: root.ID, DepartmentID: root.DepartmentID, Origin: "automation",
		Agent: "workspace_agent", Model: "test", InstructionVersion: "test", Question: "Report on orders", TraceID: "historical",
	})
	if err != nil {
		t.Fatal(err)
	}
	var runID int64
	err = pool.QueryRow(ctx, `INSERT INTO automation_runs
		(tenant_id, task_id, occurrence_key, scheduled_for, trigger_kind, status, ai_run_id)
		VALUES ('tenant-a', $1, 'legacy', now(), 'schedule', 'completed', $2) RETURNING id`, task.ID, old.ID).Scan(&runID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../foundation/db/migrations/00046_automation_conversations.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	for range 2 {
		if _, err = pool.Exec(ctx, up); err != nil {
			t.Fatal(err)
		}
	}
	conversations := decodeBody[[]*genai.AIConversation](t, owner.request(t, "GET", "/api/ai/conversations", nil, false))
	item := conversationOf(t, conversations, fmt.Sprintf("automation:%d", runID))
	if item.Title != "Historical brief" || item.MessageCount != 1 || item.AutomationAt == nil {
		t.Fatalf("historical conversation was not attached: %+v", item)
	}
}

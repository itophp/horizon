//go:build integration

package tests

import (
	"context"
	"testing"

	genai "baseline/services/api/gen/ai"

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

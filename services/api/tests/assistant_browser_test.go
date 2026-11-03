//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	genassistant "baseline/services/api/gen/assistant"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
)

func TestEmbeddedWorkspaceAgentUsesBrowserToolAndResumesSameTask(t *testing.T) {
	handler, pool, root := iamFixture(t)
	ctx := context.Background()
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role       string `json:"role"`
				Content    string `json:"content"`
				ToolCallID string `json:"tool_call_id"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("bad model request")
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"id":"model1","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"page_call_1","type":"function","function":{"name":"webmcp_baseline_page_context","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`))
		} else {
			found := false
			for _, m := range body.Messages {
				if m.Role == "tool" && m.ToolCallID == "page_call_1" && strings.Contains(m.Content, "/accounts") {
					found = true
				}
			}
			if !found {
				t.Error("resumed model did not receive browser result")
			}
			_, _ = w.Write([]byte(`{"id":"model2","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"You are on the accounts page."},"finish_reason":"stop"}]}`))
		}
	}))
	defer upstream.Close()
	core := governance.New(pool, nil, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	secret, err := core.Seal("tenant-a", "workspace:0", "assistant.token", `"test-only-token"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id) VALUES('tenant-a','workspace:0','workspace',0) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{"assistant.enabled": true, "agent.enabled": true, "agent.baseUrl": upstream.URL + "/v1", "assistant.model": "test", "assistant.token": secret}
	for key, value := range settings {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO gov_values(tenant_id,scope_key,config_key,value_json) VALUES('tenant-a','workspace:0',$1,$2) ON CONFLICT(tenant_id,scope_key,config_key) DO UPDATE SET value_json=EXCLUDED.value_json`, key, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	owner := loginIAM(t, handler, "root-admin")
	question := "Which page am I on?"
	first := owner.request(t, "POST", "/api/assistant/agent", map[string]any{"question": question, "sessionId": "embedded-test", "tools": []map[string]any{{"name": "baseline_page_context", "description": "Current page", "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}}}, true)
	checkStatus(t, first, 200)
	pending := decodeBody[genassistant.AssistantAgentReply](t, first)
	if pending.RunID == nil || pending.ResumeToken == nil || pending.PageCall == nil || pending.PageCall.Name != "baseline_page_context" {
		t.Fatalf("missing browser suspension: %+v", pending)
	}
	var storedStatus string
	if err = pool.QueryRow(ctx, "SELECT status FROM ai_runs WHERE tenant_id='tenant-a' AND account_id=$1 AND id=$2", root.ID, *pending.RunID).Scan(&storedStatus); err != nil || storedStatus != "running" {
		t.Fatalf("task not running: %s %v", storedStatus, err)
	}
	resume := map[string]any{"question": question, "sessionId": "embedded-test", "runId": *pending.RunID, "resumeToken": *pending.ResumeToken, "toolCallId": "wrong-call", "toolResult": `{"route":"/accounts"}`}
	resume["runId"] = *pending.RunID + 1000000
	checkStatus(t, owner.request(t, "POST", "/api/assistant/agent", resume, true), 400)
	resume["runId"] = *pending.RunID
	checkStatus(t, owner.request(t, "POST", "/api/assistant/agent", resume, true), 400)
	resume["toolCallId"] = pending.PageCall.ID
	second := owner.request(t, "POST", "/api/assistant/agent", resume, true)
	checkStatus(t, second, 200)
	final := decodeBody[genassistant.AssistantAgentReply](t, second)
	if final.RunID == nil || *final.RunID != *pending.RunID || final.PageCall != nil || final.Content != "You are on the accounts page." || calls.Load() != 2 {
		t.Fatalf("did not resume the same task: %+v", final)
	}
	if err = pool.QueryRow(ctx, "SELECT status FROM ai_runs WHERE tenant_id='tenant-a' AND account_id=$1 AND id=$2", root.ID, *pending.RunID).Scan(&storedStatus); err != nil || storedStatus != "completed" {
		t.Fatalf("task not completed: %s %v", storedStatus, err)
	}
	checkStatus(t, owner.request(t, "POST", "/api/assistant/agent", resume, true), 400)
}

func TestBrowserAgentResumeLeaseIsScopedSingleUseAndExpiring(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	q := dbgen.New(pool)
	run, err := harness.NewStore(pool).Start(ctx, harness.Run{TenantID: "tenant-a", AccountID: root.ID, DepartmentID: root.DepartmentID, Agent: "workspace_agent", Question: "test", Model: "test", TraceID: "browser-test"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := q.AIWaitForPageTool(ctx, dbgen.AIWaitForPageToolParams{TenantID: "tenant-a", AccountID: root.ID, DepartmentID: root.DepartmentID, ID: run.ID, Payload: "encrypted-payload", Token: "single-use"})
	if err != nil || rows != 1 {
		t.Fatalf("wait: %d %v", rows, err)
	}
	owner := dbgen.AIClaimPageToolParams{TenantID: "tenant-a", AccountID: root.ID, DepartmentID: root.DepartmentID, RunID: run.ID, Token: "single-use"}
	for _, change := range []func(*dbgen.AIClaimPageToolParams){func(p *dbgen.AIClaimPageToolParams) { p.TenantID = "other" }, func(p *dbgen.AIClaimPageToolParams) { p.AccountID++ }, func(p *dbgen.AIClaimPageToolParams) { p.DepartmentID++ }, func(p *dbgen.AIClaimPageToolParams) { p.Token = "wrong" }} {
		other := owner
		change(&other)
		if _, err = q.AIClaimPageTool(ctx, other); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("foreign or invalid resume accepted: %v", err)
		}
	}
	value, err := q.AIClaimPageTool(ctx, owner)
	if err != nil || value != "encrypted-payload" {
		t.Fatalf("owner resume: %v", err)
	}
	if _, err = q.AIClaimPageTool(ctx, owner); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("replay accepted: %v", err)
	}
	owner.Token = "claimed"
	if _, err = q.AIClaimPageTool(ctx, owner); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("claimed lease accepted as a resume token: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE orch_checkpoints SET lease_token='expired', lease_until=$1 WHERE tenant_id='tenant-a' AND run_id=$2", time.Now().Add(-time.Minute), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	owner.Token = "expired"
	if _, err = q.AIClaimPageTool(ctx, owner); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired resume accepted: %v", err)
	}
	if err = q.AIExpirePageTools(ctx, dbgen.AIExpirePageToolsParams{TenantID: "tenant-a", AccountID: root.ID}); err != nil {
		t.Fatal(err)
	}
	stored, err := harness.NewStore(pool).Get(ctx, "tenant-a", root.ID, run.ID)
	if err != nil || stored.Status != "failed" || stored.ErrorCode != "timeout" {
		t.Fatalf("abandoned task not closed: %+v %v", stored, err)
	}
}

//go:build integration

package tests

import (
	businessmcp "baseline/services/api/internal/modules/mcp"
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestArticlesMCPSubmission(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	token := decodeData[businessmcp.MCPTokenCreated](t, owner.invoke(t, "mcp.token.create", map[string]any{"name": "article client", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID, "tools": []string{"articles_list", "articles_get", "articles_create", "articles_submit"}}, true))
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := sdk.NewClient(&sdk.Implementation{Name: "article-test", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: workflowBearer{token: token.Secret}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 4 {
		t.Fatalf("tool scope: %v", err)
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if e != nil || res.IsError {
			t.Fatalf("call %s: %v %+v", name, e, res)
		}
		raw, e := json.Marshal(res.StructuredContent)
		if e != nil {
			t.Fatal(e)
		}
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if e = json.Unmarshal(raw, &envelope); e != nil {
			t.Fatal(e)
		}
		return envelope.Data
	}
	create := map[string]any{"title": "MCP article", "body": "private MCP body", "idempotencyKey": "article-create"}
	first := call("articles_create", create)
	second := call("articles_create", create)
	if first["id"] != second["id"] {
		t.Fatal("creation not idempotent")
	}
	detail := call("articles_get", map[string]any{"id": first["id"]})
	if detail["status"] != "draft" {
		t.Fatal("draft published")
	}
	call("articles_submit", map[string]any{"id": first["id"], "revision": detail["revision"], "idempotencyKey": "article-submit"})
	detail = call("articles_get", map[string]any{"id": first["id"]})
	if detail["status"] != "pending_ai" {
		t.Fatal("MCP bypassed moderation")
	}
	listed := call("articles_list", map[string]any{})
	if len(listed["items"].([]any)) != 1 {
		t.Fatal("article missing")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM cap_outbox WHERE event_type='article.submitted'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("submission event %d %v", count, err)
	}
	if res, e := client.CallTool(ctx, &sdk.CallToolParams{Name: "article.decide", Arguments: map[string]any{}}); e == nil && !res.IsError {
		t.Fatal("manual review exposed")
	}
	checkSucceeded(t, owner.invoke(t, "mcp.token.revoke", map[string]any{"id": token.Token.ID, "password": "TestOnly!2026Secure"}, true))
	if _, e := client.ListTools(ctx, nil); e == nil {
		t.Fatal("revoked credential usable")
	}
}

//go:build integration

package tests

import (
	gen "baseline/services/api/gen/capability"
	businessmcp "baseline/services/api/integrations/mcp"
	"baseline/services/api/kernel/capability/contracts"
	"context"
	"encoding/json"
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type workflowBearer struct{ token string }

func (b workflowBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(copy)
}

func TestOrchestrationMCPProtocolAndApproval(t *testing.T) {
	handler, _, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	token := decodeData[businessmcp.MCPTokenCreated](t, owner.invoke(t, "mcp.token.create", map[string]any{"name": "workflow client", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID, "tools": []string{"orchestration_workflow_list", "orchestration_workflow_get", "orchestration_workflow_run", "orchestration_workflow_cancel"}}, true))
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := sdk.NewClient(&sdk.Implementation{Name: "workflow-test", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: workflowBearer{token: token.Secret}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) <= 4 {
		t.Fatalf("token tool scope: %v %+v", err, tools)
	}

	call := func(name string, args map[string]any) contracts.Result {
		t.Helper()
		res, err := client.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("MCP %s failed: %+v", name, res.Content)
		}
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var envelope contracts.Result
		if err = json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	if result := call("orchestration_workflow_list", map[string]any{}); result.Status != contracts.StatusSucceeded {
		t.Fatal("workflow directory failed")
	}
	pending := call("orchestration_workflow_run", map[string]any{"flow": "example-create-archive", "version": "1.0.0", "input": map[string]any{"title": "MCP workflow"}})
	if pending.Status != contracts.StatusAwaitingApproval || pending.ApprovalID == 0 {
		t.Fatal("MCP start bypassed approval")
	}
	approved := decodeBody[gen.CapabilityResult](t, owner.request(t, "POST", fmt.Sprintf("/api/capabilities/approvals/%d/decisions", pending.ApprovalID), map[string]any{"approve": true}, true))
	if approved.Status != "succeeded" {
		t.Fatalf("start approval failed: %+v", approved)
	}
	id := approved.Data.(map[string]any)["runId"]
	if result := call("orchestration_workflow_get", map[string]any{"runId": id}); result.Status != contracts.StatusSucceeded {
		t.Fatal("run query failed")
	}
	pending = call("orchestration_workflow_cancel", map[string]any{"runId": id})
	if pending.Status != contracts.StatusAwaitingApproval {
		t.Fatal("MCP cancellation bypassed approval")
	}
	approved = decodeBody[gen.CapabilityResult](t, owner.request(t, "POST", fmt.Sprintf("/api/capabilities/approvals/%d/decisions", pending.ApprovalID), map[string]any{"approve": true}, true))
	if approved.Status != "succeeded" {
		t.Fatalf("cancel approval failed: %+v", approved)
	}
	if result := call("orchestration_workflow_get", map[string]any{"runId": id}); result.Data.(map[string]any)["status"] != "cancelled" {
		t.Fatal("cancelled state not visible through MCP")
	}
	checkSucceeded(t, owner.invoke(t, "mcp.token.revoke", map[string]any{"id": token.Token.ID, "password": "TestOnly!2026Secure"}, true))
	if _, err := client.ListTools(ctx, nil); err == nil {
		t.Fatal("revoked credential still authorized")
	}
}

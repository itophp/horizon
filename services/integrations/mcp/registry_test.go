package mcp

import (
	"context"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
	"testing"
)

func TestToolSchemasRejectMissingBusinessParameters(t *testing.T) {
	ctx := context.Background()
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	calls := 0
	for _, name := range Tools {
		registerReadTool(server, name, func(context.Context, toolInput) (*sdk.CallToolResult, interface{}, error) {
			calls++
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ok"}}}, nil, nil
		})
	}
	st, ct := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != len(Tools) {
		t.Fatal("tool catalog and registration drift")
	}
	for _, tool := range listed.Tools {
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != (tool.Name != "workflows_run" && tool.Name != "workflows_cancel" && tool.Name != "articles_create" && tool.Name != "articles_submit") {
			t.Fatal("missing read-only declaration")
		}
	}
	for _, name := range []string{"articles_get", "articles_create", "articles_submit", "dictionary_options", "notifications_get", "workflows_get", "workflows_run", "workflows_cancel"} {
		result, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: map[string]interface{}{}})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("missing required input accepted for %s", name)
		}
	}
	if calls != 0 {
		t.Fatal("invalid arguments reached application")
	}
	result, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "context_get", Arguments: map[string]interface{}{}})
	if err != nil || result.IsError {
		t.Fatalf("valid context call rejected: %v", err)
	}
	if calls != 1 {
		t.Fatal("valid tool did not reach application")
	}
}
func TestRateLimitsAreIndependentAndBounded(t *testing.T) {
	l := requestLimits{}
	if !l.allow("token:one", 1, 1) || l.allow("token:one", 1, 1) {
		t.Fatal("token burst not enforced")
	}
	if !l.allow("token:two", 1, 1) {
		t.Fatal("one token exhausted another")
	}
	for i := 0; i < 4094; i++ {
		if !l.allow(string(rune(i+1000)), 1, 1) {
			t.Fatal("capacity exhausted early")
		}
	}
	if l.allow("overflow", 1, 1) {
		t.Fatal("unbounded limiter storage")
	}
}

func TestDictionaryBoundaryMatchesCapabilityContract(t *testing.T) {
	// The dictionary capability is the only contract now: a tool input longer
	// than the declared maximum must be rejected before the domain is reached.
	for name, limit := range map[string]int{"code": 80, "search": 100} {
		in := toolInput{Code: "valid", Language: "en"}
		if name == "code" {
			in.Code = strings.Repeat("x", limit+1)
		} else {
			in.Search = strings.Repeat("x", limit+1)
		}
		if _, err := New(nil, nil).call(context.Background(), nil, "dictionary_options", in, nil, nil); err == nil || err.Error() != "invalid" {
			t.Fatalf("MCP did not enforce the %s maximum", name)
		}
	}
}

package mcp

import (
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
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
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("missing read-only declaration")
		}
	}
	for _, name := range []string{"dictionary_options", "notifications_get"} {
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

func TestDictionaryBoundaryMatchesGeneratedContract(t *testing.T) {
	raw, err := os.ReadFile("../../gen/http/openapi3.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string
					Schema struct{ MaxLength int }
				}
			}
		}
	}
	if err = json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	params := contract.Paths["/api/auth/dictionary-options/{code}"].Get.Parameters
	found := 0
	for _, parameter := range params {
		if parameter.Name != "code" && parameter.Name != "search" {
			continue
		}
		found++
		if parameter.Schema.MaxLength < 1 {
			t.Fatal("contract limit missing")
		}
		in := toolInput{Code: "valid", Language: "en"}
		if parameter.Name == "code" {
			in.Code = strings.Repeat("x", parameter.Schema.MaxLength+1)
		} else {
			in.Search = strings.Repeat("x", parameter.Schema.MaxLength+1)
		}
		if _, err := New(nil, nil).call(context.Background(), nil, "dictionary_options", in, nil, nil); err == nil || err.Error() != "invalid" {
			t.Fatalf("MCP did not enforce Goa %s maximum", parameter.Name)
		}
	}
	if found != 2 {
		t.Fatal("contract input mapping changed")
	}
}

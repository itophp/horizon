package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	genassistant "baseline/services/api/gen/assistant"
	"baseline/services/api/internal/governance"
)

func pointer(value string) *string { return &value }

func TestInputMapsRolesToResponseProtocolItems(t *testing.T) {
	service := &Service{}
	messages := []*genassistant.AssistantMessage{
		{Role: "system", Content: "be brief"},
		{Role: "user", Content: "where am I"},
		{Role: "assistant", Content: "", ToolCalls: []*genassistant.AssistantToolCall{{ID: "call_1", Name: "baseline_page_context", Arguments: "{}"}}},
		{Role: "tool", Content: `{"route":"/accounts"}`, ToolCallID: pointer("call_1")},
		{Role: "assistant", Content: "On the accounts page."},
		{Role: "tool", Content: "ignored"},
	}
	items, instructions := service.input(messages, "default")

	if instructions != "be brief" {
		t.Fatalf("system message must replace the default instruction, got %q", instructions)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 input items, got %d: %v", len(items), items)
	}
	if items[0]["role"] != "user" || items[0]["content"] != "where am I" {
		t.Fatalf("user turn mapped incorrectly: %v", items[0])
	}
	if items[1]["type"] != "function_call" || items[1]["call_id"] != "call_1" || items[1]["name"] != "baseline_page_context" {
		t.Fatalf("assistant tool call mapped incorrectly: %v", items[1])
	}
	if items[2]["type"] != "function_call_output" || items[2]["call_id"] != "call_1" {
		t.Fatalf("tool output mapped incorrectly: %v", items[2])
	}
	if items[3]["role"] != "assistant" || items[3]["content"] != "On the accounts page." {
		t.Fatalf("assistant text mapped incorrectly: %v", items[3])
	}
}

func TestSendUsesOpenAIResponseProtocol(t *testing.T) {
	var path, authorization string
	var body map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"glm-5.3-flash","output":[
			{"type":"reasoning","content":[]},
			{"type":"message","content":[{"type":"output_text","text":"你需要先登录。"}]},
			{"type":"function_call","call_id":"call_9","name":"baseline_navigate","arguments":"{\"path\":\"/accounts\"}"}
		]}`))
	}))
	defer upstream.Close()

	service := &Service{HTTP: upstream.Client()}
	raw, _ := json.Marshal(responseRequest{Model: "glm-5.3-flash", Input: []map[string]any{{"role": "user", "content": "hi"}}, Stream: false})
	reply, err := service.send(context.Background(), config{baseURL: upstream.URL + "/api/v1", model: "glm-5.3-flash", token: "deployment-token"}, raw)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if path != "/api/v1/responses" {
		t.Fatalf("unexpected endpoint %q", path)
	}
	if authorization != "Bearer deployment-token" {
		t.Fatalf("credential header missing: %q", authorization)
	}
	if body["model"] != "glm-5.3-flash" {
		t.Fatalf("model not forwarded: %v", body["model"])
	}
	if reply.Content != "你需要先登录。" {
		t.Fatalf("text output not extracted: %q", reply.Content)
	}
	if len(reply.ToolCalls) != 1 || reply.ToolCalls[0].ID != "call_9" || reply.ToolCalls[0].Name != "baseline_navigate" {
		t.Fatalf("function call not extracted: %+v", reply.ToolCalls)
	}
	if reply.ToolCalls[0].Arguments != `{"path":"/accounts"}` {
		t.Fatalf("arguments must pass through unchanged: %q", reply.ToolCalls[0].Arguments)
	}
}

func TestSendHidesUpstreamFailureDetail(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key sk-secret"}}`))
	}))
	defer upstream.Close()

	service := &Service{HTTP: upstream.Client()}
	_, err := service.send(context.Background(), config{baseURL: upstream.URL, model: "glm-5.3-flash", token: "x"}, []byte(`{}`))
	if err == nil || governance.Code(err) != "unavailable" {
		t.Fatalf("expected a stable unavailable code, got %v", err)
	}
	if strings.Contains(err.Error(), "sk-secret") || strings.Contains(err.Error(), "bad key") {
		t.Fatalf("upstream detail must not leak: %q", err.Error())
	}
}

func TestAssistantSettingsDefinition(t *testing.T) {
	for key, expected := range map[string]string{
		"assistant.enabled": "false",
		"assistant.baseUrl": `"https://open.bigmodel.cn/api/v1"`,
		"assistant.model":   `"glm-5.3-flash"`,
		"assistant.token":   `""`,
	} {
		definition := governance.FindDefinition(key)
		if definition == nil {
			t.Fatalf("%s is not registered", key)
		}
		if definition.Default != expected {
			t.Fatalf("%s default = %q, want %q", key, definition.Default, expected)
		}
		if !definition.UI {
			t.Fatalf("%s must stay visible in the settings form", key)
		}
	}
	if !governance.FindDefinition("assistant.token").Secret {
		t.Fatal("assistant.token must be stored as an encrypted secret")
	}
}

func TestAssistantBaseURLCannotReachArbitraryHosts(t *testing.T) {
	definition := governance.FindDefinition("assistant.baseUrl")
	cases := map[string]bool{
		`"https://open.bigmodel.cn/api/v1"`: true,
		`"http://127.0.0.1:9999/api/v1"`:    true,
		`"http://open.bigmodel.cn/api/v1"`:  false,
		`"https://user:pass@host/api"`:      false,
		`"https://host/api?x=1"`:            false,
		`"file:///etc/passwd"`:              false,
		`"https://host/api#frag"`:           false,
	}
	for raw, valid := range cases {
		err := definition.Validate(raw)
		if valid && err != nil {
			t.Fatalf("%s should be accepted: %v", raw, err)
		}
		if !valid && err == nil {
			t.Fatalf("%s should be rejected", raw)
		}
	}
}

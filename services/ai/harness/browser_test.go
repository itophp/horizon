package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"baseline/services/api/system/governance"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type memoryBrowserCheckpoint struct {
	mu     sync.Mutex
	values map[string][]byte
}

func (s *memoryBrowserCheckpoint) Set(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = append([]byte(nil), data...)
	return nil
}
func (s *memoryBrowserCheckpoint) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.values[key]
	return append([]byte(nil), data...), ok, nil
}

type browserScript struct {
	stage    int
	messages []*schema.Message
	bound    []string
}

func (s *browserScript) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	for _, tool := range tools {
		s.bound = append(s.bound, tool.Name)
	}
	return s, nil
}
func (s *browserScript) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	s.messages = messages
	s.stage++
	switch s.stage {
	case 1:
		return &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "backend1", Type: "function", Function: schema.FunctionCall{Name: "business_read", Arguments: "{}"}}}}, nil
	case 2:
		return &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "page1", Type: "function", Function: schema.FunctionCall{Name: "webmcp_baseline_page_context", Arguments: "{}"}}}}, nil
	case 3:
		return schema.AssistantMessage("You are on the accounts page.", nil), nil
	}
	return nil, fmt.Errorf("unexpected model turn")
}
func (s *browserScript) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("streaming not used")
}

type readCounter struct{ count int }

func (t *readCounter) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "business_read", Desc: "Read a business record", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}, nil
}
func (t *readCounter) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	t.count++
	return `{"ok":true}`, nil
}

func TestBrowserToolsResumeNativeEinoWithoutRepeatingBusinessCalls(t *testing.T) {
	ctx := context.Background()
	chat := &browserScript{}
	backend := &readCounter{}
	store := &memoryBrowserCheckpoint{values: map[string][]byte{}}
	definitions := []BrowserTool{{Name: "baseline_page_context", Description: "Current page", Parameters: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)}}
	answer, pending, err := browserIteration(ctx, chat, []tool.BaseTool{backend}, definitions, store, []adk.Message{schema.UserMessage("Read the record and tell me where I am")}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil || pending.ID != "page1" || pending.Name != "baseline_page_context" || answer != "" {
		t.Fatalf("expected browser interruption: %q %+v", answer, pending)
	}
	if backend.count != 1 || chat.stage != 2 {
		t.Fatalf("unexpected pre-interrupt execution: %d %d", backend.count, chat.stage)
	}
	answer, pending, err = browserIteration(ctx, chat, []tool.BaseTool{backend}, definitions, store, nil, &BrowserCall{ID: "page1", Name: "baseline_page_context", Arguments: `{"route":"/accounts"}`}, "")
	if err != nil {
		t.Fatal(err)
	}
	if pending != nil || answer != "You are on the accounts page." || backend.count != 1 {
		t.Fatalf("resume repeated action or lost answer: %q %+v %d", answer, pending, backend.count)
	}
	found := false
	for _, message := range chat.messages {
		if message.Role == schema.Tool && message.ToolCallID == "page1" && message.Content == `{"route":"/accounts"}` {
			found = true
		}
	}
	if !found {
		t.Fatal("browser result did not reach the resumed model")
	}
}

func TestBrowserToolSchemasRejectDuplicateUnsafeAndRemoteReferences(t *testing.T) {
	good := BrowserTool{Name: "baseline_page_context", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}
	if err := validateBrowserTools([]BrowserTool{good}); err != nil {
		t.Fatal(err)
	}
	if validateBrowserTools([]BrowserTool{good, good}) == nil {
		t.Fatal("duplicate accepted")
	}
	invalid := good
	invalid.Name = "../unsafe"
	if validateBrowserTools([]BrowserTool{invalid}) == nil {
		t.Fatal("invalid name accepted")
	}
	invalid = good
	invalid.Parameters = json.RawMessage(`{"type":"object","$ref":"https://example.com/remote-schema"}`)
	if validateBrowserTools([]BrowserTool{invalid}) == nil {
		t.Fatal("remote reference accepted")
	}
}

func TestBrowserCheckpointEncryptsAndBindsRuntimeState(t *testing.T) {
	core := governance.New(nil, nil, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	store := browserCheckpoint{core: core, tenant: "a", account: 1, run: 2}
	plain := []byte(`{"tools":[],"private":"page context"}`)
	sealed, err := store.seal("manifest", plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains([]byte(sealed), []byte("page context")) {
		t.Fatal("checkpoint leaked plaintext")
	}
	got, err := store.unseal("manifest", sealed)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("checkpoint roundtrip failed")
	}
	other := store
	other.run = 3
	if _, err = other.unseal("manifest", sealed); err == nil {
		t.Fatal("checkpoint accepted for another run")
	}
	other = store
	other.tenant = "b"
	if _, err = other.unseal("manifest", sealed); err == nil {
		t.Fatal("checkpoint accepted for another tenant")
	}
}

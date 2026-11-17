package modelclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"horizon/foundation/system/governance"

	"github.com/cloudwego/eino/schema"
)

func TestDeepSeekChatCompletions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string
			Thinking struct{ Type string }
			Messages []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" ||
			request.Model != "deepseek-flash" || request.Thinking.Type != "disabled" || len(request.Messages) != 1 ||
			request.Messages[0].Role != "user" || request.Messages[0].Content != "hello" {
			t.Error("unexpected chat completions request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	ctx := context.Background()
	chat, err := NewChat(ctx, governance.AIModel{
		Enabled: true, Profile: "deepseek", DisplayName: "DeepSeek-V4.1-Flash",
		Name: "deepseek-flash", Protocol: governance.ChatCompletions,
		BaseURL: upstream.URL + "/v1", Token: "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := chat.Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
	if err != nil || reply == nil || reply.Content != "hello" {
		t.Fatal("missing completion", err)
	}
}

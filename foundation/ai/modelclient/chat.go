package modelclient

import (
	"context"
	"net/http"
	"time"

	"horizon/foundation/system/governance"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

var client = &http.Client{
	Timeout: 100 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func NewChat(ctx context.Context, cfg governance.AIModel) (*openai.ChatModel, error) {
	if !cfg.Enabled || !cfg.Configured() || cfg.Protocol != governance.ChatCompletions {
		return nil, governance.Problem("unavailable")
	}
	config := &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL, APIKey: cfg.Token, Model: cfg.Name, HTTPClient: client,
	}
	if cfg.Profile == "deepseek" {
		// Tool loops do not require provider-specific reasoning history.
		config.ExtraFields = map[string]any{"thinking": map[string]string{"type": "disabled"}}
	}
	return openai.NewChatModel(ctx, config)
}

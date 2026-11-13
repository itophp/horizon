package harness

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/cloudwego/eino/schema"
	"horizon/foundation/kernel/capability/contracts"
)

// GenerateRuntimeAI uses the configured model with no tools or chat history.
func (s *Service) GenerateRuntimeAI(ctx context.Context, prompt, data string) (string, error) {
	model, err := s.WorkflowModel(ctx)
	if err != nil {
		return "", err
	}
	reply, err := model.Generate(ctx, []*schema.Message{schema.SystemMessage(prompt), schema.UserMessage(data)})
	if err != nil {
		var providerError *openai.APIError
		if errors.As(err, &providerError) && providerError.HTTPStatusCode == 429 {
			switch fmt.Sprint(providerError.Code) {
			case "1308", "1310":
				return "", contracts.Problem("ai_quota_exhausted")
			}
		}
		return "", contracts.Problem("unavailable")
	}
	if reply == nil || len(reply.ToolCalls) > 0 {
		return "", contracts.Problem("unavailable")
	}
	return reply.Content, nil
}

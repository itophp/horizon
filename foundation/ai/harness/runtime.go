package harness

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/cloudwego/eino/schema"
	"horizon/foundation/ai/attachment"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/governance"
)

// GenerateRuntimeAI uses the configured model with no tools or chat history.
func (s *Service) GenerateRuntimeAI(ctx context.Context, prompt, data string) (string, error) {
	return s.generateRuntimeAI(ctx, governance.ModelRuntimeDesign, prompt, schema.UserMessage(data))
}

func (s *Service) GenerateRuntimeAIWithImage(ctx context.Context, prompt, data, mime, image string) (string, error) {
	return s.generateRuntimeAI(ctx, governance.ModelRuntimeDesign, prompt, attachmentMessage(data, "", &attachment.Image{MIME: mime, Content: image}))
}

func attachmentMessage(question, document string, image *attachment.Image) *schema.Message {
	if image == nil {
		return schema.UserMessage(question + "\n\n<attached_document>\n" + document + "\n</attached_document>")
	}
	return &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: question},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{MIMEType: image.MIME, Base64Data: &image.Content},
			}},
		},
	}
}

func hasImage(message *schema.Message) bool {
	return message != nil && slices.ContainsFunc(message.UserInputMultiContent, func(part schema.MessageInputPart) bool {
		return part.Type == schema.ChatMessagePartTypeImageURL
	})
}

func (s *Service) GenerateProjectImportAI(ctx context.Context, prompt, data string) (string, error) {
	return s.generateRuntimeAI(ctx, governance.ModelProjectImport, prompt, schema.UserMessage(data))
}

func (s *Service) generateRuntimeAI(ctx context.Context, use governance.AIModelUse, prompt string, message *schema.Message) (string, error) {
	model, err := s.modelFor(ctx, use, hasImage(message))
	if err != nil {
		return "", err
	}
	reply, err := model.Generate(ctx, []*schema.Message{schema.SystemMessage(prompt), message})
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

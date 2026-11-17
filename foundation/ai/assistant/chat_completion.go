package assistant

import (
	"context"
	"encoding/json"
	"strings"

	"horizon/foundation/ai/modelclient"
	genassistant "horizon/foundation/gen/assistant"
	"horizon/foundation/system/governance"

	"github.com/cloudwego/eino/schema"
	einoschema "github.com/eino-contrib/jsonschema"
)

func chatCompletion(ctx context.Context, cfg governance.AIModel, payload *genassistant.ChatPayload) (*genassistant.AssistantReply, error) {
	instructions := defaultInstructions
	messages := []*schema.Message{}
	for _, message := range payload.Messages {
		if message == nil {
			continue
		}
		switch message.Role {
		case "system":
			if text := strings.TrimSpace(message.Content); text != "" {
				instructions = text
			}
		case "tool":
			if message.ToolCallID != nil && *message.ToolCallID != "" {
				messages = append(messages, schema.ToolMessage(message.Content, *message.ToolCallID))
			}
		case "assistant":
			var calls []schema.ToolCall
			for _, call := range message.ToolCalls {
				if call != nil {
					calls = append(calls, schema.ToolCall{ID: call.ID, Type: "function", Function: schema.FunctionCall{Name: call.Name, Arguments: call.Arguments}})
				}
			}
			if message.Content != "" || len(calls) > 0 {
				messages = append(messages, schema.AssistantMessage(message.Content, calls))
			}
		default:
			messages = append(messages, schema.UserMessage(message.Content))
		}
	}
	if len(messages) == 0 {
		return nil, governance.Problem("invalid")
	}
	tools := []*schema.ToolInfo{}
	for _, tool := range payload.Tools {
		if tool == nil || tool.Name == "" {
			continue
		}
		raw, err := json.Marshal(tool.Parameters)
		var parameters einoschema.Schema
		if err != nil || json.Unmarshal(raw, &parameters) != nil {
			return nil, governance.Problem("invalid")
		}
		tools = append(tools, &schema.ToolInfo{Name: tool.Name, Desc: tool.Description, ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters)})
	}
	chat, err := modelclient.NewChat(ctx, cfg)
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	bound, err := chat.WithTools(tools)
	if err != nil {
		return nil, governance.Problem("invalid")
	}
	reply, err := bound.Generate(ctx, append([]*schema.Message{schema.SystemMessage(instructions)}, messages...))
	if err != nil || reply == nil {
		return nil, governance.Problem("unavailable")
	}
	out := &genassistant.AssistantReply{Content: strings.TrimSpace(reply.Content), Model: cfg.Name, ToolCalls: []*genassistant.AssistantToolCall{}}
	for _, call := range reply.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, &genassistant.AssistantToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return out, nil
}

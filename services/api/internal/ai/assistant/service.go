package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	genassistant "baseline/services/api/gen/assistant"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
)

// Service resolves the deployment model configuration from governance settings and
// forwards a single assistant turn to the OpenAI Response compatible endpoint.
// The credential never leaves the server and the browser cannot override it.
type Service struct {
	*governance.Core
	HTTP *http.Client
}

func New(c *governance.Core) *Service {
	return &Service{Core: c, HTTP: &http.Client{Timeout: 100 * time.Second}}
}

const (
	keyEnabled = "assistant.enabled"
	keyBaseURL = "assistant.baseUrl"
	keyModel   = "assistant.model"
	keyToken   = "assistant.token"
)

// Instruction keeps the model inside the page-tool contract: it may request tools,
// but the page executes them under its own permission checks.
const defaultInstructions = "You are the embedded assistant of this administration workspace. " +
	"Answer in the user's language. Use the provided page tools when the answer depends on the current page, " +
	"route, counts or navigation; never invent business records, identifiers or credentials. " +
	"Available tools only read page state or navigate within already authorised pages; " +
	"explain what you cannot do instead of guessing."

func apiError(err *error) {
	if *err != nil {
		*err = &genassistant.IAMProblem{Code: governance.Code(*err)}
	}
}

type config struct {
	enabled    bool
	configured bool
	model      string
	baseURL    string
	token      string
}

// setting decodes one governance value. Secrets are unsealed by Core.Setting and
// still arrive JSON encoded, exactly like non-secret values.
func (s *Service) setting(ctx context.Context, p *identity.Principal, key string) (string, error) {
	raw, err := s.Core.Setting(ctx, p, key)
	if err != nil {
		return "", err
	}
	return governance.String(raw)
}

func (s *Service) settingBool(ctx context.Context, p *identity.Principal, key string) (bool, error) {
	raw, err := s.Core.Setting(ctx, p, key)
	if err != nil {
		return false, err
	}
	return governance.Bool(raw)
}

func (s *Service) resolve(ctx context.Context, p *identity.Principal) (config, error) {
	out := config{}
	var err error
	if out.enabled, err = s.settingBool(ctx, p, keyEnabled); err != nil {
		return out, err
	}
	if out.model, err = s.setting(ctx, p, keyModel); err != nil {
		return out, err
	}
	if out.baseURL, err = s.setting(ctx, p, keyBaseURL); err != nil {
		return out, err
	}
	if out.token, err = s.setting(ctx, p, keyToken); err != nil {
		return out, err
	}
	out.configured = out.token != "" && out.baseURL != "" && out.model != ""
	return out, nil
}

func (s *Service) authorize(ctx context.Context) (*identity.Principal, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, governance.Problem("unauthenticated")
	}
	if !p.Can("assistant:use") {
		return nil, governance.Problem("forbidden")
	}
	return p, nil
}

func (s *Service) Status(ctx context.Context, _ *genassistant.StatusPayload) (res *genassistant.AssistantStatus, err error) {
	defer apiError(&err)
	p, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolve(ctx, p)
	if err != nil {
		return nil, err
	}
	agentEnabled, err := s.settingBool(ctx, p, "agent.enabled")
	if err != nil {
		return nil, err
	}
	return &genassistant.AssistantStatus{Enabled: cfg.enabled, Configured: cfg.configured, AgentEnabled: agentEnabled, Model: cfg.model}, nil
}

// Response protocol payloads. Only the fields this gateway actually needs are
// modelled; unknown fields from the provider are ignored.
type responseTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`
}

type responseRequest struct {
	Model        string           `json:"model"`
	Input        []map[string]any `json:"input"`
	Instructions string           `json:"instructions,omitempty"`
	Tools        []responseTool   `json:"tools,omitempty"`
	Stream       bool             `json:"stream"`
}

type responseContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseOutputItem struct {
	Type      string                `json:"type"`
	Content   []responseContentPart `json:"content"`
	CallID    string                `json:"call_id"`
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Arguments string                `json:"arguments"`
}

type responseEnvelope struct {
	Model      string               `json:"model"`
	Output     []responseOutputItem `json:"output"`
	OutputText string               `json:"output_text"`
	Error      *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (s *Service) input(messages []*genassistant.AssistantMessage, instructions string) ([]map[string]any, string) {
	out := []map[string]any{}
	for _, message := range messages {
		if message == nil {
			continue
		}
		switch message.Role {
		case "system":
			if text := strings.TrimSpace(message.Content); text != "" {
				instructions = text
			}
		case "tool":
			if message.ToolCallID == nil || *message.ToolCallID == "" {
				continue
			}
			out = append(out, map[string]any{"type": "function_call_output", "call_id": *message.ToolCallID, "output": message.Content})
		case "assistant":
			for _, call := range message.ToolCalls {
				if call == nil {
					continue
				}
				out = append(out, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.Arguments})
			}
			if text := strings.TrimSpace(message.Content); text != "" {
				out = append(out, map[string]any{"role": "assistant", "content": text})
			}
		default:
			out = append(out, map[string]any{"role": "user", "content": message.Content})
		}
	}
	return out, instructions
}

func (s *Service) reply(body responseEnvelope) *genassistant.AssistantReply {
	text := strings.TrimSpace(body.OutputText)
	parts := []string{}
	calls := []*genassistant.AssistantToolCall{}
	for _, item := range body.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					parts = append(parts, part.Text)
				}
			}
		case "function_call":
			id := item.CallID
			if id == "" {
				id = item.ID
			}
			calls = append(calls, &genassistant.AssistantToolCall{ID: id, Name: item.Name, Arguments: item.Arguments})
		}
	}
	if text == "" {
		text = strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return &genassistant.AssistantReply{Content: text, ToolCalls: calls, Model: body.Model}
}

func (s *Service) Chat(ctx context.Context, payload *genassistant.ChatPayload) (res *genassistant.AssistantReply, err error) {
	defer apiError(&err)
	p, err := s.authorize(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolve(ctx, p)
	if err != nil {
		return nil, err
	}
	if !cfg.enabled {
		return nil, governance.Problem("unavailable")
	}
	if !cfg.configured {
		return nil, governance.Problem("unavailable")
	}
	input, instructions := s.input(payload.Messages, defaultInstructions)
	if len(input) == 0 {
		return nil, governance.Problem("invalid")
	}
	request := responseRequest{Model: cfg.model, Input: input, Instructions: instructions, Stream: false}
	for _, tool := range payload.Tools {
		if tool == nil || tool.Name == "" {
			continue
		}
		request.Tools = append(request.Tools, responseTool{Type: "function", Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters})
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, governance.Problem("invalid")
	}
	return s.send(ctx, cfg, raw)
}

// send performs one Response protocol call. It is separate from Chat so the
// translation can be verified without a database or session.
func (s *Service) send(ctx context.Context, cfg config, raw []byte) (*genassistant.AssistantReply, error) {
	endpoint := strings.TrimSuffix(cfg.baseURL, "/") + "/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 100 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, governance.Problem("unavailable")
		}
		return nil, governance.Problem("unavailable")
	}
	defer response.Body.Close()
	// Upstream error bodies are never forwarded: they can echo the prompt or credential.
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, governance.Problem("unavailable")
	}
	var envelope responseEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, governance.Problem("unavailable")
	}
	if envelope.Error != nil {
		return nil, governance.Problem("unavailable")
	}
	reply := s.reply(envelope)
	if reply.Model == "" {
		reply.Model = cfg.model
	}
	return reply, nil
}

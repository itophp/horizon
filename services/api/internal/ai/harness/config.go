package harness

import (
	"context"
	"strings"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
)

const (
	keyEnabled = "agent.enabled"
	keyBaseURL = "agent.baseUrl"
	// The agent speaks the OpenAI Chat Completion protocol, so it cannot reuse the
	// Response endpoint used by the lightweight in-page assistant.
	keyModel = "assistant.model"
	keyToken = "assistant.token"
)

// DefaultBaseURL is the GLM Coding Plan Chat Completion endpoint.
const DefaultBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"

type config struct {
	enabled bool
	baseURL string
	model   string
	token   string
}

func (c config) ready() bool {
	return c.enabled && c.baseURL != "" && c.model != "" && c.token != ""
}

func (s *Service) setting(ctx context.Context, p *identity.Principal, key string) (string, error) {
	raw, err := s.Core.Setting(ctx, p, key)
	if err != nil {
		return "", err
	}
	return governance.String(raw)
}

func (s *Service) resolve(ctx context.Context, p *identity.Principal) (config, error) {
	out := config{}
	raw, err := s.Core.Setting(ctx, p, keyEnabled)
	if err != nil {
		return out, err
	}
	if out.enabled, err = governance.Bool(raw); err != nil {
		return out, err
	}
	if out.baseURL, err = s.setting(ctx, p, keyBaseURL); err != nil {
		return out, err
	}
	if out.model, err = s.setting(ctx, p, keyModel); err != nil {
		return out, err
	}
	if out.token, err = s.setting(ctx, p, keyToken); err != nil {
		return out, err
	}
	out.baseURL = strings.TrimSpace(out.baseURL)
	return out, nil
}

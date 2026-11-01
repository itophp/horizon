package harness

import (
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"context"
	"github.com/cloudwego/eino/components/model"
)

// WorkflowModel reuses the configured server-side provider; credentials never
// become part of a workflow definition, checkpoint or tool result.
func (s *Service) WorkflowModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if !p.Can("assistant:use") {
		return nil, contracts.Problem("forbidden")
	}
	cfg, err := s.resolve(ctx, p)
	if err != nil {
		return nil, err
	}
	if !cfg.ready() {
		return nil, contracts.Problem("unavailable")
	}
	return s.chatModel(ctx, cfg)
}

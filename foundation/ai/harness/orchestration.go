package harness

import (
	"context"

	"horizon/foundation/ai/modelclient"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"

	"github.com/cloudwego/eino/components/model"
)

// WorkflowModel reuses the configured server-side provider; credentials never
// become part of a workflow definition, checkpoint or tool result.
func (s *Service) WorkflowModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return s.modelFor(ctx, governance.ModelWorkflow, false)
}

func (s *Service) modelFor(ctx context.Context, use governance.AIModelUse, image bool) (model.ToolCallingChatModel, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if !p.Can("assistant:use") {
		return nil, contracts.Problem("forbidden")
	}
	cfg, err := s.ResolveAIModel(ctx, p, use)
	if err != nil {
		return nil, err
	}
	if image && cfg.Type != governance.ModelMultimodal {
		return nil, contracts.Problem("ai_model_image_unsupported")
	}
	chat, err := modelclient.NewChat(ctx, cfg)
	if err != nil {
		return nil, contracts.Problem("unavailable")
	}
	return chat, nil
}

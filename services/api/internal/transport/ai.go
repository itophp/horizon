package transport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	genai "baseline/services/api/gen/ai"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"

	"goa.design/goa/v3/security"
)

// aiAdapter exposes AI tasks. Business execution is not reachable from here: the
// agent uses the capability gateway like every other origin.
type aiAdapter struct{ harness *harness.Service }

func newAI(service *harness.Service) *aiAdapter { return &aiAdapter{harness: service} }

func (a *aiAdapter) APIKeyAuth(ctx context.Context, key string, scheme *security.APIKeyScheme) (context.Context, error) {
	return a.harness.APIKeyAuth(ctx, key, scheme)
}

func aiProblem(err error) error {
	if err == nil {
		return nil
	}
	return &genai.IAMProblem{Code: aiErrorCode(err)}
}

func aiErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *contracts.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return governance.Code(err)
}

func (a *aiAdapter) Status(ctx context.Context, _ *genai.StatusPayload) (*genai.AIStatus, error) {
	status, err := a.harness.Status(ctx)
	if err != nil {
		return nil, aiProblem(err)
	}
	return &genai.AIStatus{
		Enabled: status.Enabled, Configured: status.Configured, Model: status.Model,
		RegistryVersion: registry.Version, CapabilityCount: status.CapabilityCount,
		TasksTotal: status.Evaluation.Total, TasksCompleted: status.Evaluation.Completed,
		TasksFailed: status.Evaluation.Failed, CompletionRate: status.Evaluation.Rate(),
		AverageDurationMs: status.Evaluation.AverageDurationMs,
		PromptTokens:      status.Evaluation.PromptTokens, CompletionTokens: status.Evaluation.CompletionTokens,
	}, nil
}

func (a *aiAdapter) Run(ctx context.Context, payload *genai.RunPayload) (*genai.AIRun, error) {
	input := harness.RunInput{Question: payload.Question, Origin: contracts.OriginHumanUI}
	if payload.SessionID != nil {
		input.SessionID = *payload.SessionID
	}
	if payload.Page != nil {
		if payload.Page.Route != nil {
			input.Page.Route = *payload.Page.Route
		}
		if payload.Page.Entity != nil {
			input.Page.Entity = *payload.Page.Entity
		}
		if payload.Page.EntityID != nil {
			input.Page.EntityID = *payload.Page.EntityID
		}
	}
	run, err := a.harness.Run(ctx, input)
	if err != nil {
		// A failed task carries no run row when it stops before the store, so the
		// cause is otherwise invisible to the operator.
		slog.Error("ai run failed", "error", err)
		return nil, aiProblem(err)
	}
	return aiRunProjection(run), nil
}

func (a *aiAdapter) Conversations(ctx context.Context, payload *genai.ConversationsPayload) ([]*genai.AIConversation, error) {
	list, err := a.harness.Conversations(ctx, payload.Limit)
	if err != nil {
		return nil, aiProblem(err)
	}
	out := make([]*genai.AIConversation, 0, len(list))
	for i := range list {
		out = append(out, aiConversationProjection(&list[i]))
	}
	return out, nil
}

func (a *aiAdapter) UpdateConversation(ctx context.Context, payload *genai.UpdateConversationPayload) (*genai.AIConversation, error) {
	updated, err := a.harness.UpdateConversation(ctx, harness.ConversationUpdate{
		SessionID: payload.SessionID, Title: payload.Title, Archived: payload.Archived,
	})
	if err != nil {
		return nil, aiProblem(err)
	}
	return aiConversationProjection(updated), nil
}

func aiConversationProjection(item *harness.Conversation) *genai.AIConversation {
	return &genai.AIConversation{
		SessionID: item.SessionID, Title: item.Title,
		UpdatedAt:    item.UpdatedAt.UTC().Format(time.RFC3339),
		MessageCount: item.MessageCount, Archived: item.Archived,
	}
}

func (a *aiAdapter) Runs(ctx context.Context, payload *genai.RunsPayload) ([]*genai.AIRun, error) {
	var runs []harness.Run
	var err error
	if payload.SessionID != nil && *payload.SessionID != "" {
		runs, err = a.harness.SessionRuns(ctx, *payload.SessionID, payload.Limit)
	} else {
		runs, err = a.harness.Runs(ctx, payload.Limit)
	}
	if err != nil {
		return nil, aiProblem(err)
	}
	out := make([]*genai.AIRun, 0, len(runs))
	for i := range runs {
		out = append(out, aiRunProjection(&runs[i]))
	}
	return out, nil
}

func (a *aiAdapter) RunDetail(ctx context.Context, payload *genai.RunDetailPayload) (*genai.AIRun, error) {
	run, err := a.harness.RunDetail(ctx, payload.ID)
	if err != nil {
		return nil, aiProblem(err)
	}
	return aiRunProjection(run), nil
}

func aiRunProjection(run *harness.Run) *genai.AIRun {
	invocations := make([]*genai.CapabilityInvocation, 0, len(run.Invocations))
	for i := range run.Invocations {
		invocations = append(invocations, aiInvocationProjection(&run.Invocations[i]))
	}
	return &genai.AIRun{
		ID: run.ID, Question: run.Question, Answer: run.Answer, Status: run.Status,
		Agent: run.Agent, Model: run.Model, InstructionVersion: run.InstructionVersion,
		Origin: run.Origin, ErrorCode: run.ErrorCode, ToolCalls: run.ToolCalls,
		PromptTokens: run.PromptTokens, CompletionTokens: run.CompletionTokens,
		DurationMs: int(run.Duration.Milliseconds()), TraceID: run.TraceID,
		CorrelationID: run.CorrelationID, CausationID: run.CausationID,
		TriggerDepth: run.TriggerDepth, TriggerPolicy: run.TriggerPolicy, EventID: run.EventID,
		CreatedAt: stamp(run.CreatedAt), FinishedAt: stamp(run.FinishedAt), Invocations: invocations,
	}
}

func aiInvocationProjection(item *audit.Invocation) *genai.CapabilityInvocation {
	return &genai.CapabilityInvocation{
		InvocationID: item.InvocationID, Capability: item.Capability, Version: item.Version,
		Origin: item.Origin, Status: item.Status, Input: decodeJSON(item.Input), InputDigest: item.Digest,
		Result: decodeJSON(item.Result), ErrorCode: item.ErrorCode, ApprovalID: item.ApprovalID,
		CorrelationID: item.CorrelationID, CausationID: item.CausationID, TriggerDepth: item.Depth,
		DurationMs: int(item.Duration.Milliseconds()), TraceID: item.TraceID,
		CreatedAt: stamp(item.CreatedAt), FinishedAt: stamp(item.FinishedAt),
	}
}

func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return decoded
}

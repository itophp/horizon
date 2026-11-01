package transport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	gen "baseline/services/api/gen/capability"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/approval"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"

	"goa.design/goa/v3/security"
)

// capabilityAdapter is the HTTP face of the invocation gateway. It projects
// contracts and audit rows; it never executes business logic itself.
type capabilityAdapter struct {
	gateway  *gateway.Gateway
	registry *registry.Registry
}

func newCapability(g *gateway.Gateway, r *registry.Registry) *capabilityAdapter {
	return &capabilityAdapter{gateway: g, registry: r}
}

func (a *capabilityAdapter) APIKeyAuth(ctx context.Context, _ string, _ *security.APIKeyScheme) (context.Context, error) {
	_, err := identity.Current(ctx)
	return ctx, err
}

func capabilityProblem(err error) error {
	if err == nil {
		return nil
	}
	return &gen.IAMProblem{Code: capabilityErrorCode(err)}
}

func capabilityErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *contracts.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return governance.Code(err)
}

func stamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (a *capabilityAdapter) Definitions(ctx context.Context, _ *gen.DefinitionsPayload) ([]*gen.CapabilityContract, error) {
	definitions, err := a.gateway.Definitions(ctx)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	out := make([]*gen.CapabilityContract, 0, len(definitions))
	for _, definition := range definitions {
		layer, kind, effect := string(definition.Layer), string(definition.Kind), string(definition.Effect)
		retry := definition.RetrySafe
		out = append(out, &gen.CapabilityContract{
			Layer: &layer, Kind: &kind, Effect: &effect, Reads: definition.Reads, Writes: definition.Writes, RetrySafe: &retry,
			Name: definition.Name, Version: definition.Version, Title: definition.Title,
			Description: definition.Description, Domain: definition.Domain, Entity: definition.Entity,
			Permission: definition.Permission, Risk: string(definition.Risk), Approval: string(definition.Approval),
			Idempotent: definition.Idempotency.Required, ReadOnly: definition.ReadOnly,
			Executor: definition.Executor, Tags: definition.Tags, SuccessEvents: definition.SuccessEvents,
			InputSchema: definition.Input.JSON(), OutputSchema: schemaOrEmpty(definition.Output),
		})
	}
	return out, nil
}

func schemaOrEmpty(schema *contracts.Schema) any {
	if schema == nil {
		return map[string]any{}
	}
	return schema.JSON()
}

func (a *capabilityAdapter) Invoke(ctx context.Context, payload *gen.InvokePayload) (*gen.CapabilityResult, error) {
	input, ok := payload.Input.(map[string]any)
	if !ok {
		return nil, capabilityProblem(contracts.Problem("invalid"))
	}
	correlation, causation := "", ""
	if payload.CorrelationID != nil {
		correlation = *payload.CorrelationID
	}
	if payload.CausationID != nil {
		causation = *payload.CausationID
	}
	version := ""
	if payload.Version != nil {
		version = *payload.Version
	}
	key := ""
	if payload.IdempotencyKey != nil {
		key = *payload.IdempotencyKey
	}
	result, err := a.gateway.Invoke(ctx, gateway.Request{
		Name: payload.Name, Version: version, Input: input, IdempotencyKey: key,
		Origin: contracts.OriginHumanUI, CorrelationID: correlation, CausationID: causation,
	})
	if err != nil {
		return nil, capabilityProblem(err)
	}
	return resultProjection(result), nil
}

func (a *capabilityAdapter) Invocations(ctx context.Context, payload *gen.InvocationsPayload) ([]*gen.CapabilityInvocation, error) {
	items, err := a.gateway.Invocations(ctx, payload.Limit)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	out := make([]*gen.CapabilityInvocation, 0, len(items))
	for i := range items {
		out = append(out, invocationProjection(&items[i]))
	}
	return out, nil
}

func (a *capabilityAdapter) Invocation(ctx context.Context, payload *gen.InvocationPayload) (*gen.CapabilityInvocation, error) {
	item, err := a.gateway.Invocation(ctx, payload.InvocationID)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	return invocationProjection(item), nil
}

func (a *capabilityAdapter) Approvals(ctx context.Context, _ *gen.ApprovalsPayload) ([]*gen.CapabilityApproval, error) {
	items, err := a.gateway.Approvals(ctx)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	owner, err := currentAccount(ctx)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	out := make([]*gen.CapabilityApproval, 0, len(items))
	for i := range items {
		out = append(out, a.approvalProjection(&items[i], owner))
	}
	return out, nil
}

func (a *capabilityAdapter) Decide(ctx context.Context, payload *gen.DecidePayload) (*gen.CapabilityResult, error) {
	note := ""
	if payload.Note != nil {
		note = *payload.Note
	}
	result, err := a.gateway.Decide(ctx, payload.ID, payload.Approve, note)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	return resultProjection(result), nil
}

func (a *capabilityAdapter) Events(ctx context.Context, payload *gen.EventsPayload) ([]*gen.CapabilityEvent, error) {
	items, err := a.gateway.Events(ctx, payload.Limit)
	if err != nil {
		return nil, capabilityProblem(err)
	}
	out := make([]*gen.CapabilityEvent, 0, len(items))
	for i := range items {
		event := &items[i]
		out = append(out, &gen.CapabilityEvent{
			EventID: event.EventID, Type: event.Type, AggregateType: event.AggregateType,
			AggregateID: event.AggregateID, Payload: event.Payload, InvocationID: event.InvocationID,
			CorrelationID: event.CorrelationID, CausationID: event.CausationID, Depth: event.Depth,
			CreatedAt: stamp(event.CreatedAt), AiStatus: event.AIStatus, AiPolicy: event.AIPolicy,
			AiRunID: event.AIRunID,
		})
	}
	return out, nil
}

func resultProjection(result *contracts.Result) *gen.CapabilityResult {
	out := &gen.CapabilityResult{
		InvocationID: result.InvocationID, Status: string(result.Status), Data: result.Data,
		EventIds: result.EventIDs, ErrorCode: result.ErrorCode, TraceID: result.TraceID,
		Replayed: result.Replayed,
	}
	if result.ApprovalID != 0 {
		id := result.ApprovalID
		out.ApprovalID = &id
	}
	return out
}

func invocationProjection(item *audit.Invocation) *gen.CapabilityInvocation {
	return &gen.CapabilityInvocation{
		InvocationID: item.InvocationID, Capability: item.Capability, Version: item.Version,
		Origin: item.Origin, Status: item.Status, Input: rawJSON(item.Input), InputDigest: item.Digest,
		Result: rawJSON(item.Result), ErrorCode: item.ErrorCode, ApprovalID: item.ApprovalID,
		CorrelationID: item.CorrelationID, CausationID: item.CausationID, TriggerDepth: item.Depth,
		DurationMs: int(item.Duration.Milliseconds()), TraceID: item.TraceID,
		CreatedAt: stamp(item.CreatedAt), FinishedAt: stamp(item.FinishedAt),
	}
}

func rawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return decoded
}

func (a *capabilityAdapter) approvalProjection(card *approval.Approval, owner int64) *gen.CapabilityApproval {
	return &gen.CapabilityApproval{
		ID: card.ID, InvocationID: card.InvocationID, Capability: card.Capability, Version: card.Version,
		Risk: card.Risk, Approval: card.Kind, Status: card.Status, Arguments: a.redact(card),
		RequestedAt: stamp(card.RequestedAt), ExpiresAt: stamp(card.ExpiresAt), DecidedAt: stamp(card.DecidedAt),
		DecisionNote: card.Note, ExecutionCode: card.ExecutionCode, Mine: card.AccountID == owner,
	}
}

// redact re-projects the reviewed payload through the capability allowlist so a
// message body or any other unbounded field never reaches the response.
func (a *capabilityAdapter) redact(card *approval.Approval) any {
	definition, ok := a.registry.Resolve(card.Capability, card.Version)
	if !ok {
		return map[string]any{}
	}
	input := contracts.Input{}
	if err := json.Unmarshal(card.Arguments, &input); err != nil {
		return map[string]any{}
	}
	return definition.Definition.Redact(input)
}

func currentAccount(ctx context.Context) (int64, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return 0, err
	}
	return p.Account.ID, nil
}

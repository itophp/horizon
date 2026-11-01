package gateway

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"context"
)

type invocationKey struct{}

// InvocationContext is injected by Gateway, never decoded from business input.
type InvocationContext struct {
	Capability     string
	Version        string
	InvocationID   string
	CorrelationID  string
	Depth          int
	IdempotencyKey string
	Origin         contracts.Origin
	InTransaction  bool
}

func CurrentInvocation(ctx context.Context) (InvocationContext, bool) {
	meta, ok := ctx.Value(invocationKey{}).(InvocationContext)
	return meta, ok
}

func withInvocation(ctx context.Context, meta InvocationContext) context.Context {
	return context.WithValue(ctx, invocationKey{}, meta)
}

// ChildKey identifies an action slot within this invocation. Retries of the
// outer invocation retain its key, while independent calls remain independent.
func ChildKey(ctx context.Context, slot string) string {
	meta, ok := CurrentInvocation(ctx)
	if !ok {
		return ""
	}
	parent := meta.IdempotencyKey
	if parent == "" {
		parent = meta.InvocationID
	}
	return contracts.Digest(contracts.Input{"capability": meta.Capability, "version": meta.Version, "parent": parent, "slot": slot})
}

func inheritRequest(ctx context.Context, r *Request) error {
	meta, ok := CurrentInvocation(ctx)
	if !ok {
		return nil
	}
	if meta.InTransaction {
		return contracts.Problem("invalid")
	}
	// A nested caller cannot impersonate UI or an approved resumption.
	r.Origin = contracts.OriginCapability
	r.TriggerDepth = meta.Depth + 1
	r.CorrelationID = meta.CorrelationID
	r.CausationID = meta.InvocationID
	return nil
}

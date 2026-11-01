package gateway

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"context"
	"testing"
)

func TestNestedInvocationCannotResetPolicyOrDepth(t *testing.T) {
	ctx := withInvocation(context.Background(), InvocationContext{InvocationID: "parent", CorrelationID: "trace", Depth: 7, IdempotencyKey: "stable"})
	r := Request{Origin: contracts.OriginApproval, TriggerDepth: 0, CorrelationID: "forged"}
	if err := inheritRequest(ctx, &r); err != nil {
		t.Fatal(err)
	}
	if r.TriggerDepth != 8 || r.CausationID != "parent" || r.CorrelationID != "trace" || r.Origin != contracts.OriginCapability {
		t.Fatalf("untrusted child metadata survived: %+v", r)
	}
	if ChildKey(ctx, "first") != ChildKey(ctx, "first") || ChildKey(ctx, "first") == ChildKey(ctx, "second") {
		t.Fatal("unstable or colliding child key")
	}
	other := withInvocation(context.Background(), InvocationContext{Capability: "other", IdempotencyKey: "stable"})
	if ChildKey(ctx, "first") == ChildKey(other, "first") {
		t.Fatal("different parent capabilities shared a child key")
	}
	ctx = withInvocation(ctx, InvocationContext{InTransaction: true})
	if inheritRequest(ctx, &r) == nil {
		t.Fatal("nested transactional invocation accepted")
	}
}

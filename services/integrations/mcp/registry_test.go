package mcp

import (
	"context"
	"testing"

	"baseline/services/api/kernel/capability/contracts"
)

func TestRateLimitsAreIndependentAndBounded(t *testing.T) {
	l := requestLimits{}
	if !l.allow("token:one", 1, 1) || l.allow("token:one", 1, 1) {
		t.Fatal("token burst not enforced")
	}
	if !l.allow("token:two", 1, 1) {
		t.Fatal("one token exhausted another")
	}
	for i := 0; i < 4094; i++ {
		if !l.allow(string(rune(i+1000)), 1, 1) {
			t.Fatal("capacity exhausted early")
		}
	}
	if l.allow("overflow", 1, 1) {
		t.Fatal("unbounded limiter storage")
	}
}

func TestContextRequiresAuthenticatedPrincipal(t *testing.T) {
	if _, err := New(nil, nil).currentContext(context.Background(), contracts.Input{}); err == nil {
		t.Fatal("anonymous context accepted")
	}
}

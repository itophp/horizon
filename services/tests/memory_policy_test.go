//go:build integration

package tests

import (
	"context"
	"testing"

	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	"baseline/services/api/system/identity"
	"baseline/services/api/system/memory"
)

func TestMemoryRetentionPolicyRequiresPermissionAndVersion(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	reader := h.account(t, "policy-reader", "memory:read")
	if _, err := h.call(t, reader, "memory.policy.list", map[string]any{}); err == nil {
		t.Fatal("policy visible without management permission")
	}
	first := h.ok(t, h.root, "memory.policy.set", map[string]any{"partition": "activity", "retentionDays": 30, "expectedRevision": 0})
	if first.Data.(map[string]any)["revision"] != int64(1) {
		t.Fatalf("unexpected initial revision: %#v", first.Data)
	}
	if _, err := h.call(t, h.root, "memory.policy.set", map[string]any{"partition": "activity", "retentionDays": 7, "expectedRevision": 0}); err == nil {
		t.Fatal("stale policy write succeeded")
	}
	second := h.ok(t, h.root, "memory.policy.set", map[string]any{"partition": "activity", "retentionDays": 0, "expectedRevision": 1})
	if second.Data.(map[string]any)["revision"] != int64(2) {
		t.Fatalf("revision did not advance: %#v", second.Data)
	}
	foreignAccount, err := identity.Bootstrap(context.Background(), h.pool, "tenant-b", "policy-foreign-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.iam.PrincipalForTask(context.Background(), "tenant-b", foreignAccount.ID, foreignAccount.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	foreign := projectActor{name: "foreign", id: foreignAccount.ID, ctx: identity.WithPrincipal(context.Background(), p)}
	listed := h.ok(t, foreign, "memory.policy.list", map[string]any{})
	if len(listed.Data.(map[string]any)["policies"].([]map[string]any)) != 0 {
		t.Fatal("policy leaked across tenants")
	}
}

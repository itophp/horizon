//go:build integration

package tests

import (
	"context"
	"testing"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/notifications"
	"baseline/services/api/internal/platform/approval"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/eventhub"
)

// TestAutomationApprovalBooking proves that an automation origin can book an
// approval card for a high risk capability, and that approving the card executes
// exactly the reviewed payload. It covers the path the seeded-row approval test
// skipped: the real insert inside the booking transaction.
func TestAutomationApprovalBooking(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()

	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{
		EncryptionKey:   []byte("01234567890123456789012345678901"),
		StorageRoot:     t.TempDir(),
		PublicWorkspace: "tenant-a",
	})
	catalog := registry.New(notifications.New(core).Entries())
	if err := catalog.Err(); err != nil {
		t.Fatalf("catalog invalid: %v", err)
	}
	dispatcher := gateway.New(gateway.Options{
		Registry: catalog, Identity: iam, Pool: pool,
		Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool),
	})
	principal, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	callCtx := identity.WithPrincipal(ctx, principal)
	input := contracts.Input{
		"targets":  []any{map[string]any{"kind": "account", "id": float64(root.ID)}},
		"title":    "automation booking",
		"markdown": "booked by an agent",
		"fileIds":  []any{},
		"email":    false,
	}
	send := func(key string) (*contracts.Result, error) {
		return dispatcher.Invoke(callCtx, gateway.Request{
			Name: "notification.send", Version: "1.0.0", Input: input,
			IdempotencyKey: key, Origin: contracts.OriginAgent,
			CorrelationID: "trace-booking", CausationID: "run:1",
		})
	}

	booked, err := send("run:1:notification.send:digest")
	if err != nil {
		t.Fatalf("agent send must book a card, got error: %v", err)
	}
	if booked.Status != contracts.StatusAwaitingApproval || booked.ApprovalID == 0 {
		t.Fatalf("agent send must await approval, got %+v", booked)
	}
	card, err := approval.NewStore(pool).Get(ctx, "tenant-a", booked.ApprovalID)
	if err != nil {
		t.Fatalf("booked card is unreadable: %v", err)
	}
	if card.Status != approval.Pending || card.Kind != string(contracts.ApprovalConfirm) {
		t.Fatalf("unexpected card: status=%s kind=%s", card.Status, card.Kind)
	}
	// The invocation must be audited even though nothing executed yet.
	if _, err = dispatcher.Invocation(callCtx, booked.InvocationID); err != nil {
		t.Fatalf("booked invocation is missing from the audit trail: %v", err)
	}
	again, err := send("run:2:notification.send:digest")
	if err != nil {
		t.Fatalf("rebooking reused the card but still failed: %v", err)
	}
	if again.ApprovalID != booked.ApprovalID {
		t.Fatalf("same payload must reuse the open card: %d vs %d", again.ApprovalID, booked.ApprovalID)
	}

	executed, err := dispatcher.Decide(callCtx, booked.ApprovalID, true, "approve")
	if err != nil {
		t.Fatalf("approving the card failed: %v", err)
	}
	if executed.Status != contracts.StatusSucceeded {
		t.Fatalf("approved send did not execute: %+v", executed)
	}
	inbox, err := dispatcher.Invoke(callCtx, gateway.Request{
		Name: "notification.inbox.read", Version: "1.0.0", Input: contracts.Input{},
		Origin: contracts.OriginAgent, CorrelationID: "trace-booking",
	})
	if err != nil {
		t.Fatal(err)
	}
	messages, ok := inbox.Data.([]*notifications.InboxMessage)
	if !ok || len(messages) == 0 {
		t.Fatalf("approved announcement is missing from the inbox: %+v", inbox.Data)
	}
}

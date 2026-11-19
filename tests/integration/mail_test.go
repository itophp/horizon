//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"horizon/applications/mail"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	"horizon/internal/repository/dbgen"
)

type mailHarness struct {
	*projectHarness
	core *governance.Core
}

func newMailHarness(t *testing.T) *mailHarness {
	t.Helper()
	h := newProjectHarness(t)
	core := governance.New(h.pool, h.iam, governance.Options{
		EncryptionKey: []byte("01234567890123456789012345678901"),
		StorageRoot:   t.TempDir(),
	})
	files.New(core)
	catalog := registry.New(mail.New(core, mail.Options{}).Entries())
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	h.gw = gateway.New(gateway.Options{
		Registry: catalog, Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool),
	})
	return &mailHarness{projectHarness: h, core: core}
}

func (h *mailHarness) mailbox(t *testing.T, actor projectActor, address string, defaultSend bool) int64 {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(h.pool)
	id, err := q.MailMailboxCreate(ctx, dbgen.MailMailboxCreateParams{
		TenantID: "tenant-a", OwnerAccountID: actor.id, Name: address,
		EmailAddress: address, DefaultSend: defaultSend,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, purpose := range []string{"smtp", "pop3"} {
		ciphertext, err := h.core.Seal("tenant-a", fmt.Sprintf("mail_mailbox:%d", id), purpose+":v1", "app-password")
		if err != nil {
			t.Fatal(err)
		}
		credential, err := q.MailCredentialCreate(ctx, dbgen.MailCredentialCreateParams{
			TenantID: "tenant-a", MailboxID: id, Purpose: purpose,
			Ciphertext: ciphertext, Fingerprint: governance.Hash(ciphertext)[:16],
		})
		if err != nil {
			t.Fatal(err)
		}
		direction, protocol, host, port, tlsMode := "send", "smtp", "smtp.example.test", 587, "starttls"
		if purpose == "pop3" {
			direction, protocol, host, port, tlsMode = "receive", "pop3", "pop.example.test", 995, "tls"
		}
		if _, err = q.MailChannelCreate(ctx, dbgen.MailChannelCreateParams{
			TenantID: "tenant-a", MailboxID: id, Direction: direction, Protocol: protocol,
			Host: host, Port: int32(port), TlsMode: tlsMode, Username: address, CredentialID: credential.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = q.MailMailboxStatusUpdate(ctx, dbgen.MailMailboxStatusUpdateParams{
		Status: "active", TenantID: "tenant-a", ID: id, OwnerAccountID: actor.id,
		Column5: []string{"unverified"},
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestMailMailboxesAndMessagesAreAccountPrivate(t *testing.T) {
	h := newMailHarness(t)
	owner := h.account(t, "mail-owner", "mail:read", "mail:send", "mail:configure")
	other := h.account(t, "mail-other", "mail:read", "mail:send", "mail:configure")
	onboarding := h.account(t, "mail-onboarding", "mail:configure")
	first := h.mailbox(t, owner, "first@example.test", true)
	second := h.mailbox(t, owner, "second@example.test", false)
	h.mailbox(t, other, "other@example.test", true)
	unverified := h.ok(t, onboarding, "mail.mailbox.onboard", map[string]any{
		"name": "Unverified", "emailAddress": "unverified@example.test", "password": "app-password",
		"smtpHost": "smtp.example.test", "smtpPort": 587, "smtpTls": "starttls",
		"pop3Host": "pop.example.test", "pop3Port": 995,
	})
	unverifiedPayload := decodePayload[struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}](t, unverified.Data)
	if unverifiedPayload.Status != "unverified" {
		t.Fatalf("disallowed endpoint did not preserve unverified mailbox: %+v", unverifiedPayload)
	}
	var failedChannels int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM mail_channels WHERE tenant_id='tenant-a' AND mailbox_id=$1 AND verification_status='failed'`,
		unverifiedPayload.ID).Scan(&failedChannels); err != nil || failedChannels != 2 {
		t.Fatalf("failed channels=%d err=%v", failedChannels, err)
	}

	ownerList := decodePayload[struct {
		Items []struct {
			ID           int64  `json:"id"`
			EmailAddress string `json:"emailAddress"`
		} `json:"items"`
	}](t, h.ok(t, owner, "mail.mailbox.list", nil).Data)
	if len(ownerList.Items) != 2 || ownerList.Items[0].ID != first || ownerList.Items[1].ID != second {
		t.Fatalf("owner mailboxes = %+v", ownerList.Items)
	}

	sendResult := h.ok(t, owner, "mail.message.send", map[string]any{
		"clientKey": "mail-integration-key", "mailboxId": first,
		"to":      []any{map[string]any{"address": "recipient@example.test"}},
		"subject": "Integration", "markdown": "**private integration body**",
	})
	if sendResult.Status != contracts.StatusSucceeded {
		t.Fatalf("send status=%s code=%s", sendResult.Status, sendResult.ErrorCode)
	}
	approved := sendResult
	payload := decodePayload[struct {
		MessageID  int64 `json:"messageId"`
		DeliveryID int64 `json:"deliveryId"`
	}](t, approved.Data)
	if payload.MessageID == 0 || payload.DeliveryID == 0 {
		t.Fatalf("missing ids: %+v", payload)
	}
	_, duplicateErr := h.callWith(t, owner, "mail.message.send", map[string]any{
		"clientKey": "mail-integration-key", "mailboxId": first,
		"to":      []any{map[string]any{"address": "recipient@example.test"}},
		"subject": "Integration", "markdown": "**private integration body**",
	}, "mail-duplicate-send", contracts.OriginHumanUI)
	rejected(t, nil, duplicateErr, "conflict")

	var message, recipient, delivery, event int
	ctx := context.Background()
	if err := h.pool.QueryRow(ctx, `SELECT
	  (SELECT count(*) FROM mail_messages WHERE tenant_id='tenant-a' AND id=$1 AND owner_account_id=$2),
	  (SELECT count(*) FROM mail_message_recipients WHERE tenant_id='tenant-a' AND message_id=$1),
	  (SELECT count(*) FROM mail_outbound_deliveries WHERE tenant_id='tenant-a' AND message_id=$1 AND status='queued'),
	  (SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND event_type='mail.message.submitted')`,
		payload.MessageID, owner.id).Scan(&message, &recipient, &delivery, &event); err != nil {
		t.Fatal(err)
	}
	if message != 1 || recipient != 1 || delivery != 1 || event != 1 {
		t.Fatalf("message=%d recipient=%d delivery=%d event=%d", message, recipient, delivery, event)
	}

	_, otherErr := h.call(t, other, "mail.message.get", map[string]any{"id": payload.MessageID})
	rejected(t, nil, otherErr, "not_found")
	var leaks int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM cap_invocations WHERE capability='mail.message.send' AND (input::text LIKE '%private integration body%' OR result::text LIKE '%private integration body%')`,
	).Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("body leaked into invocation audit: count=%d err=%v", leaks, err)
	}
	var credentialProjection []byte
	if err := h.pool.QueryRow(ctx, `SELECT input FROM cap_invocations WHERE capability='mail.mailbox.list' ORDER BY id DESC LIMIT 1`).Scan(&credentialProjection); err != nil {
		t.Fatal(err)
	}
	var invocationInput map[string]any
	if err := json.Unmarshal(credentialProjection, &invocationInput); err != nil {
		t.Fatal(err)
	}
	if _, exists := invocationInput["password"]; exists {
		t.Fatal("credential field appeared in audited input")
	}
}

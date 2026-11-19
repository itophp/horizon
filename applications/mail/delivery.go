package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/governance"
	"horizon/internal/repository/dbgen"
)

func (m *Module) listDeliveries(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	rows, err := m.Q.MailDeliveryList(ctx, dbgen.MailDeliveryListParams{TenantID: p.TenantID, SubmittedBy: p.Account.ID, Limit: 100})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.ID, "mailboxId": row.MailboxID, "messageId": row.MessageID,
			"status": row.Status, "attempts": row.Attempts, "nextAttemptAt": timeString(row.NextAttemptAt),
			"errorCode": row.ErrorCode, "sentAt": timeString(row.SentAt), "createdAt": timeString(row.CreatedAt),
			"subject": row.Subject, "from": row.FromAddress,
		})
	}
	return contracts.Output{Data: map[string]any{"items": items}, Summary: fmt.Sprintf("deliveries=%d", len(items))}, nil
}

func timeString(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func (m *Module) retryDelivery(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := DeliveryAction{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(tx).MailDeliveryRetry(ctx, dbgen.MailDeliveryRetryParams{
		TenantID: p.TenantID, ID: payload.ID, SubmittedBy: p.Account.ID, Attempts: int32(m.options.RetryMax),
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID, "status": "queued"}, Events: []contracts.Event{
		mailEvent(DeliveryRetriedEvent, "mail_outbound_delivery", payload.ID, map[string]any{"deliveryId": payload.ID, "status": "queued"}),
	}}, nil
}

func (m *Module) cancelDelivery(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := DeliveryAction{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(tx).MailDeliveryCancel(ctx, dbgen.MailDeliveryCancelParams{TenantID: p.TenantID, ID: payload.ID, SubmittedBy: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID, "status": "cancelled"}, Events: []contracts.Event{
		mailEvent(DeliveryCancelled, "mail_outbound_delivery", payload.ID, map[string]any{"deliveryId": payload.ID, "status": "cancelled"}),
	}}, nil
}

func timeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func deliveryOutcome(err error, attempts, retryMax int) (status, code, eventType string) {
	status, code, eventType = "failed", "unavailable", DeliveryFailedEvent
	if timeout(err) {
		status, code, eventType = "ambiguous", "timeout", DeliveryAmbiguous
	}
	if status == "failed" && attempts < retryMax {
		status = "queued"
	}
	return status, code, eventType
}

func (m *Module) processOutbound(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	system, err := requireSystem(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := DeliveryAction{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	row, err := q.MailOutboundGet(ctx, dbgen.MailOutboundGetParams{TenantID: system.TenantID, ID: payload.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	lease, err := governance.Token()
	if err != nil {
		return contracts.Output{}, err
	}
	started, err := q.MailOutboundStart(ctx, dbgen.MailOutboundStartParams{LeaseOwner: lease, TenantID: row.TenantID, ID: row.DeliveryID})
	if err != nil {
		return contracts.Output{}, err
	}
	if started != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	password, err := openCredential(m.Core, row.TenantID, row.MailboxID, "smtp", row.Version, row.Ciphertext)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	recipientRows, err := q.MailOutboundRecipients(ctx, dbgen.MailOutboundRecipientsParams{
		TenantID: row.TenantID, MessageID: row.MessageID, Column3: []string{"to", "cc", "bcc"},
	})
	if err != nil {
		return contracts.Output{}, err
	}
	recipients := make([]outboundRecipient, 0, len(recipientRows))
	for _, recipient := range recipientRows {
		recipients = append(recipients, outboundRecipient{
			Kind: recipient.Kind, Address: mail.Address{Address: recipient.Address},
		})
	}
	attachmentRows, err := q.MailMessageAttachmentsList(ctx, dbgen.MailMessageAttachmentsListParams{TenantID: row.TenantID, MessageID: row.MessageID})
	if err != nil {
		return contracts.Output{}, err
	}
	attachmentIDs := make([]int64, 0, len(attachmentRows))
	for _, attachment := range attachmentRows {
		attachmentIDs = append(attachmentIDs, attachment.FileID)
	}
	attachments, _, err := m.outboundAttachments(ctx, q, row.TenantID, row.OwnerAccountID, row.MessageID, attachmentIDs)
	if err != nil {
		return contracts.Output{}, err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	err = sendSMTP(sendCtx, smtpEndpoint{
		Host: row.Host, Port: int(row.Port), TLSMode: row.TlsMode, Username: row.Username, Password: password,
	}, m.options.AllowedEndpoints, row.FromAddress, recipients, row.Subject, row.BodyText, row.BodyHtml, row.ProviderUid, row.ReplyTo, attachments)
	if err == nil {
		finished, err := q.MailOutboundFinishSent(ctx, dbgen.MailOutboundFinishSentParams{TenantID: row.TenantID, ID: row.DeliveryID})
		if err != nil {
			return contracts.Output{}, err
		}
		if finished != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		return contracts.Output{Data: map[string]any{"id": row.DeliveryID, "status": "sent"}, Events: []contracts.Event{
			mailEvent(MessageDeliveredEvent, "mail_outbound_delivery", row.DeliveryID, map[string]any{"deliveryId": row.DeliveryID, "status": "sent"}),
		}}, nil
	}
	status, code, eventType := deliveryOutcome(err, int(row.Attempts), m.options.RetryMax)
	delay := time.Duration(1<<uint(row.Attempts)) * time.Minute
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	finished, updateErr := q.MailOutboundFinishRetryable(ctx, dbgen.MailOutboundFinishRetryableParams{
		Status: status, ErrorCode: code, NextAttemptAt: pgtype.Timestamptz{Time: time.Now().Add(delay), Valid: true},
		TenantID: row.TenantID, ID: row.DeliveryID,
	})
	if updateErr != nil {
		return contracts.Output{}, updateErr
	}
	if finished != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": row.DeliveryID, "status": status}, Events: []contracts.Event{
		mailEvent(eventType, "mail_outbound_delivery", row.DeliveryID, map[string]any{
			"deliveryId": row.DeliveryID, "status": status, "errorCode": code,
		}),
	}}, nil
}

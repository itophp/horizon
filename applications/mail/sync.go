package mail

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/knadh/go-pop3"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/governance"
	"horizon/internal/repository/dbgen"
)

func clip(value string, limit int) string {
	value = strings.ToValidUTF8(value, "")
	if len(value) > limit {
		return strings.ToValidUTF8(value[:limit], "")
	}
	return value
}

func (m *Module) syncInbox(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	var cleanups []func(context.Context) error
	output, err := m.syncInboxTx(ctx, tx, in, &cleanups)
	if err != nil {
		cancelContext := context.WithoutCancel(ctx)
		for _, cleanup := range cleanups {
			if cleanupErr := cleanup(cancelContext); cleanupErr != nil {
				governance.LogStorageFailure(cleanupErr)
			}
		}
		return contracts.Output{}, err
	}
	return output, nil
}

func (m *Module) syncInboxTx(ctx context.Context, tx pgx.Tx, in contracts.Input, cleanups *[]func(context.Context) error) (contracts.Output, error) {
	system, err := requireSystem(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := DeliveryAction{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	due, err := q.MailReceiveDue(ctx, dbgen.MailReceiveDueParams{TenantID: system.TenantID, ID: payload.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if len(due) != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	channel := due[0]
	lease, err := governance.Token()
	if err != nil {
		return contracts.Output{}, err
	}
	leased, err := q.MailReceiveLease(ctx, dbgen.MailReceiveLeaseParams{LeaseOwner: lease, TenantID: channel.TenantID, ID: channel.ChannelID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if leased != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	password, err := openCredential(m.Core, channel.TenantID, channel.MailboxID, "pop3", channel.Version, channel.Ciphertext)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	connection, err := pop3Connect(ctx, channel.Host, int(channel.Port), channel.Username, password, m.options.AllowedEndpoints)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok || meta.InvocationID == "" {
		_ = connection.Quit()
		return contracts.Output{}, contracts.Problem("internal")
	}
	m.sessions.Store(meta.InvocationID, connection)
	sessionFinalized := false
	defer func() {
		if !sessionFinalized {
			m.sessions.Delete(meta.InvocationID)
		}
	}()
	uids, err := connection.Uidl(0)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	syncID, err := q.MailInboxSyncCreate(ctx, dbgen.MailInboxSyncCreateParams{TenantID: channel.TenantID, MailboxID: channel.MailboxID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	events := []contracts.Event{}
	fetched, imported, duplicates := 0, 0, 0
	batch := m.options.SyncBatch
	for _, item := range uids {
		if fetched >= batch {
			break
		}
		if item.UID == "" || len(item.UID) > 255 || strings.ContainsAny(item.UID, "\r\n ") {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		fetched++
		exists, err := q.MailMessageUIDExists(ctx, dbgen.MailMessageUIDExistsParams{
			TenantID: channel.TenantID, MailboxID: channel.MailboxID, ProviderUid: item.UID,
		})
		if err != nil {
			return contracts.Output{}, err
		}
		if exists {
			duplicates++
			if err = connection.Dele(item.ID); err != nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			continue
		}
		entity, err := connection.Retr(item.ID)
		if err != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		parsed, err := ParseMIME(entity, m.options.MaxMessageBytes)
		if err != nil {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		if len(parsed.Attachments) > m.options.MaxAttachments {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		message, err := q.MailMessageCreate(ctx, dbgen.MailMessageCreateParams{
			TenantID: channel.TenantID, MailboxID: channel.MailboxID, OwnerAccountID: channel.OwnerAccountID,
			Direction: "inbound", Folder: "inbox", ProviderUid: item.UID,
			RfcMessageID: clip(parsed.RFCMessageID, 998), Subject: clip(parsed.Subject, 998),
			FromAddress: clip(parsed.From, 320), ReplyTo: clip(parsed.ReplyTo, 320),
			BodyText: parsed.Text, BodyHtml: parsed.HTML, Snippet: Snippet(parsed.Text),
			OccurredAt: pgtype.Timestamptz{Time: parsed.OccurredAt.UTC(), Valid: true},
		})
		if err != nil {
			return contracts.Output{}, problem(err)
		}
		if !message.Inserted {
			duplicates++
			if err = connection.Dele(item.ID); err != nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			continue
		}
		imported++
		for _, list := range []struct {
			kind string
			rows []mail.Address
		}{{"to", parsed.To}, {"cc", parsed.CC}} {
			for _, recipient := range list.rows {
				if err = q.MailMessageRecipientsAdd(ctx, dbgen.MailMessageRecipientsAddParams{
					TenantID: channel.TenantID, MessageID: message.ID, Kind: list.kind,
					Address: clip(recipient.Address, 320), DisplayName: clip(recipient.Name, 200),
				}); err != nil {
					return contracts.Output{}, problem(err)
				}
			}
		}
		if len(parsed.Attachments) > 0 {
			service, err := m.fileService()
			if err != nil {
				return contracts.Output{}, err
			}
			owner, err := m.Identity.PrincipalForTask(ctx, channel.TenantID, channel.OwnerAccountID, channel.OwnerDepartmentID)
			if err != nil {
				return contracts.Output{}, problem(err)
			}
			for _, attachment := range parsed.Attachments {
				if attachment.Filename == "" || attachment.MIME == "" || len(attachment.Data) > m.options.MaxAttachmentBytes {
					return contracts.Output{}, contracts.Problem("invalid")
				}
				file, cleanup, err := service.CreateBoundInTx(ctx, tx, owner, clip(attachment.Filename, 255), attachment.MIME, "mail_attachment", attachment.Data)
				if err != nil {
					return contracts.Output{}, problem(err)
				}
				*cleanups = append(*cleanups, cleanup)
				if _, err = q.MailMessageAttachmentAdd(ctx, dbgen.MailMessageAttachmentAddParams{
					TenantID: channel.TenantID, MessageID: message.ID, FileID: file.ID,
					ContentID: clip(attachment.ContentID, 255), Filename: file.Name,
					MimeType: file.Mime, Size: file.Size, Disposition: attachment.Disposition,
				}); err != nil {
					return contracts.Output{}, problem(err)
				}
			}
		}
		if err = connection.Dele(item.ID); err != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		events = append(events, mailEvent(MessageReceivedEvent, "mail_message", message.ID, map[string]any{
			"mailboxId": channel.MailboxID, "messageId": message.ID,
		}))
	}
	next := time.Now().Add(time.Minute)
	if len(uids) > batch {
		next = time.Now().Add(5 * time.Second)
	}
	if _, err = q.MailReceiveRelease(ctx, dbgen.MailReceiveReleaseParams{
		NextSyncAt: pgtype.Timestamptz{Time: next, Valid: true}, TenantID: channel.TenantID, ID: channel.ChannelID,
	}); err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.MailInboxSyncFinish(ctx, dbgen.MailInboxSyncFinishParams{
		Status: "completed", FetchedCount: int32(fetched), ImportedCount: int32(imported),
		DuplicateCount: int32(duplicates), ErrorCode: "", TenantID: channel.TenantID,
		ID: syncID, MailboxID: channel.MailboxID,
	}); err != nil {
		return contracts.Output{}, err
	}
	events = append(events, mailEvent(MailboxSyncedEvent, "mail_mailbox", channel.MailboxID, map[string]any{
		"mailboxId": channel.MailboxID, "fetched": fetched, "imported": imported, "duplicates": duplicates,
	}))
	// Keep the POP3 session open until the caller observes the committed result.
	m.sessions.Store(meta.InvocationID, connection)
	sessionFinalized = true
	return contracts.Output{
		Data: map[string]any{"mailboxId": channel.MailboxID, "fetched": fetched,
			"imported": imported, "duplicates": duplicates},
		Events: events,
	}, nil
}

// FinishSyncSession commits marked remote deletions only after the local
// transaction committed. A failed invocation calls it with commit=false.
func (m *Module) FinishSyncSession(invocationID string, commit bool) error {
	value, loaded := m.sessions.LoadAndDelete(invocationID)
	if !loaded {
		return nil
	}
	connection, ok := value.(*pop3.Conn)
	if !ok {
		return fmt.Errorf("invalid mail session")
	}
	if !commit {
		if err := connection.Rset(); err != nil {
			_ = connection.Quit()
			return err
		}
	}
	return connection.Quit()
}

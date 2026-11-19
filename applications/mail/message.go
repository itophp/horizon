package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	"horizon/internal/repository/dbgen"
)

func bindAddresses(list []Address) ([]mail.Address, error) {
	out := make([]mail.Address, 0, len(list))
	for _, item := range list {
		address, err := safeAddress(item.Address)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(item.DisplayName)
		if strings.ContainsAny(name, "\r\n") {
			return nil, contracts.Problem("invalid")
		}
		out = append(out, mail.Address{Address: address, Name: name})
	}
	return out, nil
}

func addressData(list []dbgen.MailMessageRecipientsListRow) []map[string]any {
	items := []map[string]any{}
	for _, row := range list {
		items = append(items, map[string]any{"kind": row.Kind, "address": row.Address, "displayName": row.DisplayName})
	}
	return items
}

func (m *Module) listMessages(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MessageListInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.Folder == "" {
		payload.Folder = "inbox"
	}
	if payload.Page == 0 {
		payload.Page = 1
	}
	if payload.PageSize == 0 {
		payload.PageSize = 50
	}
	rows, err := m.Q.MailMessageList(ctx, dbgen.MailMessageListParams{
		TenantID: p.TenantID, OwnerAccountID: p.Account.ID, Column3: []string{payload.Folder},
		Column4: payload.MailboxID, Limit: int32(payload.PageSize), Offset: int32((payload.Page - 1) * payload.PageSize),
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	total, err := m.Q.MailMessageCount(ctx, dbgen.MailMessageCountParams{
		TenantID: p.TenantID, OwnerAccountID: p.Account.ID, Column3: []string{payload.Folder}, Column4: payload.MailboxID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.ID, "mailboxId": row.MailboxID, "direction": row.Direction, "folder": row.Folder,
			"subject": row.Subject, "from": row.FromAddress, "snippet": row.Snippet,
			"occurredAt": governance.Time(row.OccurredAt), "read": row.ReadAt.Valid,
		})
	}
	return contracts.Output{Data: map[string]any{
		"items": items, "total": total, "page": payload.Page, "pageSize": payload.PageSize,
	}, Summary: fmt.Sprintf("messages=%d", len(items))}, nil
}

func (m *Module) messageData(ctx context.Context, q *dbgen.Queries, tenant string, owner int64, row dbgen.MailMessageGetRow) (map[string]any, error) {
	recipients, err := q.MailMessageRecipientsList(ctx, dbgen.MailMessageRecipientsListParams{TenantID: tenant, MessageID: row.ID})
	if err != nil {
		return nil, err
	}
	attachments, err := q.MailMessageAttachmentsList(ctx, dbgen.MailMessageAttachmentsListParams{TenantID: tenant, MessageID: row.ID})
	if err != nil {
		return nil, err
	}
	attachmentItems := []map[string]any{}
	for _, attachment := range attachments {
		attachmentItems = append(attachmentItems, map[string]any{
			"id": attachment.ID, "fileId": attachment.FileID, "contentId": attachment.ContentID,
			"filename": attachment.Filename, "mime": attachment.MimeType, "size": attachment.Size,
			"disposition": attachment.Disposition,
		})
	}
	return map[string]any{
		"id": row.ID, "mailboxId": row.MailboxID, "direction": row.Direction, "folder": row.Folder,
		"rfcMessageId": row.RfcMessageID, "subject": row.Subject, "from": row.FromAddress,
		"replyTo": row.ReplyTo, "bodyText": row.BodyText, "bodyHtml": row.BodyHtml,
		"snippet": row.Snippet, "occurredAt": governance.Time(row.OccurredAt),
		"read": row.ReadAt.Valid, "createdAt": governance.Time(row.CreatedAt),
		"recipients": addressData(recipients), "attachments": attachmentItems,
	}, nil
}

func (m *Module) getMessage(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MessageGetInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	row, err := m.Q.MailMessageGet(ctx, dbgen.MailMessageGetParams{TenantID: p.TenantID, ID: payload.ID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	data, err := m.messageData(ctx, m.Q, p.TenantID, p.Account.ID, row)
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{Data: data}, nil
}

func (m *Module) updateMessageFlag(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := MessageFlagInput{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if !payload.Read {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	rows, err := dbgen.New(tx).MailMessageMarkRead(ctx, dbgen.MailMessageMarkReadParams{TenantID: p.TenantID, ID: payload.ID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID, "read": true}, Events: []contracts.Event{
		mailEvent(MessageReadEvent, "mail_message", payload.ID, map[string]any{"messageId": payload.ID}),
	}}, nil
}

func (m *Module) fileService() (*files.Service, error) {
	service, ok := m.Logos.(*files.Service)
	if !ok || service == nil {
		return nil, contracts.Problem("unavailable")
	}
	return service, nil
}

func managedFile(row dbgen.MailAttachmentFileRow) dbgen.GovFileRow {
	return dbgen.GovFileRow{
		ID: row.ID, OwnerID: row.OwnerID, OrganizationID: row.OrganizationID, DepartmentID: row.DepartmentID,
		StorageProfileID: row.StorageProfileID, StorageKey: row.StorageKey, Name: row.Name, Mime: row.Mime,
		Size: row.Size, Purpose: row.Purpose, Sha256: row.Sha256, CreatedAt: row.CreatedAt,
	}
}

func (m *Module) outboundAttachments(ctx context.Context, q *dbgen.Queries, tenant string, owner int64, messageID int64, ids []int64) ([]outboundAttachment, []dbgen.MailMessageAttachmentAddParams, error) {
	if len(ids) > m.options.MaxAttachments {
		return nil, nil, contracts.Problem("invalid")
	}
	seen := map[int64]bool{}
	totalBytes := 0
	service, err := m.fileService()
	if err != nil {
		return nil, nil, err
	}
	attachments := make([]outboundAttachment, 0, len(ids))
	bindings := make([]dbgen.MailMessageAttachmentAddParams, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, nil, contracts.Problem("invalid")
		}
		seen[id] = true
		file, err := q.MailAttachmentFile(ctx, dbgen.MailAttachmentFileParams{TenantID: tenant, ID: id, OwnerID: owner})
		if err != nil {
			return nil, nil, problem(err)
		}
		if file.Size > int64(m.options.MaxAttachmentBytes) {
			return nil, nil, contracts.Problem("invalid")
		}
		totalBytes += int(file.Size)
		if totalBytes > m.options.MaxMessageBytes {
			return nil, nil, contracts.Problem("invalid")
		}
		data, err := service.ReadBound(ctx, tenant, managedFile(file))
		if err != nil {
			return nil, nil, problem(err)
		}
		attachments = append(attachments, outboundAttachment{Filename: file.Name, MIME: file.Mime, Data: data})
		bindings = append(bindings, dbgen.MailMessageAttachmentAddParams{
			TenantID: tenant, MessageID: messageID, FileID: id, Filename: file.Name,
			MimeType: file.Mime, Size: file.Size, Disposition: "attachment",
		})
	}
	return attachments, bindings, nil
}

func (m *Module) sendMessage(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := SendMessage{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.ClientKey == "" || !headerSafe(payload.Subject, payload.ClientKey) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	to, err := bindAddresses(payload.To)
	if err != nil {
		return contracts.Output{}, err
	}
	cc, err := bindAddresses(payload.CC)
	if err != nil {
		return contracts.Output{}, err
	}
	bcc, err := bindAddresses(payload.BCC)
	if err != nil {
		return contracts.Output{}, err
	}
	if len(to)+len(cc)+len(bcc) > m.options.SendMaxRecipients {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	textBody, htmlBody, err := RenderMarkdown(payload.Markdown)
	if err != nil {
		return contracts.Output{}, err
	}
	if len(textBody)+len(htmlBody) > m.options.MaxMessageBytes {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if payload.ReplyTo != "" {
		if _, err = safeAddress(payload.ReplyTo); err != nil {
			return contracts.Output{}, err
		}
	}
	q := dbgen.New(tx)
	mailbox, err := q.MailChannelSendGet(ctx, dbgen.MailChannelSendGetParams{TenantID: p.TenantID, ID: payload.MailboxID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	fingerprint, err := json.Marshal(struct {
		To, CC, BCC []mail.Address
		Subject     string
		Markdown    string
		Files       []int64
	}{to, cc, bcc, payload.Subject, payload.Markdown, payload.FileIDs})
	if err != nil {
		return contracts.Output{}, err
	}
	uid := governance.Hash(payload.ClientKey + ":" + string(fingerprint))
	message, err := q.MailMessageCreate(ctx, dbgen.MailMessageCreateParams{
		TenantID: p.TenantID, MailboxID: mailbox.MailboxID, OwnerAccountID: p.Account.ID,
		Direction: "outbound", Folder: "sent", ProviderUid: uid, RfcMessageID: uid,
		Subject: payload.Subject, FromAddress: mailbox.EmailAddress, ReplyTo: payload.ReplyTo,
		BodyText: textBody, BodyHtml: htmlBody, Snippet: Snippet(textBody),
		OccurredAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if !message.Inserted {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	for _, list := range []struct {
		kind string
		rows []mail.Address
	}{{"to", to}, {"cc", cc}, {"bcc", bcc}} {
		for _, recipient := range list.rows {
			if err = q.MailMessageRecipientsAdd(ctx, dbgen.MailMessageRecipientsAddParams{
				TenantID: p.TenantID, MessageID: message.ID, Kind: list.kind,
				Address: recipient.Address, DisplayName: recipient.Name,
			}); err != nil {
				return contracts.Output{}, problem(err)
			}
		}
	}
	_, bindings, err := m.outboundAttachments(ctx, q, p.TenantID, p.Account.ID, message.ID, payload.FileIDs)
	if err != nil {
		return contracts.Output{}, err
	}
	attachmentBytes := 0
	for _, binding := range bindings {
		attachmentBytes += int(binding.Size)
	}
	if len(textBody)+len(htmlBody)+attachmentBytes > m.options.MaxMessageBytes {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	for _, binding := range bindings {
		if _, err = q.MailMessageAttachmentAdd(ctx, binding); err != nil {
			return contracts.Output{}, problem(err)
		}
	}
	delivery, err := q.MailDeliveryCreate(ctx, dbgen.MailDeliveryCreateParams{
		TenantID: p.TenantID, MailboxID: mailbox.MailboxID, MessageID: message.ID, SubmittedBy: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{
		Data: map[string]any{"messageId": message.ID, "deliveryId": delivery.ID, "status": delivery.Status},
		Events: []contracts.Event{mailEvent(MessageSubmittedEvent, "mail_outbound_delivery", delivery.ID, map[string]any{
			"mailboxId": mailbox.MailboxID, "messageId": message.ID, "deliveryId": delivery.ID, "status": "queued",
		})},
	}, nil
}

func (m *Module) readAttachment(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	payload := struct {
		MessageID    int64 `json:"messageId"`
		AttachmentID int64 `json:"attachmentId"`
	}{}
	if err = contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	message, err := m.Q.MailMessageGet(ctx, dbgen.MailMessageGetParams{TenantID: p.TenantID, ID: payload.MessageID, OwnerAccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	attachments, err := m.Q.MailMessageAttachmentsList(ctx, dbgen.MailMessageAttachmentsListParams{TenantID: p.TenantID, MessageID: message.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	for _, attachment := range attachments {
		if attachment.ID != payload.AttachmentID {
			continue
		}
		file, err := m.Q.MailAttachmentFile(ctx, dbgen.MailAttachmentFileParams{TenantID: p.TenantID, ID: attachment.FileID, OwnerID: p.Account.ID})
		if err != nil {
			return contracts.Output{}, problem(err)
		}
		service, err := m.fileService()
		if err != nil {
			return contracts.Output{}, err
		}
		data, err := service.ReadBound(ctx, p.TenantID, managedFile(file))
		if err != nil {
			return contracts.Output{}, problem(err)
		}
		return contracts.Output{Data: map[string]any{"name": file.Name, "mime": file.Mime, "content": data}}, nil
	}
	return contracts.Output{}, contracts.Problem("not_found")
}

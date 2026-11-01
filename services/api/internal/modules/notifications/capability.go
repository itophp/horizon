package notifications

import (
	"context"
	"fmt"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types published by the notifications module.
const (
	SentEvent       = "notification.sent"
	ReadEvent       = "notification.read"
	RetriedEvent    = "notification.retried"
	SMTPTestedEvent = "notification.smtp_tested"
)

const version = "1.0.0"

// Entries is the capability catalog contributed by the notifications module.
// The executors only delegate: recipient resolution, validation and the
// transaction stay in the service methods.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: s.audienceDefinition(), Handler: contracts.Handler{Execute: s.readAudience}},
		{Definition: s.previewDefinition(), Handler: contracts.Handler{Execute: s.previewRecipients}},
		{Definition: s.sendDefinition(), Handler: contracts.Handler{Execute: s.sendAnnouncement}},
		{Definition: s.sentDefinition(), Handler: contracts.Handler{Execute: s.readSent}},
		{Definition: s.attachmentDefinition(), Handler: contracts.Handler{Execute: s.readAttachment}},
		{Definition: s.messageDefinition(), Handler: contracts.Handler{Execute: s.readMessage}},
		{Definition: s.inboxDefinition(), Handler: contracts.Handler{Execute: s.readInbox}},
		{Definition: s.markReadDefinition(), Handler: contracts.Handler{Execute: s.markRead}},
		{Definition: s.deliveriesDefinition(), Handler: contracts.Handler{Execute: s.readDeliveries}},
		{Definition: s.retryDefinition(), Handler: contracts.Handler{Execute: s.retryDelivery}},
		{Definition: s.testDefinition(), Handler: contracts.Handler{Execute: s.testSMTP}},
	}
}

func resourceIDSchema() *contracts.Schema { return contracts.Int(1, 1<<40) }

// targetSchema is a notification audience selector: kind is account,
// department, company or enterprise, id is the target identifier. The module
// validates the pair against the caller's visible audience.
func targetSchema() *contracts.Schema {
	schema := contracts.FreeObject()
	schema.Description = "One recipient selector object: {kind, id}. kind is account|department|company|enterprise, id is the target identifier."
	return schema
}

func targetsSchema() *contracts.Schema {
	schema := contracts.Array(targetSchema(), 1, 200)
	schema.Description = "Recipients to address. Every target must be visible to the caller."
	return schema
}

func fileNameSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"name": {Type: "string"}, "mime": {Type: "string"},
	}, "name", "mime")
}

func messageSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "integer"}, "title": {Type: "string"}, "event": {Type: "string"},
		"status": {Type: "string"}, "read": {Type: "boolean"}, "createdAt": {Type: "string"},
	}, "id", "title", "event", "read", "createdAt")
}

func (s *Service) audienceDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.audience.read", Version: version, Title: "List notification audiences",
		Description: "List the accounts, departments and organizations the caller may address, with recipient counts.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:send",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.audience", Tags: []string{"notification", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"audiences": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"kind": {Type: "string"}, "id": {Type: "integer"},
				"name": {Type: "string"}, "recipients": {Type: "integer"},
			}, "kind", "id", "name", "recipients")},
		}),
	}
}

func (s *Service) previewDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.preview.read", Version: version, Title: "Preview recipients",
		Description: "Count the recipients of a target list without sending anything. Rejects targets the caller may not address.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:send",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.preview", Tags: []string{"notification", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{"targets": targetsSchema()}, "targets"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"recipientCount": {Type: "integer"},
		}, "recipientCount"),
		Loggable: []string{"targets"},
	}
}

func (s *Service) sendDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.send", Version: version, Title: "Send an announcement",
		Description: "Send an in-app announcement, optionally duplicated by email, to the given audiences. Emits notification.sent.",
		Domain:      "notification", Entity: "announcement", Permission: "notifications:send",
		Entities: []string{"announcement", "file", "notification_delivery"},
		// A broadcast reaches many people at once: automation must obtain human
		// confirmation, exactly like the example archive capability.
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{SentEvent}, Executor: "go:notification.send",
		Tags: []string{"notification", "write", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"targets": targetsSchema(),
			"title":   contracts.Str(1, 200),
			// Markdown may reference only the files listed in fileIds.
			"markdown":  contracts.Str(1, 50000),
			"fileIds":   contracts.Array(resourceIDSchema(), 0, 20),
			"email":     contracts.Bool(),
			"clientKey": contracts.Str(0, 64),
		}, "targets", "title", "markdown"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "title": {Type: "string"},
			"recipientCount": {Type: "integer"}, "createdAt": {Type: "string"},
		}, "id", "title", "recipientCount", "createdAt"),
		// The body is not logged: it may contain personal data.
		Loggable: []string{"title", "targets", "fileIds", "email"},
	}
}

func (s *Service) sentDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.sent.read", Version: version, Title: "List sent announcements",
		Description: "List the announcements the caller has sent, newest first, with recipient counts and attachments.",
		Domain:      "notification", Entity: "announcement", Permission: "notifications:send",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.sent", Tags: []string{"notification", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"messages": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "title": {Type: "string"}, "recipientCount": {Type: "integer"},
				"createdAt": {Type: "string"},
				"attachments": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
					"id": {Type: "integer"}, "name": {Type: "string"}, "mime": {Type: "string"},
				}, "id", "name", "mime")},
			}, "id", "title", "recipientCount", "createdAt")},
		}),
	}
}

func (s *Service) attachmentDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.attachment.read", Version: version, Title: "Read an announcement attachment",
		Description: "Return one attachment of a message the caller received, as base64 with its name and mime type.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.attachment", Tags: []string{"notification", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":     resourceIDSchema(),
			"fileId": resourceIDSchema(),
		}, "id", "fileId"),
		Output:   fileNameSchema(),
		Loggable: []string{"id", "fileId"},
	}
}

func (s *Service) messageDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.message.read", Version: version, Title: "Read one inbox message",
		Description: "Read one message of the caller's own inbox, including its attachments.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.message", Tags: []string{"notification", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"id": resourceIDSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"message": messageSchema(),
			"attachments": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "mime": {Type: "string"},
			}, "id", "name", "mime")},
		}, "message"),
		Loggable: []string{"id"},
	}
}

func (s *Service) inboxDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.inbox.read", Version: version, Title: "Read my inbox",
		Description: "Read the caller's own in-app notifications, newest first, with the unread count.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.inbox", Tags: []string{"notification", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"unread":   {Type: "integer"},
			"messages": {Type: "array", Items: messageSchema()},
		}),
	}
}

func (s *Service) markReadDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.inbox.mark_read", Version: version, Title: "Mark a message as read",
		Description: "Mark one message of the caller's own inbox as read. Emits notification.read.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{ReadEvent}, Executor: "go:notification.read",
		Tags:  []string{"notification", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"id": resourceIDSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "read": {Type: "boolean"},
		}, "id", "read"),
		Loggable: []string{"id"},
	}
}

func (s *Service) deliveriesDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.delivery.read", Version: version, Title: "List email deliveries",
		Description: "List the email delivery attempts of the tenant with their status, attempt count and error code.",
		Domain:      "notification", Entity: "notification_delivery", Permission: "notifications:send",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:notification.deliveries", Tags: []string{"notification", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"deliveries": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "messageId": {Type: "integer"}, "channel": {Type: "string"},
				"status": {Type: "string"}, "attempts": {Type: "integer"},
				"errorCode": {Type: "string"}, "createdAt": {Type: "string"},
			}, "id", "messageId", "channel", "status", "attempts", "createdAt")},
		}),
	}
}

func (s *Service) retryDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.delivery.retry", Version: version, Title: "Retry an email delivery",
		Description: "Put one failed delivery back in the queue. Emits notification.retried.",
		Domain:      "notification", Entity: "notification_delivery", Permission: "notifications:retry",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{RetriedEvent}, Executor: "go:notification.retry",
		Tags:     []string{"notification", "write"},
		Input:    contracts.Object(map[string]*contracts.Schema{"id": resourceIDSchema()}, "id"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Loggable: []string{"id"},
	}
}

func (s *Service) testDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "notification.smtp.test", Version: version, Title: "Test the SMTP configuration",
		Description: "Verify the configured SMTP server with one message to the caller. send=false only validates the connection.",
		Domain:      "notification", Entity: "notification", Permission: "notifications:test",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{SMTPTestedEvent}, Executor: "go:notification.smtp_test",
		Tags:  []string{"notification", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"send": contracts.Bool()}, "send"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"status": {Type: "string"},
		}, "status"),
		Loggable: []string{"send"},
	}
}

func (s *Service) readAudience(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AudiencePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Audience(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("audiences=%d", len(out))}, nil
}

func (s *Service) previewRecipients(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := PreviewPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Preview(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("recipients=%d", out.RecipientCount)}, nil
}

func (s *Service) sendAnnouncement(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := SendPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.ClientKey == "" {
		// Keep the replay window of the module aligned with the capability
		// payload when the caller does not supply its own key.
		payload.ClientKey = contracts.Digest(in)
	}
	out, err := s.Send(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("message=%d recipients=%d", out.ID, out.RecipientCount), Events: []contracts.Event{{
		Type: SentEvent, AggregateType: "notification", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"messageId": out.ID, "recipientCount": out.RecipientCount, "email": payload.Email},
	}}}, nil
}

func (s *Service) readSent(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := SentPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Sent(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("messages=%d", len(out))}, nil
}

func (s *Service) readAttachment(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AttachmentPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Attachment(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("file=%d", payload.FileID)}, nil
}

func (s *Service) readMessage(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := MessagePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Message(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("message=%d", out.ID)}, nil
}

func (s *Service) readInbox(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := InboxPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Inbox(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	unread := 0
	for _, message := range out {
		if !message.Read {
			unread++
		}
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("messages=%d unread=%d", len(out), unread),
	}, nil
}

func (s *Service) markRead(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ReadPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.Read(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": payload.ID, "read": true},
		Summary: fmt.Sprintf("message=%d", payload.ID),
		Events: []contracts.Event{{
			Type: ReadEvent, AggregateType: "notification", AggregateID: fmt.Sprintf("%d", payload.ID),
			Payload: map[string]any{"messageId": payload.ID},
		}},
	}, nil
}

func (s *Service) readDeliveries(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DeliveriesPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Deliveries(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("deliveries=%d", len(out))}, nil
}

func (s *Service) retryDelivery(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RetryPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.Retry(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": payload.ID},
		Summary: fmt.Sprintf("delivery=%d", payload.ID),
		Events: []contracts.Event{{
			Type: RetriedEvent, AggregateType: "notification_delivery", AggregateID: fmt.Sprintf("%d", payload.ID),
			Payload: map[string]any{"deliveryId": payload.ID},
		}},
	}, nil
}

func (s *Service) testSMTP(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := TestPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Test(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("status=%s", out.Status),
		Events: []contracts.Event{{
			Type: SMTPTestedEvent, AggregateType: "notification_smtp", AggregateID: "smtp",
			Payload: map[string]any{"send": payload.Send, "status": out.Status},
		}},
	}, nil
}

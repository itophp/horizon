package mail

import (
	"horizon/foundation/kernel/capability/contracts"
)

const version = "1.0.0"

const (
	MailboxOnboardedEvent = "mail.mailbox.onboarded"
	MailboxUpdatedEvent   = "mail.mailbox.updated"
	CredentialRotated     = "mail.mailbox.credential.rotated"
	MailboxEnabledEvent   = "mail.mailbox.enabled"
	MailboxDisabledEvent  = "mail.mailbox.disabled"
	ConnectionTestedEvent = "mail.mailbox.connection.tested"
	MessageReadEvent      = "mail.message.read"
	MessageSubmittedEvent = "mail.message.submitted"
	MessageDeliveredEvent = "mail.message.delivered"
	DeliveryFailedEvent   = "mail.delivery.failed"
	DeliveryAmbiguous     = "mail.delivery.ambiguous"
	DeliveryRetriedEvent  = "mail.delivery.retried"
	DeliveryCancelled     = "mail.delivery.cancelled"
	MessageReceivedEvent  = "mail.message.received"
	MailboxSyncedEvent    = "mail.mailbox.synced"
)

func idSchema() *contracts.Schema { return contracts.Int(1, 1<<53) }

func addressSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"address": contracts.Str(3, 320), "displayName": contracts.Str(0, 200),
	}, "address")
}

func addressArray(max int) *contracts.Schema {
	return contracts.Array(addressSchema(), 1, max)
}

func mailboxSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id": idSchema(), "name": contracts.Str(1, 120), "emailAddress": contracts.Str(3, 254),
		"defaultSend": {Type: "boolean"}, "status": contracts.Enum("unverified", "active", "disabled"),
		"revision": contracts.Int(1, 1<<53), "createdAt": {Type: "string"}, "updatedAt": {Type: "string"},
		"channels": contracts.Array(contracts.Object(map[string]*contracts.Schema{
			"direction": contracts.Enum("send", "receive"), "protocol": contracts.Enum("smtp", "pop3"),
			"host": contracts.Str(1, 253), "port": contracts.Int(1, 65535),
			"tlsMode": contracts.Enum("starttls", "tls"), "username": contracts.Str(0, 200),
			"verificationStatus": contracts.Enum("unknown", "ok", "failed"), "lastErrorCode": contracts.Str(0, 100),
			"lastVerifiedAt": {Type: "string"},
			"credential": contracts.Object(map[string]*contracts.Schema{
				"version": contracts.Int(1, 1<<31), "fingerprint": contracts.Str(16, 16),
			}, "version", "fingerprint"),
		}, "direction", "protocol", "host", "port", "tlsMode", "username", "verificationStatus", "credential"), 0, 2),
	}, "id", "name", "emailAddress", "defaultSend", "status", "revision", "channels")
}

func messageSummarySchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id": idSchema(), "mailboxId": idSchema(), "direction": contracts.Enum("inbound", "outbound"),
		"folder": contracts.Enum("inbox", "sent"), "subject": contracts.Str(0, 998),
		"from": contracts.Str(0, 320), "snippet": contracts.Str(0, 500),
		"occurredAt": {Type: "string"}, "read": {Type: "boolean"},
	}, "id", "mailboxId", "direction", "folder", "subject", "from", "occurredAt", "read")
}

func messageDetailSchema() *contracts.Schema {
	schema := messageSummarySchema()
	schema.Properties["rfcMessageId"] = contracts.Str(0, 998)
	schema.Properties["replyTo"] = contracts.Str(0, 320)
	schema.Properties["bodyText"] = contracts.Str(0, 26214400)
	schema.Properties["bodyHtml"] = contracts.Str(0, 26214400)
	schema.Properties["createdAt"] = &contracts.Schema{Type: "string"}
	schema.Properties["recipients"] = contracts.Array(contracts.Object(map[string]*contracts.Schema{
		"kind": contracts.Enum("to", "cc", "bcc"), "address": contracts.Str(3, 320),
		"displayName": contracts.Str(0, 200),
	}, "kind", "address", "displayName"), 0, 600)
	schema.Properties["attachments"] = contracts.Array(contracts.Object(map[string]*contracts.Schema{
		"id": idSchema(), "fileId": idSchema(), "contentId": contracts.Str(0, 255),
		"filename": contracts.Str(1, 255), "mime": contracts.Str(3, 255), "size": contracts.Int(1, 1<<53),
		"disposition": contracts.Enum("attachment", "inline"),
	}, "id", "fileId", "filename", "mime", "size", "disposition"), 0, 20)
	return schema
}

func deliverySchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id": idSchema(), "mailboxId": idSchema(), "messageId": idSchema(),
		"status":   contracts.Enum("queued", "sending", "sent", "failed", "ambiguous", "cancelled"),
		"attempts": contracts.Int(0, 100), "nextAttemptAt": {Type: "string"},
		"errorCode": contracts.Str(0, 100), "sentAt": {Type: "string"}, "createdAt": {Type: "string"},
		"subject": contracts.Str(0, 998), "from": contracts.Str(0, 320),
	}, "id", "mailboxId", "messageId", "status", "attempts", "createdAt")
}

func mailboxListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: "mail.mailbox.list", Version: version, Title: "List mailboxes",
		Description: "List the caller's own external mailbox configurations without credentials.",
		Domain:      "mail", Entity: "mail_mailbox", Permission: "mail:configure",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:mail.mailbox.list", Tags: []string{"mail", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"items": contracts.Array(mailboxSchema(), 0, 100),
		}, "items"),
	}
}

func mailboxGetDefinition() contracts.Definition {
	d := mailboxListDefinition()
	d.Name, d.Title, d.Executor = "mail.mailbox.get", "Read a mailbox", "go:mail.mailbox.get"
	d.Input = contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id")
	d.Output = mailboxSchema()
	return d
}

func mailboxOnboardDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindExternal,
		Name: "mail.mailbox.onboard", Version: version, Title: "Connect a mailbox",
		Description: "Create and verify one SMTP/POP3S mailbox for the caller, then activate it after both connections pass.",
		Domain:      "mail", Entity: "mail_mailbox", Entities: []string{"mail_mailbox", "mail_channel", "mail_credential"},
		Permission: "mail:configure", Risk: contracts.RiskHigh, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{MailboxOnboardedEvent}, Executor: "go:mail.mailbox.onboard",
		Tags: []string{"mail", "write", "manual-only"}, Loggable: []string{"name", "emailAddress", "smtpHost", "pop3Host"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"name": contracts.Str(1, 120), "emailAddress": contracts.Str(3, 254), "password": contracts.Str(1, 4096),
			"smtpHost": contracts.Str(1, 253), "smtpPort": contracts.Int(1, 65535),
			"smtpTls": contracts.Enum("starttls", "tls"), "smtpUsername": contracts.Str(0, 200),
			"pop3Host": contracts.Str(1, 253), "pop3Port": contracts.Int(1, 65535), "pop3Username": contracts.Str(0, 200),
		}, "name", "emailAddress", "password", "smtpHost", "pop3Host"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": idSchema(), "status": contracts.Enum("active", "unverified")}, "id", "status"),
	}
}

func mailboxUpdateDefinition() contracts.Definition {
	d := mailboxOnboardDefinition()
	d.Name, d.Title, d.Executor = "mail.mailbox.update", "Update a mailbox", "go:mail.mailbox.update"
	d.Risk, d.SuccessEvents = contracts.RiskMedium, []string{MailboxUpdatedEvent}
	d.Kind = contracts.KindSQL
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id": idSchema(), "revision": contracts.Int(1, 1<<53), "name": contracts.Str(1, 120),
		"emailAddress": contracts.Str(3, 254), "defaultSend": {Type: "boolean"},
		"smtpHost": contracts.Str(1, 253), "smtpPort": contracts.Int(1, 65535),
		"smtpTls": contracts.Enum("starttls", "tls"), "smtpUsername": contracts.Str(0, 200),
		"pop3Host": contracts.Str(1, 253), "pop3Port": contracts.Int(1, 65535), "pop3Username": contracts.Str(0, 200),
	}, "id", "revision", "name", "emailAddress", "defaultSend", "smtpHost", "smtpTls", "pop3Host")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id")
	return d
}

func credentialReplaceDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindService,
		Name: "mail.mailbox.credential.replace", Version: version, Title: "Replace a mailbox credential",
		Description: "Seal and replace one external-mail password for the caller's mailbox.",
		Domain:      "mail", Entity: "mail_credential", Entities: []string{"mail_credential", "mail_channel"},
		Permission: "mail:configure", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{CredentialRotated}, Executor: "go:mail.mailbox.credential.replace",
		Tags: []string{"mail", "write", "manual-only"}, Loggable: []string{"mailboxId", "purpose"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"mailboxId": idSchema(), "purpose": contracts.Enum("smtp", "pop3"), "password": contracts.Str(1, 4096),
		}, "mailboxId", "purpose", "password"),
		Output: contracts.Object(map[string]*contracts.Schema{"mailboxId": idSchema(), "purpose": contracts.Enum("smtp", "pop3"), "version": contracts.Int(1, 1<<31)}, "mailboxId", "purpose", "version"),
	}
}

func mailboxEnableDefinition() contracts.Definition {
	return statusDefinition("mail.mailbox.enable", "Enable a mailbox", MailboxEnabledEvent)
}

func mailboxDisableDefinition() contracts.Definition {
	return statusDefinition("mail.mailbox.disable", "Disable a mailbox", MailboxDisabledEvent)
}

func statusDefinition(name, title, event string) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: name, Version: version, Title: title, Description: title + " for the caller only.",
		Domain: "mail", Entity: "mail_mailbox", Permission: "mail:configure",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:" + name, Tags: []string{"mail", "write"},
		Input:  contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": idSchema(), "status": {Type: "string"}}, "id", "status"),
	}
}

func connectionTestDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindExternal,
		Name: "mail.mailbox.connection.test", Version: version, Title: "Test mailbox connections",
		Description: "Authenticate one configured external endpoint and record only a stable verification result.",
		Domain:      "mail", Entity: "mail_channel", Entities: []string{"mail_channel", "mail_credential"},
		Permission: "mail:configure", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{ConnectionTestedEvent}, Executor: "go:mail.mailbox.connection.test",
		Tags: []string{"mail", "write", "manual-only"}, Loggable: []string{"mailboxId", "direction"},
		Input:  contracts.Object(map[string]*contracts.Schema{"mailboxId": idSchema(), "direction": contracts.Enum("send", "receive")}, "mailboxId", "direction"),
		Output: contracts.Object(map[string]*contracts.Schema{"mailboxId": idSchema(), "direction": contracts.Enum("send", "receive"), "status": contracts.Enum("ok", "failed")}, "mailboxId", "direction", "status"),
	}
}

func messageListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: "mail.message.list", Version: version, Title: "List messages",
		Description: "List the caller's own inbound or sent messages.",
		Domain:      "mail", Entity: "mail_message", Permission: "mail:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:mail.message.list", Tags: []string{"mail", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"mailboxId": contracts.Int(0, 1<<53), "folder": contracts.Enum("inbox", "sent"),
			"page": contracts.Int(1, 1000000), "pageSize": contracts.Int(1, 100),
		}, "folder"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"items": contracts.Array(messageSummarySchema(), 0, 100),
			"total": contracts.Int(0, 1<<53), "page": contracts.Int(1, 1000000),
			"pageSize": contracts.Int(1, 100),
		}, "items", "total", "page", "pageSize"),
	}
}

func messageGetDefinition() contracts.Definition {
	d := messageListDefinition()
	d.Name, d.Title, d.Executor = "mail.message.get", "Read a message", "go:mail.message.get"
	d.Input = contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id")
	d.Output = messageDetailSchema()
	return d
}

func messageFlagDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: "mail.message.flag.update", Version: version, Title: "Mark a message read",
		Description: "Mark one unread message of the caller as read.",
		Domain:      "mail", Entity: "mail_message", Permission: "mail:read", Risk: contracts.RiskLow,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{MessageReadEvent}, Executor: "go:mail.message.flag.update",
		Tags: []string{"mail", "write"}, Input: contracts.Object(map[string]*contracts.Schema{
			"id": idSchema(), "read": {Type: "boolean"},
		}, "id", "read"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": idSchema(), "read": {Type: "boolean"}}, "id", "read"),
	}
}

func messageSendDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindService,
		Name: "mail.message.send", Version: version, Title: "Queue an email",
		Description: "Create an outbound message and durable SMTP delivery job for the caller's mailbox.",
		Domain:      "mail", Entity: "mail_message",
		Entities:   []string{"mail_message", "mail_message_recipient", "mail_message_attachment", "mail_outbound_delivery"},
		Permission: "mail:send", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{MessageSubmittedEvent}, Executor: "go:mail.message.send",
		Tags: []string{"mail", "write", "agent"}, Loggable: []string{"clientKey", "mailboxId", "to", "cc", "bcc", "subject", "fileIds"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"clientKey": contracts.Str(1, 64), "mailboxId": idSchema(), "to": addressArray(200),
			"cc": contracts.Array(addressSchema(), 0, 200), "bcc": contracts.Array(addressSchema(), 0, 200),
			"subject": contracts.Str(1, 998), "markdown": contracts.Str(1, 100000),
			"fileIds": contracts.Array(idSchema(), 0, 20), "replyTo": contracts.Str(0, 320),
		}, "clientKey", "mailboxId", "to", "subject", "markdown"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"messageId": idSchema(), "deliveryId": idSchema(), "status": contracts.Enum("queued"),
		}, "messageId", "deliveryId", "status"),
	}
}

func attachmentReadDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: "mail.attachment.read", Version: version, Title: "Read a mail attachment",
		Description: "Return one managed file bound to a message the caller owns.",
		Domain:      "mail", Entity: "mail_message_attachment", Permission: "mail:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:mail.attachment.read", Tags: []string{"mail", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"messageId": idSchema(), "attachmentId": idSchema()}, "messageId", "attachmentId"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"name": contracts.Str(1, 255), "mime": contracts.Str(3, 255),
			"content": contracts.Str(1, 13981016),
		}, "name", "mime", "content"),
	}
}

func deliveryListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: "mail.delivery.list", Version: version, Title: "List email deliveries",
		Description: "List the caller's outbound delivery queue and stable result codes.",
		Domain:      "mail", Entity: "mail_outbound_delivery", Permission: "mail:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:mail.delivery.list", Tags: []string{"mail", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"items": contracts.Array(deliverySchema(), 0, 100),
		}, "items"),
	}
}

func deliveryRetryDefinition() contracts.Definition {
	return deliveryActionDefinition("mail.delivery.retry", "Retry an email delivery", "mail:send", "queued")
}

func deliveryCancelDefinition() contracts.Definition {
	return deliveryActionDefinition("mail.delivery.cancel", "Cancel an email delivery", "mail:send", "cancelled")
}

func deliveryActionDefinition(name, title, permission, status string) contracts.Definition {
	event := DeliveryCancelled
	if status == "queued" {
		event = DeliveryRetriedEvent
	}
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindSQL,
		Name: name, Version: version, Title: title, Description: title + " if it belongs to the caller.",
		Domain: "mail", Entity: "mail_outbound_delivery", Permission: permission, Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:" + name, Tags: []string{"mail", "write"},
		Input:  contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": idSchema(), "status": contracts.Enum(status)}, "id", "status"),
	}
}

func outboundProcessDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindExternal,
		Name: "mail.outbound.process", Version: version, Title: "Process an outbound email",
		Description: "Send one due external SMTP delivery and record its durable outcome.",
		Domain:      "mail", Entity: "mail_outbound_delivery",
		Entities:   []string{"mail_outbound_delivery", "mail_message", "mail_channel", "mail_credential"},
		Permission: "mail:sync", Risk: contracts.RiskHigh, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{MessageDeliveredEvent, DeliveryFailedEvent, DeliveryAmbiguous},
		Executor:      "go:mail.outbound.process", Tags: []string{"mail", "internal", "system-only"},
		Input:  contracts.Object(map[string]*contracts.Schema{"id": idSchema()}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": idSchema(), "status": {Type: "string"}}, "id", "status"),
	}
}

func inboxSyncDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Kind: contracts.KindExternal,
		Name: "mail.inbox.sync", Version: version, Title: "Synchronize a mailbox",
		Description: "Fetch one bounded POP3S batch, import it transactionally and mark remote deletions.",
		Domain:      "mail", Entity: "mail_inbox_sync",
		Entities:   []string{"mail_inbox_sync", "mail_channel", "mail_credential", "mail_message", "mail_message_recipient", "mail_mailbox"},
		Permission: "mail:sync", Risk: contracts.RiskHigh, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{MessageReceivedEvent, MailboxSyncedEvent},
		Executor:      "go:mail.inbox.sync", Tags: []string{"mail", "internal", "system-only"},
		Input: contracts.Object(map[string]*contracts.Schema{"channelId": idSchema()}, "channelId"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"mailboxId": idSchema(), "fetched": contracts.Int(0, 100), "imported": contracts.Int(0, 100),
			"duplicates": contracts.Int(0, 100),
		}, "mailboxId", "fetched", "imported", "duplicates"),
	}
}

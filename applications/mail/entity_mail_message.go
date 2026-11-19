package mail

import "horizon/foundation/kernel/capability/contracts"

func messageEntity() contracts.Entity {
	return contracts.Entity{
		Name: "mail_message", Domain: "mail", Table: "mail_messages",
		Description: "A normalized inbound or outbound email owned by one account.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "mailboxId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "ownerAccountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "direction", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"inbound", "outbound"}},
			{Name: "folder", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"inbox", "sent"}},
			{Name: "providerUid", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "rfcMessageId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "subject", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "fromAddress", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "replyTo", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "bodyText", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "bodyHtml", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "snippet", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "occurredAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "readAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "mailbox", Target: "mail_mailbox", Kind: "one", Via: "mailbox_id", Required: true},
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_account_id", Required: true},
			{Name: "recipients", Target: "mail_message_recipient", Kind: "many", Via: "mail_message_recipients"},
			{Name: "attachments", Target: "mail_message_attachment", Kind: "many", Via: "mail_message_attachments"},
		},
	}
}

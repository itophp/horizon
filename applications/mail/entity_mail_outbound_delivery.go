package mail

import "horizon/foundation/kernel/capability/contracts"

func outboundDeliveryEntity() contracts.Entity {
	return contracts.Entity{
		Name: "mail_outbound_delivery", Domain: "mail", Table: "mail_outbound_deliveries",
		Description: "The durable delivery state of one outbound email.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "mailboxId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "messageId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"queued", "sending", "sent", "failed", "ambiguous", "cancelled"}},
			{Name: "attempts", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "nextAttemptAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "leaseUntil", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "leaseOwner", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "submittedBy", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "sentAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "mailbox", Target: "mail_mailbox", Kind: "one", Via: "mailbox_id", Required: true},
			{Name: "message", Target: "mail_message", Kind: "one", Via: "message_id", Required: true},
			{Name: "submitter", Target: "account", Kind: "one", Via: "submitted_by", Required: true},
		},
		States: []contracts.State{{Name: "queued"}, {Name: "sending"}, {Name: "sent"}, {Name: "failed"}, {Name: "ambiguous"}, {Name: "cancelled"}},
		Transitions: []contracts.Transition{
			{To: "queued", Capability: "mail.message.send"},
			{From: []string{"queued", "failed", "ambiguous"}, To: "sending", Capability: "mail.outbound.process"},
			{From: []string{"sending"}, To: "sent", Capability: "mail.outbound.process"},
			{From: []string{"sending"}, To: "failed", Capability: "mail.outbound.process"},
			{From: []string{"sending"}, To: "ambiguous", Capability: "mail.outbound.process"},
			{From: []string{"sending"}, To: "queued", Capability: "mail.outbound.process"},
			{From: []string{"failed", "ambiguous"}, To: "queued", Capability: "mail.delivery.retry"},
			{From: []string{"queued"}, To: "cancelled", Capability: "mail.delivery.cancel"},
		},
	}
}

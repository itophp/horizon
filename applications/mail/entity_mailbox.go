package mail

import "horizon/foundation/kernel/capability/contracts"

func mailboxEntity() contracts.Entity {
	maxName, maxLength := 120, 254
	return contracts.Entity{
		Name: "mail_mailbox", Domain: "mail", Table: "mail_mailboxes",
		Description: "An external mailbox owned and configured by one IAM account.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "ownerAccountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxName},
			{Name: "emailAddress", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxLength},
			{Name: "defaultSend", Kind: contracts.AttributeBusiness, Type: contracts.TypeBoolean, Required: true},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"unverified", "active", "disabled"}},
			{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_account_id", Required: true},
			{Name: "channels", Target: "mail_channel", Kind: "many", Via: "mail_channels"},
			{Name: "credentials", Target: "mail_credential", Kind: "many", Via: "mail_credentials"},
			{Name: "messages", Target: "mail_message", Kind: "many", Via: "mail_messages"},
		},
		States: []contracts.State{{Name: "unverified"}, {Name: "active"}, {Name: "disabled"}},
		Transitions: []contracts.Transition{
			{To: "unverified", Capability: "mail.mailbox.onboard"},
			{From: []string{"unverified"}, To: "active", Capability: "mail.mailbox.onboard"},
			{From: []string{"active"}, To: "disabled", Capability: "mail.mailbox.disable"},
			{From: []string{"disabled"}, To: "active", Capability: "mail.mailbox.enable"},
		},
	}
}

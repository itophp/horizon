package mail

import "horizon/foundation/kernel/capability/contracts"

func messageRecipientEntity() contracts.Entity {
	maxAddress, maxName := 320, 200
	return contracts.Entity{
		Name: "mail_message_recipient", Domain: "mail", Table: "mail_message_recipients",
		Description: "One addressed recipient of an email.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "messageId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "kind", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"to", "cc", "bcc"}},
			{Name: "address", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxAddress},
			{Name: "displayName", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxName},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{{Name: "message", Target: "mail_message", Kind: "one", Via: "message_id", Required: true}},
	}
}

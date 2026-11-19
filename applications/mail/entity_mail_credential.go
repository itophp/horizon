package mail

import "horizon/foundation/kernel/capability/contracts"

func credentialEntity() contracts.Entity {
	return contracts.Entity{
		Name: "mail_credential", Domain: "mail", Table: "mail_credentials",
		Description: "An encrypted external-mail credential for one transport purpose.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "mailboxId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "purpose", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"smtp", "pop3"}},
			{Name: "ciphertext", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "fingerprint", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "rotatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "lastUsedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{{Name: "mailbox", Target: "mail_mailbox", Kind: "one", Via: "mailbox_id", Required: true}},
	}
}

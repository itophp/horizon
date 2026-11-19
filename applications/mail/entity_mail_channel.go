package mail

import "horizon/foundation/kernel/capability/contracts"

func channelEntity() contracts.Entity {
	maxHost, maxUsername, maxError := 253, 200, 100
	return contracts.Entity{
		Name: "mail_channel", Domain: "mail", Table: "mail_channels",
		Description: "One configured external transport endpoint for a mailbox.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "mailboxId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "direction", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"send", "receive"}},
			{Name: "protocol", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"smtp", "pop3"}},
			{Name: "host", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxHost},
			{Name: "port", Kind: contracts.AttributeBusiness, Type: contracts.TypeInteger, Required: true},
			{Name: "tlsMode", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"starttls", "tls"}},
			{Name: "username", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxUsername},
			{Name: "credentialId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "verificationStatus", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"unknown", "ok", "failed"}},
			{Name: "lastErrorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, MaxLength: &maxError},
			{Name: "lastVerifiedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "nextSyncAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "leaseUntil", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "leaseOwner", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "mailbox", Target: "mail_mailbox", Kind: "one", Via: "mailbox_id", Required: true},
			{Name: "credential", Target: "mail_credential", Kind: "one", Via: "credential_id", Required: true},
		},
	}
}

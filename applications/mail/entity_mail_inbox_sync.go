package mail

import "horizon/foundation/kernel/capability/contracts"

func inboxSyncEntity() contracts.Entity {
	return contracts.Entity{
		Name: "mail_inbox_sync", Domain: "mail", Table: "mail_inbox_syncs",
		Description: "One bounded POP3 retrieval batch for a mailbox.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "mailboxId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "status", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"running", "completed", "failed"}},
			{Name: "fetchedCount", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "importedCount", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "duplicateCount", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "startedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "finishedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{{Name: "mailbox", Target: "mail_mailbox", Kind: "one", Via: "mailbox_id", Required: true}},
	}
}

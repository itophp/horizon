package mail

import "horizon/foundation/kernel/capability/contracts"

func messageAttachmentEntity() contracts.Entity {
	maxContentID, maxFilename, maxMime := 255, 255, 255
	return contracts.Entity{
		Name: "mail_message_attachment", Domain: "mail", Table: "mail_message_attachments",
		Description: "A private managed file bound to one email message.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "messageId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "fileId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "contentId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, MaxLength: &maxContentID},
			{Name: "filename", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, MaxLength: &maxFilename},
			{Name: "mimeType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, MaxLength: &maxMime},
			{Name: "size", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "disposition", Kind: contracts.AttributeSystem, Type: contracts.TypeEnum, Required: true, Enum: []string{"attachment", "inline"}},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "message", Target: "mail_message", Kind: "one", Via: "message_id", Required: true},
			{Name: "file", Target: "file", Kind: "one", Via: "file_id", Required: true},
		},
	}
}

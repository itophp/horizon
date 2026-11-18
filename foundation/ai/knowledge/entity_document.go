package knowledge

import "horizon/foundation/kernel/capability/contracts"

func Entities() []contracts.Entity {
	maxTitle := 200
	return []contracts.Entity{{
		Name: "knowledge_document", Domain: "knowledge", Table: "kb_documents", Key: "id",
		Description: "Original text and indexing metadata for a workspace knowledge document.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, MaxLength: &maxTitle},
			{Name: "source", Kind: contracts.AttributeSystem, Type: contracts.TypeEnum, Required: true, Enum: []string{"manual", "file"}},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "chunks", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Description: "Identifies the current document text and its active vector generation."},
			{Name: "content", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, Sensitive: true},
		},
	}}
}

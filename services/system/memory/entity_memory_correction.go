package memory

import "baseline/services/api/kernel/capability/contracts"

func memoryCorrectionEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_correction", Domain: "memory", Table: "memory_corrections", Key: "id", Description: "An auditable previous version of a corrected memory.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "memoryId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		{Name: "actorId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "previousVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "previousTitle", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "previousSummary", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, Relations: []contracts.Relation{{Name: "memory", Target: "memory_item", Kind: "one", Via: "memory_id", Required: true}}}
}

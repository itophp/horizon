package memory

import "horizon/foundation/kernel/capability/contracts"

func memoryRelationEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_relation", Domain: "memory", Table: "memory_relations", Key: "id", Description: "A tenant-scoped relation between memory items.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "sourceMemoryId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		{Name: "targetMemoryId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		{Name: "relationType", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, Relations: []contracts.Relation{{Name: "source", Target: "memory_item", Kind: "one", Via: "source_memory_id", Required: true}, {Name: "target", Target: "memory_item", Kind: "one", Via: "target_memory_id", Required: true}}}
}

package memory

import "baseline/services/api/kernel/capability/contracts"

func memorySourceEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_source", Domain: "memory", Table: "memory_sources", Key: "id", Description: "Reference from a memory to an original event.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "memoryId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		{Name: "eventId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, Relations: []contracts.Relation{{Name: "memory", Target: "memory_item", Kind: "one", Via: "memory_id", Required: true}, {Name: "event", Target: "memory_event", Kind: "one", Via: "event_id", Required: true}}}
}

package memory

import "baseline/services/api/kernel/capability/contracts"

func memoryInboxEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_event_inbox", Domain: "memory", Table: "memory_event_inbox", Key: "source_event_id", Description: "Durable processing state for a source event.", Attributes: []contracts.Attribute{
		{Name: "sourceEventId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "eventId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "attempts", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "availableAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "completedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		{Name: "deadAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
	}}
}

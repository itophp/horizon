package memory

import "horizon/foundation/kernel/capability/contracts"

func memoryAccessAuditEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_access_audit", Domain: "memory", Table: "memory_access_audit", Key: "id", Description: "An access record without memory content or search terms.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "actorId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "action", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "objectId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "correlationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}}
}

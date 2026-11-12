package memory

import "horizon/foundation/kernel/capability/contracts"

func memoryItemEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_item", Domain: "memory", Table: "memory_items", Key: "id", Description: "An explicitly managed long-term memory.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "partition", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}},
		{Name: "memoryType", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "scopeType", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"user", "project", "department", "organization", "business_entity"}},
		{Name: "scopeId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "ownerId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "summary", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "sensitivity", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "verificationStatus", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"unverified", "confirmed", "rejected"}},
		{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"active", "archived", "deleted"}},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "validFrom", Kind: contracts.AttributeBusiness, Type: contracts.TypeTimestamp, Required: true},
		{Name: "validTo", Kind: contracts.AttributeBusiness, Type: contracts.TypeTimestamp, Nullable: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, States: []contracts.State{{Name: "active"}, {Name: "archived"}, {Name: "deleted", Terminal: true}},
		Transitions: []contracts.Transition{{To: "active", Capability: "memory.item.create"}, {To: "active", Capability: "memory.project.data.create"}, {To: "active", Capability: "memory.group.create"}, {To: "active", Capability: "memory.entity.data.create"}, {From: []string{"active"}, To: "archived", Capability: "memory.item.archive"}, {From: []string{"active"}, To: "archived", Capability: "memory.project.data.archive"}, {From: []string{"active"}, To: "archived", Capability: "memory.group.archive"}, {From: []string{"active"}, To: "archived", Capability: "memory.entity.data.archive"}, {From: []string{"active", "archived"}, To: "deleted", Capability: "memory.item.delete"}, {From: []string{"active", "archived"}, To: "deleted", Capability: "memory.project.data.delete"}, {From: []string{"active", "archived"}, To: "deleted", Capability: "memory.group.delete"}, {From: []string{"active", "archived"}, To: "deleted", Capability: "memory.entity.data.delete"}},
	}
}

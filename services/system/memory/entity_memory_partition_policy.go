package memory

import "baseline/services/api/kernel/capability/contracts"

func memoryPartitionPolicyEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_partition_policy", Domain: "memory", Table: "memory_partition_policies", Key: "id", Description: "Tenant-specific recorded retention policy; no automatic deletion in this phase.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "partition", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}},
		{Name: "retentionDays", Kind: contracts.AttributeBusiness, Type: contracts.TypeInteger, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "updatedBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}}
}

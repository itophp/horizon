package applicationruntime

import "horizon/foundation/kernel/capability/contracts"

func RuntimeInstallationHistoryEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_installation_history", Domain: "runtime", Table: "runtime_installation_history", Key: "id", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "accountId", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
		{Name: "action", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}}
}

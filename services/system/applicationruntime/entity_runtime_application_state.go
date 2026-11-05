package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeApplicationStateEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_application_state", Domain: "runtime", Table: "runtime_application_states", Key: "applicationId", Attributes: []contracts.Attribute{
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "enabled", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
	}}
}

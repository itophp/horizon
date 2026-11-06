package executionplans

import "baseline/services/api/kernel/capability/contracts"

func execution_bindingEntity() contracts.Entity {
	return contracts.Entity{Name: "execution_binding", Domain: "executionplans", Table: "cap_execution_bindings", Key: "capability", Attributes: []contracts.Attribute{
		{Name: "capability", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "capabilityVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "planTenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "planId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "planVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "updatedBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
	}}
}

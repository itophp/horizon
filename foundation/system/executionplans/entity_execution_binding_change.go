package executionplans

import "horizon/foundation/kernel/capability/contracts"

func execution_binding_changeEntity() contracts.Entity {
	return contracts.Entity{Name: "execution_binding_change", Domain: "executionplans", Table: "cap_execution_binding_changes", Key: "id", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "capability", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "capabilityVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "planId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "planVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "action", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "reason", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "accountId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
	}}
}

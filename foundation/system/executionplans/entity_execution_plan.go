package executionplans

import "horizon/foundation/kernel/capability/contracts"

func execution_planEntity() contracts.Entity {
	return contracts.Entity{Name: "execution_plan", Domain: "executionplans", Table: "cap_execution_plans", Key: "planId", Attributes: []contracts.Attribute{
		{Name: "workflowHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "planId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "capability", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "capabilityVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "mode", Kind: contracts.AttributeSystem, Type: contracts.TypeEnum, Required: true, Enum: []string{"direct", "workflow", "graph"}},
		{Name: "implementationRef", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "contractHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"draft", "validated", "published", "archived"}},
		{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "publishedBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "publishedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: false},
	}, States: []contracts.State{{Name: "draft"}, {Name: "validated"}, {Name: "published", Terminal: true}, {Name: "archived", Terminal: true}}, Transitions: []contracts.Transition{{From: []string{"draft"}, To: "validated", Capability: "execution.plan.validate"}, {From: []string{"validated"}, To: "published", Capability: "execution.plan.publish"}}}
}

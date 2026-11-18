package configuration

import "horizon/foundation/kernel/capability/contracts"

func Entities() []contracts.Entity {
	return []contracts.Entity{{
		Name: "ai_configuration", Domain: "aiConfig", Table: "ai_configuration", Key: "tenantId",
		Description: "The workspace AI model and service configuration currently in effect.",
		Attributes: []contracts.Attribute{
			{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "valuesJson", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "Validated AI settings with sealed credentials."},
		},
	}}
}

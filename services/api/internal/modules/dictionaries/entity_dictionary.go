package dictionaries

import "baseline/services/api/internal/platform/capability/contracts"

func dictionaryEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "dictionary",
		Domain:      "dictionaries",
		Table:       "gov_dictionaries",
		Description: "A published reference list shared by the workspace.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "scopeKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Description: "Always workspace:0 after ADR 007."},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "spec", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "Target, provider and visibility settings."},
			{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "items", Target: "dictionary_item", Kind: "many", Via: "gov_dictionary_items", Required: false},
		},
	}
}

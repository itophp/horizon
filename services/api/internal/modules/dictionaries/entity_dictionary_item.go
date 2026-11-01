package dictionaries

import "baseline/services/api/internal/platform/capability/contracts"

func dictionaryItemEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "dictionary_item",
		Domain:      "dictionaries",
		Table:       "gov_dictionary_items",
		Description: "One selectable option of a dictionary.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "dictionaryId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "scopeKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "parentCode", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "Empty string means a root option."},
			{Name: "spec", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "Labels, ordering and the default flag."},
		},
		Relations: []contracts.Relation{
			{Name: "dictionary", Target: "dictionary", Kind: "one", Via: "dictionary_id", Required: true},
		},
	}
}

package settings

import "baseline/services/api/internal/platform/capability/contracts"

func settingsVersionEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "settings_version",
		Domain:      "settings",
		Table:       "gov_versions",
		Description: "One immutable published revision of a workspace settings resource.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "domain", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "settings or dictionary; the table has no CHECK on this column."},
			{Name: "resourceKey", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "settings:<key> or dictionary:<code>."},
			{Name: "scopeKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "entityId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "Published change set."},
			{Name: "reason", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "actorId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "actor", Target: "account", Kind: "one", Via: "actor_id", Required: true},
		},
	}
}

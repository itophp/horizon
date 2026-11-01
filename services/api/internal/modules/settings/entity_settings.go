package settings

import "baseline/services/api/internal/platform/capability/contracts"

// settingsEntity mirrors gov_values: the key/value store behind every settings
// read. It has no id column, so the key is the config key.
func settingsEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "settings",
		Domain:      "settings",
		Table:       "gov_values",
		Description: "One published configuration value, shared by the whole workspace.",
		Key:         "configKey",
		Attributes: []contracts.Attribute{
			{Name: "scopeKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Description: "workspace:0 for shared settings."},
			{Name: "configKey", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "valueJson", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "Registered value; secrets are encrypted before this point."},
		},
	}
}

package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeInstallationTemplateEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_installation_template", Domain: "runtime", Table: "runtime_installation_templates", Key: "pluginId", Attributes: []contracts.Attribute{
		{Name: "pluginId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "enabled", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
	}}
}

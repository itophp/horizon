package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeInstallationEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_installation", Domain: "runtime", Table: "runtime_installations", Key: "pluginId", Attributes: []contracts.Attribute{
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"installed", "enabled", "disabled", "uninstalled"}},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "digest", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "previousVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, States: []contracts.State{{Name: "installed"}, {Name: "enabled"}, {Name: "disabled"}, {Name: "uninstalled"}}, Transitions: []contracts.Transition{
		{To: "installed", Capability: "application.runtime.installation.install"},
		{From: []string{"uninstalled"}, To: "installed", Capability: "application.runtime.installation.install"},
		{From: []string{"installed", "disabled"}, To: "enabled", Capability: "application.runtime.installation.enable"},
		{From: []string{"installed", "enabled"}, To: "disabled", Capability: "application.runtime.installation.disable"},
		{From: []string{"installed", "enabled", "disabled"}, To: "uninstalled", Capability: "application.runtime.installation.uninstall"},
	}}
}

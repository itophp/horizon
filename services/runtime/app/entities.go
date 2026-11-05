package app

import (
	"baseline/services/api/applications/articles"
	"baseline/services/api/applications/drive"
	"baseline/services/api/applications/example"

	"baseline/services/api/integrations/mcp"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/plugin"
	"baseline/services/api/system/dictionaries"
	"baseline/services/api/system/files"
	"baseline/services/api/system/identity"
	"baseline/services/api/system/memory"
	"baseline/services/api/system/notifications"
	"baseline/services/api/system/orchestration"
	"baseline/services/api/system/settings"
)

// Entities lists every hand written business entity description. Modules join
// this list when they ship their entities.go, which keeps the composition root
// and the entity check reading the same source.
func Entities() []contracts.Entity { return EntitiesWithPlugins(BuiltinPlugins(nil)) }

// EntitiesWithPlugins builds metadata from the same registered plugins as capabilities.
func EntitiesWithPlugins(plugins *plugin.Manager) []contracts.Entity {
	out := []contracts.Entity{}
	out = append(out, articles.Entities()...)
	out = append(out, drive.Entities()...)
	out = append(out, plugins.Catalog.Entities...)
	out = append(out, orchestration.Entities()...)
	out = append(out, identity.Entities()...)
	out = append(out, memory.Entities()...)
	out = append(out, example.Entities()...)
	out = append(out, notifications.Entities()...)
	out = append(out, files.Entities()...)
	out = append(out, dictionaries.Entities()...)
	out = append(out, settings.Entities()...)
	out = append(out, mcp.Entities()...)
	return out
}

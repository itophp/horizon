package app

import (
	"horizon/applications/articles"
	"horizon/applications/drive"
	"horizon/applications/example"

	"horizon/foundation/integrations/mcp"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/plugin"
	"horizon/foundation/system/applicationruntime"
	"horizon/foundation/system/dictionaries"
	"horizon/foundation/system/executionplans"
	"horizon/foundation/system/files"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/memory"
	"horizon/foundation/system/notifications"
	"horizon/foundation/system/orchestration"
	"horizon/foundation/system/settings"
)

// Entities lists every hand written business entity description. Modules join
// this list when they ship their entities.go, which keeps the composition root
// and the entity check reading the same source.
func Entities() []contracts.Entity { return EntitiesWithPlugins(BuiltinPlugins(nil)) }

// EntitiesWithPlugins builds metadata from the same registered plugins as capabilities.
func EntitiesWithPlugins(plugins *plugin.Manager) []contracts.Entity {
	out := append(applicationruntime.Entities(), executionplans.Entities()...)
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

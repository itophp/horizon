package app

import (
	"horizon/foundation/ai/automation"

	"horizon/foundation/integrations/mcp"
	"horizon/foundation/kernel/capability/contracts"
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
func Entities() []contracts.Entity { return EntitiesWithRegistration(Registration{}) }

// EntitiesWithPlugins builds metadata from the same registered plugins as capabilities.
func EntitiesWithRegistration(registration Registration) []contracts.Entity {
	out := append(applicationruntime.Entities(), executionplans.Entities()...)
	out = append(out, automation.Entities()...)
	out = append(out, registration.Entities...)
	if plugins := registration.Plugins; plugins != nil {
		out = append(out, plugins.Catalog.Entities...)
	}
	out = append(out, orchestration.Entities()...)
	out = append(out, identity.Entities()...)
	out = append(out, memory.Entities()...)
	out = append(out, notifications.Entities()...)
	out = append(out, files.Entities()...)
	out = append(out, dictionaries.Entities()...)
	out = append(out, settings.Entities()...)
	out = append(out, mcp.Entities()...)
	return out
}

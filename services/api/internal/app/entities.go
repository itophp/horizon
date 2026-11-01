package app

import (
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/articles"
	"baseline/services/api/internal/modules/dictionaries"
	"baseline/services/api/internal/modules/example"
	"baseline/services/api/internal/modules/files"
	"baseline/services/api/internal/modules/mcp"
	"baseline/services/api/internal/modules/notifications"
	"baseline/services/api/internal/modules/orchestration"
	"baseline/services/api/internal/modules/settings"
	"baseline/services/api/internal/platform/capability/contracts"
)

// Entities lists every hand written business entity description. Modules join
// this list when they ship their entities.go, which keeps the composition root
// and the entity check reading the same source.
func Entities() []contracts.Entity {
	out := []contracts.Entity{}
	out = append(out, articles.Entities()...)
	out = append(out, orchestration.Entities()...)
	out = append(out, identity.Entities()...)
	out = append(out, example.Entities()...)
	out = append(out, notifications.Entities()...)
	out = append(out, files.Entities()...)
	out = append(out, dictionaries.Entities()...)
	out = append(out, settings.Entities()...)
	out = append(out, mcp.Entities()...)
	return out
}

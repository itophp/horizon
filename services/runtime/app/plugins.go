package app

import (
	"baseline/services/api/applications/projects"
	"baseline/services/api/kernel/plugin"
	"baseline/services/api/system/identity"
	"context"
)

// BuiltinPlugins is the single plugin composition root for runtime and offline checks.
func BuiltinPlugins(projectsModule *projects.Module) *plugin.Manager {
	if projectsModule == nil {
		projectsModule = projects.New(nil)
	}
	manager := plugin.New(projects.Plugin{Module: projectsModule})
	if err := manager.Register(context.Background()); err != nil {
		panic(err)
	}
	// Offline tools perform the same identity-catalog conflict checks as startup.
	if err := identity.New(nil, false).ConfigurePluginPermissions(manager.Catalog.Permissions); err != nil {
		panic(err)
	}
	return manager
}

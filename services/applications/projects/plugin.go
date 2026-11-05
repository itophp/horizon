package projects

import (
	"context"
	_ "embed"
	"sort"

	"baseline/services/api/kernel/plugin"
	"baseline/services/api/system/identity"
)

//go:embed plugin.yaml
var manifestYAML []byte

// Plugin adapts the module's existing declarations without copying business logic.
type Plugin struct{ Module *Module }

func (p Plugin) Manifest() plugin.Manifest {
	manifest, err := plugin.ParseManifest(manifestYAML)
	if err != nil {
		panic(err)
	}
	return manifest
}
func (p Plugin) Register(_ context.Context, r *plugin.Registrar) error {
	entries := p.Module.Entries()
	r.Capabilities(entries)
	r.Entities(Entities())
	r.Applications(RuntimeApplication())
	permissions, events := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		permissions[entry.Definition.Permission] = true
		for _, event := range entry.Definition.SuccessEvents {
			events[event] = true
		}
	}
	permissionNames, eventNames := make([]string, 0, len(permissions)), make([]string, 0, len(events))
	for name := range permissions {
		permissionNames = append(permissionNames, name)
	}
	for name := range events {
		eventNames = append(eventNames, name)
	}
	sort.Strings(permissionNames)
	sort.Strings(eventNames)
	catalog := map[string]plugin.Permission{}
	for _, definition := range identity.PermissionCatalog() {
		catalog[definition.Code] = plugin.Permission{Code: definition.Code, Module: definition.Module, Function: definition.Function, Action: definition.Action, ScopeConfigurable: definition.ScopeConfigurable, TemporaryEligible: definition.TemporaryEligible}
	}
	for _, name := range permissionNames {
		r.Permissions(catalog[name])
	}
	r.Events(eventNames...)
	return nil
}
func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

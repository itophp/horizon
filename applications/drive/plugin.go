package drive

import (
	"context"
	_ "embed"
	"sort"

	"horizon/foundation/ai/harness"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/plugin"
	"horizon/foundation/runtime/app"
)

//go:embed plugin.yaml
var manifestYAML []byte

// Plugin contributes the drive module to the builtin plugin catalog.
// Permission metadata is declarative manifest data. The module publishes no
// application package, so its capabilities carry no runtime installation.
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
	// The registrar only needs the event names the capability definitions
	// produce; the permission catalog comes from the manifest.
	events := map[string]bool{}
	for _, entry := range entries {
		for _, event := range entry.Definition.SuccessEvents {
			events[event] = true
		}
	}
	eventNames := make([]string, 0, len(events))
	for name := range events {
		eventNames = append(eventNames, name)
	}
	sort.Strings(eventNames)
	r.Events(eventNames...)
	return nil
}

// Workers runs the storage cleanup loop under the platform worker supervision.
func (p Plugin) Workers() []app.Worker {
	return []app.Worker{func(ctx context.Context, gw *gateway.Gateway, _ *harness.Service) {
		RunCleanup(ctx, p.Module.pool, gw)
	}}
}

func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

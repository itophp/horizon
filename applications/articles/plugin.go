package articles

import (
	workflow "horizon/foundation/kernel/workflow/spec"

	"context"
	_ "embed"
	"sort"

	"horizon/foundation/kernel/plugin"
)

//go:embed plugin.yaml
var manifestYAML []byte

// Plugin contributes the articles module to the builtin plugin catalog.
// Permission metadata is declarative manifest data.
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

func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

// Workflows exposes the application's durable flow assets to the deployment
// aggregation, which collects them by optional interface.
func (p Plugin) Workflows() []workflow.Definition { return Workflows() }

package example

import (
	workflow "horizon/foundation/kernel/workflow/spec"

	"context"
	_ "embed"
	"sort"

	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/plugin"
)

//go:embed plugin.yaml
var manifestYAML []byte

// Plugin contributes the example module to the builtin plugin catalog.
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
	r.Applications(ExampleRuntimeApplication())
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

// AttachGateway forwards the gateway to the module's orchestration capability.
func (p Plugin) AttachGateway(g *gateway.Gateway) { p.Module.AttachGateway(g) }

func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

// Workflows and Graphs expose the application's durable flow assets to the
// deployment aggregation, which collects them by optional interface.
func (p Plugin) Workflows() []workflow.Definition { return Workflows() }

func (p Plugin) Graphs() []workflow.GraphDefinition { return Graphs() }

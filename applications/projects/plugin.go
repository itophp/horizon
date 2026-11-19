package projects

import (
	workflow "horizon/foundation/kernel/workflow/spec"

	"context"
	_ "embed"
	"sort"

	"horizon/foundation/kernel/plugin"
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
	r.Applications(runtimeApplication("2.0.0"), runtimeApplication("2.0.1"), runtimeApplication("2.0.2"), RuntimeApplication())
	// Permission metadata is declarative manifest data; the registrar only
	// needs the event names the capability definitions produce.
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
func (p Plugin) AttachModel(generate func(context.Context, string, string) (string, error)) {
	p.Module.GenerateAI = generate
}

func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

// Workflows exposes the application's durable flow assets to the deployment
// aggregation, which collects them by optional interface.
func (p Plugin) Workflows() []workflow.Definition { return Workflows() }

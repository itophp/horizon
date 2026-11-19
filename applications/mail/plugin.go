package mail

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

// Plugin contributes the mail module to the builtin plugin catalog. Permission
// metadata and the mail_attachment file purpose are declarative manifest data.
type Plugin struct{ Module *Module }

func (p Plugin) Manifest() plugin.Manifest {
	manifest, err := plugin.ParseManifest(manifestYAML)
	if err != nil {
		panic(err)
	}
	return manifest
}

func (p Plugin) Register(_ context.Context, r *plugin.Registrar) error {
	RegisterUIContract()
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

// Workers runs outbound delivery and inbox sync under the platform worker
// supervision with the gateway attached.
func (p Plugin) Workers() []app.Worker {
	return []app.Worker{func(ctx context.Context, gw *gateway.Gateway, _ *harness.Service) {
		Run(ctx, p.Module.Pool, gw, p.Module)
	}}
}

func (p Plugin) Start(context.Context) error { return nil }
func (p Plugin) Stop(context.Context) error  { return nil }

package app

import (
	"context"

	"horizon/foundation/ai/automation"
	"horizon/foundation/ai/harness"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/plugin"
	workflow "horizon/foundation/kernel/workflow/spec"
	"horizon/foundation/system/governance"
)

type Worker func(context.Context, *gateway.Gateway, *harness.Service)

// Registration holds application contributions for one deployment instance.
type Registration struct {
	Plugins       *plugin.Manager
	Entries       []registry.Entry
	Entities      []contracts.Entity
	Applications  []ar.ApplicationV2
	AttachGateway []func(*gateway.Gateway)
	Workers       []Worker
	Workflows     []workflow.Definition
	Graphs        []workflow.GraphDefinition
}

type Register func(*governance.Core) Registration

// PlatformWorkers registers background services owned by the foundation itself.
func PlatformWorkers(core *governance.Core) []Worker {
	return []Worker{func(ctx context.Context, _ *gateway.Gateway, launcher *harness.Service) {
		automation.NewWorker(core.Pool, launcher, core.Identity).Run(ctx)
	}}
}

func (r Registration) WorkflowDefinitions() []workflow.Definition {
	return append(append([]workflow.Definition(nil), r.Workflows...), workflow.RuntimeDesigner())
}

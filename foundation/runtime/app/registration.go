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

	// Application-domain wiring declared by the deployment for platform
	// services. Values are deployment composition data; the kernel only
	// carries the references and never hardcodes business names.
	MemoryActivityPages    []string
	MemoryProjectWrite     string
	MemoryProjectAuthority string

	// MemoryEntityScopes declares the application resource types memory may
	// reference, with the guarding permission and existence capability.
	MemoryEntityScopes []MemoryEntityScope
}

// MemoryEntityScope is deployment composition data for one resource type.
type MemoryEntityScope struct {
	Resource      string
	Permission    string
	AggregateType string
	Authority     string
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

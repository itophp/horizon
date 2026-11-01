package orchestration

import (
	runtime "baseline/services/api/internal/ai/orchestration"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
	"context"
)

type Module struct{ Engine *runtime.Engine }

func (m *Module) Entries() []registry.Entry {
	object := func() *contracts.Schema { return &contracts.Schema{Type: "object", AdditionalProperties: true} }
	id := contracts.Object(map[string]*contracts.Schema{"runId": {Type: "integer", Minimum: ptr(1)}}, "runId")
	definition := func(name, permission string, read bool, input *contracts.Schema) contracts.Definition {
		d := contracts.Definition{Layer: contracts.LayerOrchestration, Kind: contracts.KindService, Name: "orchestration.workflow." + name, Version: "1.0.0", Title: "Workflow " + name, Description: "Execute a versioned workflow operation for the current account.", Domain: "orchestration", Entity: "orchestration_checkpoint", Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, Executor: "go:orchestration." + name, Input: input, Output: object(), Loggable: []string{"flow", "version", "runId"}}
		if !read {
			d.Risk = contracts.RiskHigh
			d.Approval = contracts.ApprovalConfirm
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
		}
		return d
	}
	start := definition("run", "orchestration:run", false, contracts.Object(map[string]*contracts.Schema{"flow": {Type: "string"}, "version": {Type: "string"}, "input": object()}, "flow", "version", "input"))
	start.SuccessEvents = []string{"orchestration.workflow.accepted"}
	cancel := definition("cancel", "orchestration:cancel", false, id)
	cancel.SuccessEvents = []string{"orchestration.workflow.cancelled"}
	return []registry.Entry{
		{Definition: start, Handler: contracts.Handler{ExecuteTx: m.Engine.StartTx}},
		{Definition: cancel, Handler: contracts.Handler{ExecuteTx: m.Engine.CancelTx}},
		{Definition: definition("get", "orchestration:read", true, id), Handler: contracts.Handler{Execute: m.Engine.Query}},
		{Definition: definition("list", "orchestration:read", true, contracts.Object(nil)), Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
			return contracts.Output{Data: map[string]any{"workflows": m.Engine.Definitions()}}, nil
		}}},
	}
}
func ptr(v float64) *float64 { return &v }

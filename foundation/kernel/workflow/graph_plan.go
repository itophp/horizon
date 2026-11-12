package orchestration

import (
	"context"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/workflow/spec"
	"horizon/internal/repository/dbgen"
)

// Graph plans may replace only read-only, non-transient capabilities with the
// exact same I/O schemas. Business writes remain outside the Graph adapter.
func (e *Engine) ValidateGraphPlan(ctx context.Context, _ *dbgen.Queries, target contracts.Definition, ref string) (string, error) {
	if !target.ReadOnly || target.TransientResult || target.Input == nil || target.Output == nil {
		return "", contracts.Problem("invalid")
	}
	for _, g := range e.graphs {
		if g.Key() != ref {
			continue
		}
		if contracts.Digest(contracts.Input{"input": g.Input, "output": g.Output}) != contracts.Digest(contracts.Input{"input": target.Input, "output": target.Output}) {
			return "", contracts.Problem("invalid")
		}
		if err := e.authorizeGraph(ctx, g); err != nil {
			return "", err
		}
		return g.Hash(), nil
	}
	return "", contracts.Problem("not_found")
}
func (e *Engine) startGraphPlanTx(ctx context.Context, tx pgx.Tx, b contracts.ExecutionBinding, input contracts.Input) (int64, error) {
	target, ok := e.registry.Resolve(b.Plan.CapabilityID, b.Plan.CapabilityVersion)
	if !ok {
		return 0, contracts.Problem("not_found")
	}
	hash, err := e.ValidateGraphPlan(ctx, dbgen.New(tx), target.Definition, b.Plan.ImplementationRef)
	if err != nil {
		return 0, err
	}
	if hash != b.Plan.GraphHash {
		return 0, contracts.Problem("invalid")
	}
	for _, g := range e.graphs {
		if g.Key() == b.Plan.ImplementationRef {
			d := spec.GraphWorkflow(g)
			out, err := e.startTx(ctx, tx, contracts.Input{"flow": d.Name, "version": d.Version, "input": map[string]any(input)}, "graph:result")
			if err != nil {
				return 0, err
			}
			id, ok := out.Data.(map[string]any)["runId"].(int64)
			if !ok {
				return 0, contracts.Problem("invalid")
			}
			return id, nil
		}
	}
	return 0, contracts.Problem("unavailable")
}

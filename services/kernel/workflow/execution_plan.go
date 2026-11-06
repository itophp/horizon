package orchestration

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgtype"
	"slices"
	"strings"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5"
)

// ValidatePlanReference accepts only registered immutable workflow references.
// An unconditional or mutually exclusive native result preserves the I/O contract.
func (e *Engine) ValidatePlanReference(ctx context.Context, target contracts.Definition, ref string) (string, error) {
	return e.ValidatePlanWithQueries(ctx, dbgen.New(e.Core.Pool), target, ref)
}
func (e *Engine) ValidatePlanWithQueries(ctx context.Context, q *dbgen.Queries, target contracts.Definition, ref string) (string, error) {
	d, ok := e.definitions[ref]
	if !ok || e.registry == nil {
		return "", contracts.Problem("unavailable")
	}
	if err := d.ValidateWithGraphs(e.registry, e.graphs); err != nil {
		return "", contracts.Problem("invalid")
	}
	if target.TransientResult || target.Layer != contracts.LayerAtomic || !target.AllowsOrigin(contracts.OriginWorkflow) {
		return "", contracts.Problem("invalid")
	}
	if contracts.Digest(contracts.Input{"schema": d.Input}) != contracts.Digest(contracts.Input{"schema": target.Input}) {
		return "", contracts.Problem("invalid")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return "", err
	}
	result := []string{}
	for _, s := range d.Steps {
		if s.Graph != nil {
			graph, err := e.graphDefinition(s.Graph)
			if err != nil {
				return "", err
			}
			if err = e.authorizeGraph(ctx, graph); err != nil {
				return "", err
			}
			continue
		}
		entry, ok := e.registry.Resolve(s.Capability, s.Version)
		if !ok || s.Agent != nil || s.Compensation != nil || entry.Definition.TransientResult || (entry.Definition.Permission != "" && !p.Can(entry.Definition.Permission)) || !entry.Definition.AllowsOrigin(contracts.OriginWorkflow) || slices.Contains(entry.Definition.Tags, "manual-only") || slices.Contains(entry.Definition.Tags, "internal-only") || slices.Contains(entry.Definition.Tags, "system-only") {
			return "", contracts.Problem("forbidden")
		}
		if target.ReadOnly && !entry.Definition.ReadOnly {
			return "", contracts.Problem("invalid")
		}
		if s.CallMode == "invoke_native" {
			if entry.Definition.Key() != target.Key() || s.Poll != nil || s.MaxAttempts > 1 {
				return "", contracts.Problem("invalid")
			}
			result = append(result, s.ID)
		} else if entry.Definition.Key() == target.Key() {
			return "", contracts.Problem("invalid")
		}
	}
	if len(result) == 0 {
		return "", contracts.Problem("invalid")
	}
	// Result must follow every node; deterministic specs already validate edges.
	deps := map[string][]string{}
	for _, s := range d.Steps {
		deps[s.ID] = append(deps[s.ID], s.DependsOn...)
		for _, m := range s.Input {
			if m.From != "input" {
				deps[s.ID] = append(deps[s.ID], m.From)
			}
		}
		if s.When != nil {
			deps[s.ID] = append(deps[s.ID], s.When.From)
		}
	}
	var visits func(string, string) bool
	visits = func(node, want string) bool {
		for _, dep := range deps[node] {
			if dep == want || visits(dep, want) {
				return true
			}
		}
		return false
	}
	if len(result) > 1 {
		var group *When
		for _, s := range d.Steps {
			if slices.Contains(result, s.ID) {
				if s.When == nil {
					return "", contracts.Problem("invalid")
				}
				if group == nil {
					group = s.When
				} else if group.From != s.When.From || group.Field != s.When.Field {
					return "", contracts.Problem("invalid")
				}
			}
		}
	}
	for _, s := range d.Steps {
		if slices.Contains(result, s.ID) {
			continue
		}
		covered := false
		for _, id := range result {
			if visits(id, s.ID) {
				covered = true
			}
		}
		if !covered {
			return "", contracts.Problem("invalid")
		}
	}
	if err = e.planDependencies(ctx, q, p.TenantID, target.Key(), ref, map[string]bool{}); err != nil {
		return "", err
	}
	return d.Hash(), nil
}
func (e *Engine) StartPlanTx(ctx context.Context, tx pgx.Tx, b contracts.ExecutionBinding, in contracts.Input) (int64, error) {
	if b.Plan.Mode == "graph" {
		return e.startGraphPlanTx(ctx, tx, b, in)
	}
	d, ok := e.definitions[b.Plan.ImplementationRef]
	if !ok || d.Hash() != b.Plan.WorkflowHash {
		return 0, contracts.Problem("unavailable")
	}
	target, ok := e.registry.Resolve(b.Plan.CapabilityID, b.Plan.CapabilityVersion)
	if !ok {
		return 0, contracts.Problem("unavailable")
	}
	hash, err := e.ValidatePlanWithQueries(ctx, dbgen.New(tx), target.Definition, b.Plan.ImplementationRef)
	if err != nil || hash != b.Plan.WorkflowHash {
		return 0, contracts.Problem("invalid")
	}
	result := []string{}
	for _, s := range d.Steps {
		if s.CallMode == "invoke_native" {
			result = append(result, s.ID)
		}
	}
	name, version, _ := strings.Cut(b.Plan.ImplementationRef, "@")
	out, err := e.startTx(ctx, tx, contracts.Input{"flow": name, "version": version, "input": map[string]any(in)}, strings.Join(result, ","))
	if err != nil {
		return 0, err
	}
	id, ok := out.Data.(map[string]any)["runId"].(int64)
	if !ok {
		return 0, contracts.Problem("invalid")
	}
	return id, nil
}
func (e *Engine) failPlan(ctx context.Context, row dbgen.OrchCandidatesRow, code string) error {
	q := dbgen.New(e.Core.Pool)
	parent, err := q.ExecutionRootForRun(ctx, dbgen.ExecutionRootForRunParams{TenantID: row.TenantID, AccountID: row.AccountID, RunID: pgtype.Int8{Int64: row.ID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if code == "" {
		code = "workflow_failed"
	}
	_, err = q.ExecutionFinishWorkflow(ctx, dbgen.ExecutionFinishWorkflowParams{TenantID: row.TenantID, AccountID: row.AccountID, InvocationID: parent, Status: string(contracts.StatusFailed), ErrorCode: code})
	return err
}

func (e *Engine) planDependencies(ctx context.Context, q *dbgen.Queries, tenant, owner, ref string, path map[string]bool) error {
	if path[owner] || len(path) >= gateway.MaxDepth {
		return contracts.Problem("invalid")
	}
	path[owner] = true
	defer delete(path, owner)
	d, ok := e.definitions[ref]
	if !ok {
		return contracts.Problem("unavailable")
	}
	for _, s := range d.Steps {
		if s.CallMode == "invoke_native" || s.Graph != nil {
			continue
		}
		var id string
		var version int64
		if s.CallMode == "invoke_pinned_plan" {
			id = s.PlanID
			version = s.PlanVersion
		} else {
			if path[s.Capability+"@"+s.Version] {
				return contracts.Problem("invalid")
			}
			b, err := q.ExecutionGetBinding(ctx, dbgen.ExecutionGetBindingParams{TenantID: tenant, Capability: s.Capability, CapabilityVersion: s.Version})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			id = b.PlanID
			version = b.PlanVersion
		}
		planTenant := tenant
		entry, ok := e.registry.Resolve(s.Capability, s.Version)
		if !ok {
			return contracts.Problem("invalid")
		}
		if id == contracts.DefaultExecutionBinding(entry.Definition).Plan.PlanID {
			planTenant = ""
		}
		plan, err := q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: planTenant, PlanID: id, Version: version})
		if err != nil {
			return err
		}
		if plan.Status != "published" || plan.Capability != s.Capability || plan.CapabilityVersion != s.Version {
			return contracts.Problem("invalid")
		}
		if plan.Mode == "workflow" {
			definition, ok := e.definitions[plan.ImplementationRef]
			if !ok || definition.Hash() != plan.WorkflowHash {
				return contracts.Problem("invalid")
			}
			if err = e.planDependencies(ctx, q, tenant, entry.Definition.Key(), plan.ImplementationRef, path); err != nil {
				return err
			}
		}
	}
	return nil
}

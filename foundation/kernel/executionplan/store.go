// Package executionplan persists execution choices without owning capability
// definitions, authorization, business transactions or workflow execution.
package executionplan

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/internal/repository/dbgen"
)

func ContractHash(d contracts.Definition) string {
	return contracts.Digest(contracts.Input{"key": d.Key(), "input": d.Input, "output": d.Output})
}

func Seed(ctx context.Context, q *dbgen.Queries, d contracts.Definition) error {
	b := contracts.DefaultExecutionBinding(d)
	return q.ExecutionSeedDirect(ctx, dbgen.ExecutionSeedDirectParams{PlanID: b.Plan.PlanID, Capability: d.Name, CapabilityVersion: d.Version, Name: d.Title, ImplementationRef: d.Key(), ContractHash: ContractHash(d)})
}

// Resolve returns an immutable value, never a mutable registry entry. Database
// failures fail closed; only absence of an override selects the platform plan.
func Resolve(ctx context.Context, q *dbgen.Queries, tenant string, d contracts.Definition) (contracts.ExecutionBinding, error) {
	if err := Seed(ctx, q, d); err != nil {
		return contracts.ExecutionBinding{}, err
	}
	b := contracts.DefaultExecutionBinding(d)
	row, err := q.ExecutionGetBinding(ctx, dbgen.ExecutionGetBindingParams{TenantID: tenant, Capability: d.Name, CapabilityVersion: d.Version})
	if errors.Is(err, pgx.ErrNoRows) {
		return b, nil
	}
	if err != nil {
		return b, err
	}
	plan, err := q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: row.PlanTenantID, PlanID: row.PlanID, Version: row.PlanVersion})
	if err != nil {
		return b, err
	}
	if plan.Status != "published" || plan.Capability != d.Name || plan.CapabilityVersion != d.Version || plan.ContractHash != ContractHash(d) || !Executable(plan, d) {
		return b, contracts.Problem("unavailable")
	}
	b.ScopeType = "tenant"
	b.ScopeID = tenant
	b.BindingID = d.Key() + ".tenant:" + tenant
	b.Revision = row.Revision
	b.Plan = Plan(plan)
	return b, nil
}
func Plan(p dbgen.CapExecutionPlan) contracts.ExecutionPlan {
	result := contracts.ExecutionPlan{PlanID: p.PlanID, CapabilityID: p.Capability, CapabilityVersion: p.CapabilityVersion, Version: int(p.Version), Mode: p.Mode, ImplementationRef: p.ImplementationRef, Status: p.Status}
	// The existing workflow_hash column stores the pinned Eino implementation digest.
	if p.Mode == "graph" {
		result.GraphHash = p.WorkflowHash
	} else {
		result.WorkflowHash = p.WorkflowHash
	}
	return result
}

func Decode(raw []byte, d contracts.Definition) (contracts.ExecutionBinding, error) {
	var b contracts.ExecutionBinding
	if err := json.Unmarshal(raw, &b); err != nil {
		return b, contracts.Problem("unavailable")
	}
	if (b.Plan.Mode != "direct" && b.Plan.Mode != "workflow" && b.Plan.Mode != "graph") || b.Plan.CapabilityID != d.Name || b.Plan.CapabilityVersion != d.Version || (b.Plan.Mode == "direct" && b.Plan.ImplementationRef != d.Key()) || (b.Plan.Mode == "workflow" && b.Plan.WorkflowHash == "") || (b.Plan.Mode == "graph" && b.Plan.GraphHash == "") || b.Plan.Status != "published" || b.Plan.Version < 1 {
		return b, contracts.Problem("unavailable")
	}
	return b, nil
}

func Executable(p dbgen.CapExecutionPlan, d contracts.Definition) bool {
	return p.Mode == "direct" && p.ImplementationRef == d.Key() || (p.Mode == "workflow" || p.Mode == "graph") && p.WorkflowHash != ""
}

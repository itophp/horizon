package gateway

import (
	"context"
	"encoding/json"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/executionplan"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// WorkflowPlanRuntime enqueues infrastructure state only. Business nodes run
// after commit in the existing durable Workflow engine.
type WorkflowPlanRuntime interface {
	StartPlanTx(context.Context, pgx.Tx, contracts.ExecutionBinding, contracts.Input) (int64, error)
}

func (g *Gateway) AttachWorkflowPlans(runtime WorkflowPlanRuntime) { g.workflowPlans = runtime }

// InvokeNative is available only to the run pinned to an executing Workflow
// plan for this very capability. It still executes the entire Gateway pipeline.
func (g *Gateway) InvokeNative(ctx context.Context, r Request, parent string) (*contracts.Result, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	task, ok := CurrentTaskReference(ctx)
	if !ok {
		return nil, contracts.Problem("forbidden")
	}
	raw, err := dbgen.New(g.pool).ExecutionCheckRun(ctx, dbgen.ExecutionCheckRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: parent, RunID: pgtype.Int8{Int64: task.RunID, Valid: true}})
	if err != nil {
		return nil, contracts.Problem("forbidden")
	}
	entry, ok := g.registry.Resolve(r.Name, r.Version)
	if !ok {
		return nil, contracts.Problem("not_found")
	}
	b, err := executionplan.Decode(raw, entry.Definition)
	if err != nil || b.Plan.Mode != "workflow" {
		return nil, contracts.Problem("forbidden")
	}
	native := contracts.DefaultExecutionBinding(entry.Definition)
	return g.invoke(ctx, r, &native)
}

// InvokePinned selects only a published version in the caller's tenant (or its
// deployment Direct fallback). Clients cannot set this selection in Request.
func (g *Gateway) InvokePinned(ctx context.Context, r Request, id string, version int64) (*contracts.Result, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	entry, ok := g.registry.Resolve(r.Name, r.Version)
	if !ok {
		return nil, contracts.Problem("not_found")
	}
	tenant := p.TenantID
	if id == contracts.DefaultExecutionBinding(entry.Definition).Plan.PlanID {
		tenant = ""
		if err = executionplan.Seed(ctx, dbgen.New(g.pool), entry.Definition); err != nil {
			return nil, err
		}
	}
	plan, err := dbgen.New(g.pool).ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: tenant, PlanID: id, Version: version})
	if err != nil {
		return nil, err
	}
	if plan.Status != "published" || plan.Capability != entry.Definition.Name || plan.CapabilityVersion != entry.Definition.Version || plan.ContractHash != executionplan.ContractHash(entry.Definition) || !executionplan.Executable(plan, entry.Definition) {
		return nil, contracts.Problem("invalid")
	}
	b := contracts.ExecutionBinding{ScopeType: "tenant", ScopeID: p.TenantID, BindingID: "pinned", Plan: executionplan.Plan(plan)}
	return g.invoke(ctx, r, &b)
}

// CompleteWorkflowPlan stores only the original contract's business result.
// Native business events belong to the native invocation and are not emitted twice.
func (g *Gateway) CompleteWorkflowPlan(ctx context.Context, parent, nativeInvocation string) error {
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	task, ok := CurrentTaskReference(ctx)
	if !ok {
		return contracts.Problem("forbidden")
	}
	q := dbgen.New(g.pool)
	raw, err := q.ExecutionCheckRun(ctx, dbgen.ExecutionCheckRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: parent, RunID: pgtype.Int8{Int64: task.RunID, Valid: true}})
	if err != nil {
		return err
	}
	var b contracts.ExecutionBinding
	if err = json.Unmarshal(raw, &b); err != nil {
		return err
	}
	entry, ok := g.registry.Resolve(b.Plan.CapabilityID, b.Plan.CapabilityVersion)
	if !ok {
		return contracts.Problem("unavailable")
	}
	native, err := q.CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: nativeInvocation})
	if err != nil {
		return err
	}
	if native.Status != "succeeded" || native.CausationID != parent || native.Capability != entry.Definition.Name || native.CapabilityVersion != entry.Definition.Version {
		return contracts.Problem("invalid")
	}
	var stored storedResult
	if err = json.Unmarshal(native.Result, &stored); err != nil {
		return err
	}
	data, ok := stored.Data.(map[string]any)
	if !ok {
		return contracts.Problem("invalid")
	}
	if entry.Definition.Output != nil {
		if err = entry.Definition.Output.Validate(contracts.Input(data)); err != nil {
			return err
		}
	}
	stored.Data, err = projectOutput(p, entry.Definition, data)
	if err != nil {
		return err
	}
	stored.ExecutionBinding = &b
	result, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	_, err = q.ExecutionFinishWorkflow(ctx, dbgen.ExecutionFinishWorkflowParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: parent, Status: string(contracts.StatusSucceeded), Result: result})
	return err
}

func (g *Gateway) startWorkflowInvocation(ctx context.Context, p *identity.Principal, entry registry.Entry, request Request, id, trace, digest string, input []byte, resumed bool) (*contracts.Result, error) {
	if g.workflowPlans == nil {
		return nil, contracts.Problem("unavailable")
	}
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	fail := func(cause error) (*contracts.Result, error) {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			return nil, rollbackErr
		}
		if isUnique(cause) && request.IdempotencyKey != "" {
			if replay, found, e := g.replayWait(ctx, p, entry.Definition, request.IdempotencyKey, digest); e == nil && found {
				return replay, nil
			}
		}
		if resumed {
			if e := g.finish(ctx, p.TenantID, id, contracts.StatusFailed, nil, mapCode(cause), 0); e != nil {
				return nil, e
			}
		}
		return nil, cause
	}
	if resumed {
		err = q.CapResumeInvocation(ctx, dbgen.CapResumeInvocationParams{TenantID: p.TenantID, InvocationID: id})
	} else {
		_, err = q.CapCreateInvocation(ctx, dbgen.CapCreateInvocationParams{TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID, InvocationID: id, Capability: entry.Definition.Name, CapabilityVersion: entry.Definition.Version, Origin: string(request.Origin), IdempotencyKey: request.IdempotencyKey, Status: string(contracts.StatusExecuting), Input: input, InputDigest: digest, CorrelationID: trace, CausationID: request.CausationID, TriggerDepth: int32(request.TriggerDepth), TraceID: trace, ExecutionBinding: bindingJSON(request.executionBinding)})
	}
	if err != nil {
		return fail(err)
	}
	if err = g.checkEntry(ctx, q, p, entry, request.Input, true); err != nil {
		return fail(err)
	}
	if request.Runtime != nil {
		if err = g.runtimeCheck(ctx, request.Runtime, entry.Definition, q); err != nil {
			return fail(err)
		}
	}
	ctx = withInvocation(ctx, InvocationContext{Capability: entry.Definition.Name, Version: entry.Definition.Version, InvocationID: id, CorrelationID: trace, CausationID: request.CausationID, Depth: request.TriggerDepth, IdempotencyKey: request.IdempotencyKey, Origin: request.Origin, ExecutionBinding: request.executionBinding, InTransaction: true})
	run, err := g.workflowPlans.StartPlanTx(ctx, tx, request.executionBinding, request.Input)
	if err != nil {
		return fail(err)
	}
	changed, err := q.ExecutionAttachRun(ctx, dbgen.ExecutionAttachRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: id, RunID: pgtype.Int8{Int64: run, Valid: true}})
	if err != nil {
		return fail(err)
	}
	if changed != 1 {
		return fail(contracts.Problem("conflict"))
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &contracts.Result{InvocationID: id, Status: contracts.StatusExecuting, TraceID: trace, EventIDs: []string{}}, nil
}

// CompleteGraphPlan publishes a read-only result from the run bound to this
// invocation. It cannot create business events or finish a write capability.
func (g *Gateway) CompleteGraphPlan(ctx context.Context, parent string, data map[string]any) error {
	p, entry, b, err := g.executionPlanTask(ctx, parent)
	if err != nil {
		return err
	}
	if b.Plan.Mode != "graph" || !entry.Definition.ReadOnly || entry.Definition.TransientResult || entry.Definition.Output == nil {
		return contracts.Problem("invalid")
	}
	q := dbgen.New(g.pool)

	if err = entry.Definition.Output.Validate(contracts.Input(data)); err != nil {
		return err
	}
	projected, err := projectOutput(p, entry.Definition, data)
	if err != nil {
		return err
	}
	result, err := json.Marshal(storedResult{ExecutionBinding: &b, Data: projected, EventIDs: []string{}})
	if err != nil {
		return err
	}
	_, err = q.ExecutionFinishWorkflow(ctx, dbgen.ExecutionFinishWorkflowParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: parent, Status: string(contracts.StatusSucceeded), Result: result})
	return err
}

// CheckExecutionPlanTask revalidates the original capability authority before
// spending on a Graph node. It never grants the run permissions of its own.
func (g *Gateway) CheckExecutionPlanTask(ctx context.Context, parent string) error {
	_, _, _, err := g.executionPlanTask(ctx, parent)
	return err
}
func (g *Gateway) executionPlanTask(ctx context.Context, parent string) (*identity.Principal, registry.Entry, contracts.ExecutionBinding, error) {
	var entry registry.Entry
	var binding contracts.ExecutionBinding
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, entry, binding, err
	}
	if g.identity != nil {
		p, err = g.identity.PrincipalForTask(ctx, p.TenantID, p.Account.ID, p.Account.DepartmentID)
		if err != nil {
			return nil, entry, binding, contracts.Problem("forbidden")
		}
	}
	task, ok := CurrentTaskReference(ctx)
	if !ok {
		return nil, entry, binding, contracts.Problem("forbidden")
	}
	raw, err := dbgen.New(g.pool).ExecutionCheckRun(ctx, dbgen.ExecutionCheckRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: parent, RunID: pgtype.Int8{Int64: task.RunID, Valid: true}})
	if err != nil {
		return nil, entry, binding, err
	}
	if json.Unmarshal(raw, &binding) != nil || (binding.Plan.Mode != "graph" && binding.Plan.Mode != "workflow") {
		return nil, entry, binding, contracts.Problem("invalid")
	}
	entry, ok = g.registry.Resolve(binding.Plan.CapabilityID, binding.Plan.CapabilityVersion)
	if !ok {
		return nil, entry, binding, contracts.Problem("unavailable")
	}
	if entry.Definition.Permission != "" && !p.Can(entry.Definition.Permission) {
		return nil, entry, binding, contracts.Problem("forbidden")
	}
	if err = g.available(ctx, dbgen.New(g.pool), p, entry.Definition, false); err != nil {
		return nil, entry, binding, err
	}
	return p, entry, binding, nil
}

// Package executionplans contributes authorized execution-plan management actions.
package executionplans

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/executionplan"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

type Module struct {
	Pool             *pgxpool.Pool
	Definitions      map[string]contracts.Definition
	ValidateWorkflow func(context.Context, *dbgen.Queries, contracts.Definition, string) (string, error)
	ValidateGraph    func(context.Context, *dbgen.Queries, contracts.Definition, string) (string, error)
	References       func(context.Context, *dbgen.Queries, contracts.Definition, string, string) ([]map[string]any, error)
	Inspect          func(context.Context, *dbgen.Queries, contracts.Definition, string, string, string) (map[string]any, error)
}

func (m *Module) Entries() []registry.Entry {
	text := func() *contracts.Schema {
		min, max := 1, 160
		return &contracts.Schema{Type: "string", MinLength: &min, MaxLength: &max}
	}
	positive := func() *contracts.Schema { min := float64(1); return &contracts.Schema{Type: "integer", Minimum: &min} }
	fields := func() map[string]*contracts.Schema {
		return map[string]*contracts.Schema{"capability": text(), "capabilityVersion": text()}
	}
	required := []string{"capability", "capabilityVersion"}
	var entries []registry.Entry
	add := func(action, permission, entity string, read bool, props map[string]*contracts.Schema, req []string) {
		d := contracts.Definition{Name: "execution.plan." + action, Version: "1.0.0", Title: "Execution plan " + action, Description: "Manage versioned execution plans within the current tenant.", Domain: "executionplans", Entity: entity, Layer: contracts.LayerAtomic, Kind: contracts.KindService, Permission: "executionPlans:" + permission, ReadOnly: read, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(props, req...), Output: outputSchema(action), Executor: "go:executionplans." + action, Tags: []string{"manual-only"}, Loggable: []string{"capability", "capabilityVersion", "planId", "planVersion", "expectedRevision"}}
		h := contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
			return m.read(ctx, in, action)
		}}
		if !read {
			d.Risk = contracts.RiskHigh
			d.Approval = contracts.ApprovalConfirm
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{"execution.plan." + action}
			h = contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.write(ctx, tx, in, action)
			}}
		}
		if action == "bind" || action == "rollback" {
			d.Entities = []string{"execution_binding", "execution_binding_change"}
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: h})
	}
	add("list", "read", "execution_plan", true, fields(), required)
	add("binding", "read", "execution_binding", true, fields(), required)
	add("history", "read", "execution_binding_change", true, fields(), required)
	add("records", "records", "execution_record", true, fields(), nil)
	refs := fields()
	refs["mode"] = &contracts.Schema{Type: "string", Enum: []string{"workflow", "graph"}}
	refs["query"] = &contracts.Schema{Type: "string", MaxLength: ptr(160)}
	add("references", "read", "execution_plan", true, refs, append(append([]string{}, required...), "mode"))
	for _, action := range []string{"inspect", "preflight"} {
		f := fields()
		f["planId"] = text()
		f["planVersion"] = positive()
		permission := "read"
		if action == "preflight" {
			permission = "validate"
			f["input"] = genericObject()
		}
		add(action, permission, "execution_plan", true, f, append(append([]string{}, required...), "planId", "planVersion"))
	}
	add("trace", "records", "execution_record", true, map[string]*contracts.Schema{"invocationId": text()}, []string{"invocationId"})
	f := fields()
	f["planId"] = text()
	f["name"] = text()
	f["mode"] = &contracts.Schema{Type: "string", Enum: []string{"direct", "workflow", "graph"}}
	f["workflowRef"] = text()
	f["graphRef"] = text()
	add("create", "edit", "execution_plan", false, f, append(append([]string{}, required...), "planId", "name"))
	for _, action := range []string{"validate", "publish", "bind", "rollback"} {
		f := fields()
		f["planId"] = text()
		f["planVersion"] = positive()
		req := append(append([]string{}, required...), "planId", "planVersion")
		entity := "execution_plan"
		if action == "bind" || action == "rollback" {
			min := float64(0)
			f["expectedRevision"] = &contracts.Schema{Type: "integer", Minimum: &min}
			f["reason"] = text()
			req = append(req, "expectedRevision", "reason")
			entity = "execution_binding"
		}
		add(action, action, entity, false, f, req)
	}
	return entries
}
func (m *Module) target(p *identity.Principal, in contracts.Input) (contracts.Definition, error) {
	d, ok := m.Definitions[fmt.Sprint(in["capability"])+"@"+fmt.Sprint(in["capabilityVersion"])]
	if !ok {
		return d, contracts.Problem("not_found")
	}
	// Management permission never grants access to an otherwise invisible target.
	if d.Permission != "" && !p.Can(d.Permission) {
		return d, contracts.Problem("forbidden")
	}
	return d, nil
}
func (m *Module) read(ctx context.Context, in contracts.Input, action string) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(m.Pool)
	if action == "records" {
		return m.records(ctx, p, in, q)
	}
	if action == "trace" {
		id, _ := in["invocationId"].(string)
		if _, err := q.ExecutionGetRecord(ctx, dbgen.ExecutionGetRecordParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: id}); err != nil {
			return contracts.Output{}, err
		}
		rows, err := q.ExecutionTrace(ctx, dbgen.ExecutionTraceParams{TenantID: p.TenantID, AccountID: p.Account.ID, InvocationID: id})
		if err != nil {
			return contracts.Output{}, err
		}
		events := []map[string]any{}
		for _, r := range rows {
			events = append(events, map[string]any{"sequence": r.Sequence, "stepName": r.StepName, "eventType": r.EventType, "occurredAt": r.CreatedAt.Time.Format(time.RFC3339Nano)})
		}
		return contracts.Output{Data: map[string]any{"events": events}}, nil
	}
	d, err := m.target(p, in)
	if err != nil {
		return contracts.Output{}, err
	}
	if err = executionplan.Seed(ctx, q, d); err != nil {
		return contracts.Output{}, err
	}
	switch action {
	case "references":
		if m.References == nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		mode, _ := in["mode"].(string)
		query, _ := in["query"].(string)
		rows, err := m.References(ctx, q, d, mode, query)
		return contracts.Output{Data: map[string]any{"references": rows}}, err
	case "inspect", "preflight":
		return m.inspect(ctx, p, in, q, d, action)
	case "binding":
		b, e := executionplan.Resolve(ctx, q, p.TenantID, d)
		return contracts.Output{Data: map[string]any{"binding": b}}, e
	case "history":
		rows, e := q.ExecutionBindingHistory(ctx, dbgen.ExecutionBindingHistoryParams{TenantID: p.TenantID, Capability: d.Name, CapabilityVersion: d.Version})
		return contracts.Output{Data: map[string]any{"changes": rows}}, e

	default:
		rows, e := q.ExecutionListPlans(ctx, dbgen.ExecutionListPlansParams{TenantID: p.TenantID, Capability: d.Name, CapabilityVersion: d.Version})
		out := []map[string]any{}
		for _, row := range rows {
			item := map[string]any{"plan": executionplan.Plan(row), "name": row.Name, "scopeType": scope(row.TenantID), "createdBy": row.CreatedBy, "createdAt": row.CreatedAt.Time.Format(time.RFC3339Nano), "publishedBy": row.PublishedBy}
			if row.PublishedAt.Valid {
				item["publishedAt"] = row.PublishedAt.Time.Format(time.RFC3339Nano)
			}
			out = append(out, item)
		}
		return contracts.Output{Data: map[string]any{"plans": out}}, e
	}
}

var planID = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,119}$`)

func (m *Module) write(ctx context.Context, tx pgx.Tx, in contracts.Input, action string) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	d, err := m.target(p, in)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = executionplan.Seed(ctx, q, d); err != nil {
		return contracts.Output{}, err
	}
	id := fmt.Sprint(in["planId"])
	var version int64
	if action == "create" {
		if !planID.MatchString(id) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		if err = q.ExecutionPlanLock(ctx, p.TenantID+":"+id); err != nil {
			return contracts.Output{}, err
		}
		existing, identityErr := q.ExecutionPlanIdentity(ctx, dbgen.ExecutionPlanIdentityParams{TenantID: p.TenantID, PlanID: id})
		if identityErr != nil && !errors.Is(identityErr, pgx.ErrNoRows) {
			return contracts.Output{}, identityErr
		}
		if identityErr == nil && (existing.Capability != d.Name || existing.CapabilityVersion != d.Version) {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		mode, ref, hash := "direct", d.Key(), ""
		if in["mode"] == "graph" {
			mode = "graph"
			ref, _ = in["graphRef"].(string)
			if _, conflict := in["workflowRef"]; conflict {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			if m.ValidateGraph == nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			hash, err = m.ValidateGraph(ctx, q, d, ref)
			if err != nil {
				return contracts.Output{}, err
			}
		} else if in["mode"] == "workflow" {
			if _, conflict := in["graphRef"]; conflict {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			mode = "workflow"
			ref, _ = in["workflowRef"].(string)
			if m.ValidateWorkflow == nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			hash, err = m.ValidateWorkflow(ctx, q, d, ref)
			if err != nil {
				return contracts.Output{}, err
			}
		} else {
			if _, provided := in["workflowRef"]; provided {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			if _, provided := in["graphRef"]; provided {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		}
		version, err = q.ExecutionCreateDraft(ctx, dbgen.ExecutionCreateDraftParams{TenantID: p.TenantID, PlanID: id, Capability: d.Name, CapabilityVersion: d.Version, Name: fmt.Sprint(in["name"]), Mode: mode, WorkflowHash: hash, ImplementationRef: ref, ContractHash: executionplan.ContractHash(d), CreatedBy: p.Account.ID})
	} else {
		version, err = integer(in["planVersion"])
		if err != nil {
			return contracts.Output{}, err
		}
		tenant := p.TenantID
		if (action == "bind" || action == "rollback") && id == contracts.DefaultExecutionBinding(d).Plan.PlanID {
			tenant = ""
		}
		plan, e := q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: tenant, PlanID: id, Version: version})
		if e != nil {
			return contracts.Output{}, e
		}
		if plan.Capability != d.Name || plan.CapabilityVersion != d.Version || !executionplan.Executable(plan, d) || plan.ContractHash != executionplan.ContractHash(d) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		if action == "bind" || action == "rollback" {
			if err = q.ExecutionPlanLock(ctx, p.TenantID+":execution-bindings"); err != nil {
				return contracts.Output{}, err
			}
		}
		if plan.Mode == "workflow" {
			if m.ValidateWorkflow == nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			hash, e := m.ValidateWorkflow(ctx, q, d, plan.ImplementationRef)
			if e != nil {
				return contracts.Output{}, e
			}
			if hash != plan.WorkflowHash {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		}
		if plan.Mode == "graph" {
			if m.ValidateGraph == nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			hash, e := m.ValidateGraph(ctx, q, d, plan.ImplementationRef)
			if e != nil {
				return contracts.Output{}, e
			}
			if hash != plan.WorkflowHash {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		}
		var changed int64
		switch action {
		case "validate":
			changed, err = q.ExecutionValidatePlan(ctx, dbgen.ExecutionValidatePlanParams{TenantID: p.TenantID, PlanID: id, Version: version, ContractHash: executionplan.ContractHash(d)})
		case "publish":
			changed, err = q.ExecutionPublishPlan(ctx, dbgen.ExecutionPublishPlanParams{TenantID: p.TenantID, PlanID: id, Version: version, PublishedBy: p.Account.ID, ContractHash: executionplan.ContractHash(d)})
		case "bind", "rollback":
			if plan.Status != "published" {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			revision, e := integer(in["expectedRevision"])
			if e != nil {
				return contracts.Output{}, e
			}
			if revision == 0 {
				changed, err = q.ExecutionSetBinding(ctx, dbgen.ExecutionSetBindingParams{TenantID: p.TenantID, Capability: d.Name, CapabilityVersion: d.Version, PlanTenantID: tenant, PlanID: id, PlanVersion: version, UpdatedBy: p.Account.ID, ExpectedRevision: revision})
			} else {
				changed, err = q.ExecutionUpdateBinding(ctx, dbgen.ExecutionUpdateBindingParams{TenantID: p.TenantID, Capability: d.Name, CapabilityVersion: d.Version, PlanTenantID: tenant, PlanID: id, PlanVersion: version, UpdatedBy: p.Account.ID, ExpectedRevision: revision})
			}
			if err == nil && changed == 1 {
				err = q.ExecutionAppendBindingChange(ctx, dbgen.ExecutionAppendBindingChangeParams{TenantID: p.TenantID, Capability: d.Name, CapabilityVersion: d.Version, PlanID: id, PlanVersion: version, Revision: revision + 1, Action: action, Reason: fmt.Sprint(in["reason"]), AccountID: p.Account.ID})
			}
		}
		if err == nil && changed != 1 {
			err = contracts.Problem("conflict")
		}
	}
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"planId": id, "planVersion": version}, Events: []contracts.Event{{Type: "execution.plan." + action, AggregateType: "execution_plan", AggregateID: id, Payload: map[string]any{"capability": d.Name, "planVersion": version}}}}, nil
}
func scope(tenant string) string {
	if tenant == "" {
		return "platform"
	}
	return "tenant"
}
func integer(value any) (int64, error) {
	var n int64
	switch v := value.(type) {
	case int:
		n = int64(v)
	case int64:
		n = v
	case float64:
		if v < 0 || v > 9007199254740991 || float64(int64(v)) != v {
			return 0, contracts.Problem("invalid")
		}
		n = int64(v)
	default:
		return 0, contracts.Problem("invalid")
	}
	if n < 0 {
		return 0, contracts.Problem("invalid")
	}
	return n, nil
}

func ptr(n int) *int { return &n }

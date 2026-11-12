package executionplans

import (
	"context"
	"encoding/json"
	"time"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/executionplan"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func (m *Module) records(ctx context.Context, p *identity.Principal, in contracts.Input, q *dbgen.Queries) (contracts.Output, error) {
	name, _ := in["capability"].(string)
	version, _ := in["capabilityVersion"].(string)
	if name != "" || version != "" {
		if _, err := m.target(p, in); err != nil {
			return contracts.Output{}, err
		}
	}
	rows, err := q.ExecutionListRecords(ctx, dbgen.ExecutionListRecordsParams{TenantID: p.TenantID, AccountID: p.Account.ID, Capability: name, CapabilityVersion: version})
	if err != nil {
		return contracts.Output{}, err
	}
	records := []map[string]any{}
	for _, r := range rows {
		item := map[string]any{"invocationId": r.InvocationID, "binding": json.RawMessage(r.Binding), "status": r.Status, "runStatus": r.RunStatus, "traceId": r.TraceID, "causationId": r.CausationID, "startedAt": r.CreatedAt.Time.Format(time.RFC3339Nano), "errorCode": r.ErrorCode}
		if r.RunID.Valid {
			item["runId"] = r.RunID.Int64
		}
		if r.FinishedAt.Valid {
			item["finishedAt"] = r.FinishedAt.Time.Format(time.RFC3339Nano)
		}
		records = append(records, item)
	}
	return contracts.Output{Data: map[string]any{"records": records}}, nil
}
func (m *Module) inspect(ctx context.Context, p *identity.Principal, in contracts.Input, q *dbgen.Queries, d contracts.Definition, action string) (contracts.Output, error) {
	id, _ := in["planId"].(string)
	version, err := integer(in["planVersion"])
	if err != nil {
		return contracts.Output{}, err
	}
	tenant := p.TenantID
	if id == contracts.DefaultExecutionBinding(d).Plan.PlanID {
		tenant = ""
	}
	plan, err := q.ExecutionGetPlan(ctx, dbgen.ExecutionGetPlanParams{TenantID: tenant, PlanID: id, Version: version})
	if err != nil {
		return contracts.Output{}, err
	}
	if plan.Capability != d.Name || plan.CapabilityVersion != d.Version || plan.ContractHash != executionplan.ContractHash(d) || !executionplan.Executable(plan, d) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if sample, ok := in["input"].(map[string]any); ok {
		if err = d.Input.Validate(contracts.Input(sample)); err != nil {
			return contracts.Output{}, err
		}
	}
	result := map[string]any{"inputSchema": d.Input, "referenceHash": ""}
	if d.Output != nil {
		result["outputSchema"] = d.Output
	}
	if plan.Mode != "direct" {
		if m.Inspect == nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		metadata, e := m.Inspect(ctx, q, d, plan.Mode, plan.ImplementationRef, plan.WorkflowHash)
		if e != nil {
			return contracts.Output{}, e
		}
		for k, v := range metadata {
			result[k] = v
		}
	}
	if action == "preflight" {
		return contracts.Output{Data: map[string]any{"valid": true, "effectsExecuted": false}}, nil
	}
	return contracts.Output{Data: result}, nil
}

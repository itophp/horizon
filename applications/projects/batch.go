package projects

import (
	"context"
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/identity"
	"strconv"
)

func (p Plugin) AttachCapabilities(invoke func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error)) {
	p.Module.Invoke = invoke
}
func (m *Module) batchEntry() registry.Entry {
	max, min := 100, 1
	d := writeBase("project.task.assign.batch", "Assign selected tasks", "Bounded non-atomic task assignment through the Gateway.", "project_task", WritePermission, "project.task.assignment_batch.completed")
	d.Layer = contracts.LayerOrchestration
	d.Input = contracts.Object(map[string]*contracts.Schema{"ids": {Type: "array", Items: projectIDSchema(), MinItems: &min, MaxItems: &max}, "assigneeId": contracts.Int(0, 1<<53), "operationId": contracts.Str(1, 100)}, "ids", "assigneeId", "operationId")
	d.Output = contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "status": {Type: "string"}, "errorCode": {Type: "string"}, "invocationId": {Type: "string"}}, "id", "status", "errorCode", "invocationId")}}, "items")
	return registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
		if m.Invoke == nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		p, err := identity.Current(ctx)
		if err != nil {
			return contracts.Output{}, err
		}
		ids := in["ids"].([]any)
		seen := map[int64]bool{}
		rows := []map[string]any{}
		for _, raw := range ids {
			id := integer(contracts.Input{"id": raw}, "id")
			if seen[id] {
				continue
			}
			seen[id] = true
			row := map[string]any{"id": id, "status": "not_executed", "errorCode": "", "invocationId": ""}
			if ctx.Err() == nil {
				key := contracts.Digest(contracts.Input{"tenant": p.TenantID, "account": p.Account.ID, "operation": in["operationId"], "id": id, "assignee": in["assigneeId"]})
				result, err := m.Invoke(ctx, "project.task.assign", "1.0.0", contracts.Input{"id": id, "assigneeId": in["assigneeId"]}, key)
				if err != nil {
					row["status"] = "failed"
					row["errorCode"] = contracts.Code(err)
					if row["errorCode"] == "" {
						row["errorCode"] = "internal"
					}
				} else {
					row["status"] = string(result.Status)
					row["errorCode"] = result.ErrorCode
					row["invocationId"] = result.InvocationID
				}
			}
			rows = append(rows, row)
		}
		return contracts.Output{Data: map[string]any{"items": rows}, Summary: fmt.Sprintf("items=%d", len(rows)), Events: []contracts.Event{{Type: "project.task.assignment_batch.completed", AggregateType: "project_task", AggregateID: strconv.FormatInt(p.Account.ID, 10), Payload: map[string]any{"count": len(rows)}}}}, nil
	}}}
}

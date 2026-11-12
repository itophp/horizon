package orchestration

import (
	"fmt"
	"strconv"
	"time"

	canonical "horizon/foundation/kernel/memory"
	"horizon/internal/repository/dbgen"
)

// ObservationSink is infrastructure telemetry; it must never execute a
// business capability or delay the workflow.
type ObservationSink interface{ Submit(canonical.Envelope) }

func (e *Engine) observeStep(state execution, row dbgen.AIGetRunRow, step Step, iteration int, phase string, occurred time.Time) {
	kind := "capability"
	if step.Graph != nil {
		kind = "graph"
	}
	if step.Agent != nil {
		kind = "agent"
	}
	key := fmt.Sprintf("wf:%d:%s:%d:%s", state.ID, step.ID, iteration, phase)
	event := canonical.Envelope{EventID: key, EventType: "workflow.node." + phase, TenantID: state.Tenant, OccurredAt: occurred,
		Actor: canonical.Actor{ID: strconv.FormatInt(state.Account, 10), Type: "user"}, DepartmentID: row.DepartmentID,
		Source: canonical.Source{Type: "workflow", ID: step.ID}, Resource: canonical.Resource{Type: "ai_run", ID: strconv.FormatInt(state.ID, 10)},
		TaskID: strconv.FormatInt(state.ID, 10), WorkflowInstanceID: strconv.FormatInt(state.ID, 10),
		TraceID: row.TraceID, CorrelationID: row.CorrelationID, CausationID: row.CausationID, Depth: int(row.TriggerDepth), Sensitivity: "internal",
		Payload: map[string]string{"kind": kind}}
	e.Observations.Submit(event)
	if phase == "failed" {
		event.EventID = "exception:" + key
		event.EventType = "system.exception.detected"
		event.Payload = map[string]string{"kind": "workflow_node"}
		e.Observations.Submit(event)
	}
}

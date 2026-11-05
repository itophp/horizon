package orchestration

import (
	"testing"
	"time"

	"baseline/services/api/internal/repository/dbgen"
	canonical "baseline/services/api/kernel/memory"
)

type capturedObservations struct{ events []canonical.Envelope }

func (c *capturedObservations) Submit(e canonical.Envelope) { c.events = append(c.events, e) }

func TestWorkflowFailureObservationExcludesInputsAndErrors(t *testing.T) {
	sink := &capturedObservations{}
	e := &Engine{Observations: sink}
	state := execution{ID: 42, Tenant: "tenant", Account: 7}
	row := dbgen.AIGetRunRow{DepartmentID: 9, TraceID: "trace", CorrelationID: "correlation", Question: "secret question", Answer: "secret answer"}
	e.observeStep(state, row, Step{ID: "review"}, 1, "failed", time.Now())
	if len(sink.events) != 2 || sink.events[0].EventType != "workflow.node.failed" || sink.events[1].EventType != "system.exception.detected" {
		t.Fatalf("unexpected observations: %#v", sink.events)
	}
	if sink.events[0].Payload["kind"] != "capability" || sink.events[0].Source.ID != "review" || sink.events[0].Resource.ID != "42" {
		t.Fatalf("lost node context: %#v", sink.events[0])
	}
	if sink.events[1].Payload["kind"] != "workflow_node" {
		t.Fatalf("unexpected exception: %#v", sink.events[1])
	}
}

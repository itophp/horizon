package orchestration

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
	"context"
	"testing"
)

func testCatalog() *registry.Registry {
	entries := []registry.Entry{}
	for _, name := range []string{"read", "write"} {
		read := name == "read"
		d := contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1", Entity: "order", ReadOnly: read, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil)}
		if !read {
			d.Idempotency.Required = true
			d.SuccessEvents = []string{"order.updated"}
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) { return contracts.Output{}, nil }}})
	}
	return registry.New(entries)
}
func TestExplicitOrderAndResourceConflicts(t *testing.T) {
	r := testCatalog()
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	d := Definition{Name: "test", Version: "1", Input: contracts.Object(nil), Steps: []Step{{ID: "check", Capability: "read", Version: "1"}, {ID: "update", Capability: "write", Version: "1"}}}
	if d.Validate(r) == nil {
		t.Fatal("ambiguous read/write order accepted")
	}
	d.Steps[1].DependsOn = []string{"check"}
	if err := d.Validate(r); err != nil {
		t.Fatal(err)
	}
	d.Steps[0].DependsOn = []string{"update"}
	if d.Validate(r) == nil {
		t.Fatal("cycle accepted")
	}
	d.Steps[0].DependsOn = nil
	d.Steps[1].DependsOn = nil
	d.Steps[1].Input = map[string]Mapping{"value": {From: "check", Field: "value"}}
	if err := d.Validate(r); err != nil {
		t.Fatalf("mapping did not establish dependency: %v", err)
	}
}
func TestPollAndAgentBounds(t *testing.T) {
	r := testCatalog()
	d := Definition{Name: "test", Version: "1", Input: contracts.Object(nil), Steps: []Step{{ID: "poll", Capability: "write", Version: "1", Poll: &Poll{MaxSteps: 3, DoneField: "done"}}}}
	if d.Validate(r) == nil {
		t.Fatal("write loop accepted")
	}
	d.Steps[0].Capability = "read"
	if err := d.Validate(r); err != nil {
		t.Fatal(err)
	}
	d.Steps[0].Poll.MaxSteps = 0
	if d.Validate(r) == nil {
		t.Fatal("unbounded loop accepted")
	}
	d.Steps = []Step{{ID: "agent", Agent: &Agent{Tools: []string{"read@1"}, Instruction: "inspect", MaxIterations: 0}}}
	if d.Validate(r) == nil {
		t.Fatal("unbounded agent accepted")
	}
}

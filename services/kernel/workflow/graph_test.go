package orchestration

import (
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/workflow/spec"
	"strings"
	"testing"
)

func TestGraphReferencesAndReadOnlyContracts(t *testing.T) {
	catalog := testCatalog()
	graph := spec.BuiltinGraphs()[0]
	graph.Reads = []spec.GraphRead{{Capability: "read", Version: "1"}}
	graph.Input = contracts.Object(nil)
	flow := Definition{Name: "test", Version: "1", Input: contracts.Object(nil), Steps: []Step{{ID: "risk", Graph: graph.Reference()}}}
	if err := flow.ValidateWithGraphs(catalog, []spec.GraphDefinition{graph}); err != nil {
		t.Fatal(err)
	}
	for _, modify := range []func(*spec.GraphDefinition){
		func(g *spec.GraphDefinition) { g.Reads[0].Capability = "write" },
		func(g *spec.GraphDefinition) { g.MaxModelCalls = 0 },
		func(g *spec.GraphDefinition) { g.MaxModelCalls = 9 },
		func(g *spec.GraphDefinition) { g.TimeoutMillis = 30001 },
		func(g *spec.GraphDefinition) { g.MaxToolCalls = 0 },
		func(g *spec.GraphDefinition) { g.MaxOutputTokens = 2049 },
		func(g *spec.GraphDefinition) { g.Output = contracts.Object(nil) },
	} {
		bad := graph
		bad.Reads = append([]spec.GraphRead{}, graph.Reads...)
		modify(&bad)
		if err := bad.Validate(catalog); err == nil {
			t.Fatal("unsafe graph accepted", bad)
		}
	}
	flow.Steps[0].Graph.Hash = strings.Repeat("0", 64)
	if err := flow.ValidateWithGraphs(catalog, []spec.GraphDefinition{graph}); err == nil {
		t.Fatal("changed graph reference accepted")
	}
	flow.Steps[0].Graph = graph.Reference()
	flow.Steps[0].Capability = "read"
	if err := flow.ValidateWithGraphs(catalog, []spec.GraphDefinition{graph}); err == nil {
		t.Fatal("ambiguous graph executor accepted")
	}
}

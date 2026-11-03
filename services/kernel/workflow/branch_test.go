package orchestration

import (
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/governance"
	"context"
	"testing"
)

// branchCatalog declares a read, a write and a step that reports a verdict, so
// branch rules can be checked without a database or a model.
func branchCatalog() *registry.Registry {
	entries := []registry.Entry{}
	for _, name := range []string{"read", "write", "verdict"} {
		output := contracts.Object(nil)
		if name == "verdict" {
			output = contracts.Object(map[string]*contracts.Schema{"decision": {Type: "string", Enum: []string{"approve", "reject"}}}, "decision")
		}
		read := name == "read"
		definition := contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1", Entity: "order", ReadOnly: read, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil), Output: output}
		if !read {
			definition.Idempotency.Required = true
			definition.SuccessEvents = []string{"order.updated"}
		}
		entries = append(entries, registry.Entry{Definition: definition, Handler: contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) { return contracts.Output{}, nil }}})
	}
	return registry.New(entries)
}

func branchDefinition() Definition {
	return Definition{Name: "branch", Version: "1", Input: contracts.Object(nil), Steps: []Step{
		{ID: "check", Capability: "verdict", Version: "1"},
		{ID: "approve", Capability: "write", Version: "1", When: &When{From: "check", Field: "decision", Equals: "approve"}},
		{ID: "deny", Capability: "write", Version: "1", When: &When{From: "check", Field: "decision", Equals: "reject"}},
	}}
}

func TestBranchAlternativesDoNotNeedOrdering(t *testing.T) {
	// Both alternatives write the same entity. Ordering them would invent a
	// sequence the flow never has, so mutual exclusion must satisfy the check.
	if err := branchDefinition().Validate(branchCatalog()); err != nil {
		t.Fatal(err)
	}
}

func TestBranchRejectsAmbiguousOrUnknownCases(t *testing.T) {
	catalog := branchCatalog()
	d := branchDefinition()
	d.Steps[2].When.Equals = "approve"
	if d.Validate(catalog) == nil {
		t.Fatal("duplicate branch case accepted")
	}
	d = branchDefinition()
	d.Steps[1].When.From = "missing"
	if d.Validate(catalog) == nil {
		t.Fatal("unknown branch source accepted")
	}
	d = branchDefinition()
	d.Steps[1].When.Field = "missing"
	if d.Validate(catalog) == nil {
		t.Fatal("undeclared branch field accepted")
	}
	d = branchDefinition()
	d.Steps[1].When.Equals = 1
	if d.Validate(catalog) == nil {
		t.Fatal("branch case type mismatch accepted")
	}
}

func TestBranchOnAgentRequiresDeclaredOutput(t *testing.T) {
	d := Definition{Name: "branch", Version: "1", Input: contracts.Object(nil), Steps: []Step{
		{ID: "review", Agent: &Agent{Instruction: "review", Tools: []string{"read@1"}, MaxIterations: 1}},
		{ID: "approve", Capability: "write", Version: "1", When: &When{From: "review", Field: "answer", Equals: "ok"}},
	}}
	if d.Validate(branchCatalog()) == nil {
		t.Fatal("branch on unstructured agent answer accepted")
	}
	d.Steps[0].Agent.Output = contracts.Object(map[string]*contracts.Schema{"answer": {Type: "string"}}, "answer")
	if err := d.Validate(branchCatalog()); err != nil {
		t.Fatal(err)
	}
}

func TestCompiledBranchIsASingleChoice(t *testing.T) {
	d := branchDefinition()
	engine := New(&governance.Core{}, []Definition{d})
	if err := engine.Attach(nil, branchCatalog()); err != nil {
		t.Fatal(err)
	}
	diagram, err := engine.Diagram(context.Background(), "branch", "1")
	if err != nil {
		t.Fatal(err)
	}
	branches := map[string]string{}
	for _, edge := range diagram.Edges {
		if edge.Kind == "branch" {
			branches[edge.From+"->"+edge.To] = edge.Kind
		}
	}
	if len(branches) != 2 || branches["check->approve"] != "branch" || branches["check->deny"] != "branch" {
		t.Fatalf("compiled branch lost: %+v", diagram.Edges)
	}
	for _, node := range diagram.Nodes {
		if node.ID == "check" || node.ID == "start" || node.ID == "end" {
			continue
		}
		if !node.Compiled {
			t.Fatalf("branch node %s missing from the compiled graph", node.ID)
		}
	}
	warned := false
	for _, node := range diagram.Nodes {
		if node.ID == "approve" && node.When != nil && node.When.Equals == "approve" {
			warned = true
		}
	}
	if !warned {
		t.Fatal("diagram must expose the branch condition")
	}
}

func TestStructuredAgentOutput(t *testing.T) {
	output := contracts.Object(map[string]*contracts.Schema{
		"decision": {Type: "string", Enum: []string{"approve", "reject"}},
		"code":     {Type: "string", Enum: []string{"safe", "blocked", "suspected"}},
	}, "decision", "code")
	for _, test := range []struct {
		answer string
		ok     bool
	}{
		{`{"decision":"approve","code":"safe"}`, true},
		{"```json\n{\"decision\":\"reject\",\"code\":\"blocked\"}\n```", true},
		// Models wrap the verdict in prose; the object inside is still the answer.
		{`The article is fine. {"decision":"approve","code":"safe"}`, true},
		// An added explanation field is dropped, not treated as a wrong answer.
		{`{"decision":"approve","code":"safe","reason":"looks fine"}`, true},
		// The last object wins over an example quoted earlier.
		{`For example {"decision":"reject","code":"blocked"}; verdict: {"decision":"approve","code":"safe"}`, true},
		{`{"decision":"maybe","code":"safe"}`, false},
		{`{"decision":"approve"}`, false},
		{`{"decision":"Approve","code":"safe"}`, false},
		{`{"decision":"approve","code":"safe"`, false},
		{`approved`, false},
		{``, false},
	} {
		got, err := structured(output, test.answer)
		if (err == nil) != test.ok {
			t.Fatalf("answer %q ok=%v err=%v", test.answer, test.ok, err)
		}
		if test.ok && (got["decision"] == nil || got["code"] == nil) {
			t.Fatalf("answer %q lost a declared field: %+v", test.answer, got)
		}
	}
}

func TestShapeDescribesDeclaredFields(t *testing.T) {
	output := contracts.Object(map[string]*contracts.Schema{
		"code": {Type: "string", Enum: []string{"safe", "blocked", "suspected"}},
	}, "code")
	if got := shape(output); got != `{"code":"safe"|"blocked"|"suspected"}` {
		t.Fatalf("shape = %s", got)
	}
}

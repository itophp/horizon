package metadata

import (
	"strings"
	"testing"

	"baseline/services/api/internal/platform/capability/contracts"
)

func stateEntity() contracts.Entity {
	return contracts.Entity{
		Name: "widget", Table: "widgets", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"draft", "done"}},
		},
		States: []contracts.State{{Name: "draft"}, {Name: "done", Terminal: true}},
		Transitions: []contracts.Transition{
			{From: []string{"draft"}, To: "done", Capability: "widget.finish"},
		},
	}
}

func writeCapability(name, entity string, input *contracts.Schema) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: "1.0.0", Domain: "widget", Entity: entity,
		Input: input, Executor: "go:" + name,
	}
}

func transitionIssues(issues []Issue) string {
	var out []string
	for _, issue := range issues {
		if issue.Code == "E8" {
			out = append(out, string(issue.Severity)+": "+issue.Message)
		}
	}
	return strings.Join(out, " | ")
}

func TestTransitionBoundToCapabilityPasses(t *testing.T) {
	definitions := []contracts.Definition{
		writeCapability("widget.finish", "widget", contracts.Object(map[string]*contracts.Schema{
			"id": contracts.Int(1, 1<<40),
		}, "id")),
	}
	if got := transitionIssues(ValidateTransitions(definitions, []contracts.Entity{stateEntity()})); got != "" {
		t.Fatalf("expected no transition issues, got %s", got)
	}
}

func TestTransitionNamingUnknownCapabilityFails(t *testing.T) {
	got := transitionIssues(ValidateTransitions(nil, []contracts.Entity{stateEntity()}))
	if !strings.Contains(got, "unknown capability widget.finish") {
		t.Fatalf("expected unknown capability error, got %q", got)
	}
}

func TestTransitionOnForeignEntityFails(t *testing.T) {
	definitions := []contracts.Definition{
		writeCapability("widget.finish", "gadget", contracts.Object(map[string]*contracts.Schema{
			"id": contracts.Int(1, 1<<40),
		}, "id")),
	}
	got := transitionIssues(ValidateTransitions(definitions, []contracts.Entity{stateEntity()}))
	if !strings.Contains(got, "does not declare entity widget") {
		t.Fatalf("expected entity mismatch, got %q", got)
	}
}

func TestCapabilityThatSetsStateMustBeBound(t *testing.T) {
	entity := stateEntity()
	entity.Transitions = nil
	entity.States = nil
	// Without a graph the entity is not a state machine, so nothing is expected.
	if got := transitionIssues(ValidateTransitions(nil, []contracts.Entity{entity})); got != "" {
		t.Fatalf("entity without states must be skipped, got %s", got)
	}

	machine := stateEntity()
	machine.Transitions = nil
	definitions := []contracts.Definition{
		writeCapability("widget.setStatus", "widget", contracts.Object(map[string]*contracts.Schema{
			"status": {Type: "string", Enum: []string{"draft", "done"}},
		}, "status")),
	}
	got := transitionIssues(ValidateTransitions(definitions, []contracts.Entity{machine}))
	if !strings.Contains(got, "accepts") || !strings.Contains(got, "no transition binds it") {
		t.Fatalf("expected unbound state write, got %q", got)
	}
}

func TestUnboundStateWriteIsAllowedWhenBound(t *testing.T) {
	machine := stateEntity()
	machine.Transitions = []contracts.Transition{{From: []string{"draft"}, To: "done", Capability: "widget.setStatus"}}
	definitions := []contracts.Definition{
		writeCapability("widget.setStatus", "widget", contracts.Object(map[string]*contracts.Schema{
			"status": {Type: "string", Enum: []string{"draft", "done"}},
		}, "status")),
	}
	if got := transitionIssues(ValidateTransitions(definitions, []contracts.Entity{machine})); got != "" {
		t.Fatalf("bound write must pass, got %s", got)
	}
}

func TestReadOnlyCapabilityDoesNotNeedTransition(t *testing.T) {
	machine := stateEntity()
	machine.Transitions = nil
	read := writeCapability("widget.list", "widget", contracts.Object(map[string]*contracts.Schema{
		"status": {Type: "string", Enum: []string{"draft", "done"}},
	}, "status"))
	read.ReadOnly = true
	if got := transitionIssues(ValidateTransitions([]contracts.Definition{read}, []contracts.Entity{machine})); got != "" {
		t.Fatalf("read only filter is not a state write, got %s", got)
	}
}

func TestTransitionWithoutCapabilityIsHint(t *testing.T) {
	machine := stateEntity()
	machine.Transitions = []contracts.Transition{{From: []string{"draft"}, To: "done"}}
	issues := ValidateTransitions(nil, []contracts.Entity{machine})
	if len(issues) != 1 {
		t.Fatalf("expected one hint, got %v", issues)
	}
	if issues[0].Severity != SeverityWarning || !strings.Contains(issues[0].Message, "background code") {
		t.Fatalf("worker driven transition should be a hint, got %+v", issues[0])
	}
	if len(Errors(issues)) != 0 {
		t.Fatal("hint must not block the check")
	}
}

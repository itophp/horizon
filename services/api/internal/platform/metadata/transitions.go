package metadata

import (
	"fmt"
	"sort"
	"strings"

	"baseline/services/api/internal/platform/capability/contracts"
)

// ValidateTransitions checks the declared state graph against the capability
// catalog. A state machine is only useful if every move has one controlled
// entry, so both directions are checked:
//
//   - every transition names a capability that exists and touches the entity;
//   - every write capability that accepts a state attribute in its input is
//     bound to some transition, otherwise a caller could set a state without
//     the graph knowing about it.
//
// A transition with no capability is reported as a hint: background workers
// legitimately move states, and pretending they are capability calls would be
// worse than saying so.
func ValidateTransitions(definitions []contracts.Definition, entities []contracts.Entity) []Issue {
	byName := make(map[string]contracts.Definition, len(definitions))
	for _, definition := range definitions {
		byName[definition.Name] = definition
	}
	var issues []Issue
	for _, entity := range entities {
		if len(entity.Transitions) == 0 && len(entity.States) == 0 {
			continue
		}
		bound := map[string]bool{}
		for _, transition := range entity.Transitions {
			if transition.Capability == "" {
				issues = append(issues, Issue{
					Code: "E8", Entity: entity.Name, Severity: SeverityWarning,
					Message: fmt.Sprintf("transition %s->%s has no capability; only background code can perform it",
						describeFrom(transition.From), transition.To),
				})
				continue
			}
			bound[transition.Capability] = true
			definition, ok := byName[transition.Capability]
			if !ok {
				issues = append(issues, Issue{
					Code: "E8", Entity: entity.Name, Severity: SeverityError,
					Message: fmt.Sprintf("transition %s->%s names unknown capability %s",
						describeFrom(transition.From), transition.To, transition.Capability),
				})
				continue
			}
			if !touches(definition, entity.Name) {
				issues = append(issues, Issue{
					Code: "E8", Entity: entity.Name, Severity: SeverityError,
					Message: fmt.Sprintf("capability %s does not declare entity %s",
						transition.Capability, entity.Name),
				})
			}
		}
		for _, definition := range definitions {
			if definition.ReadOnly || !touches(definition, entity.Name) {
				continue
			}
			attribute, writes := stateInput(entity, definition.Input)
			if !writes || bound[definition.Name] {
				continue
			}
			issues = append(issues, Issue{
				Code: "E8", Entity: entity.Name, Severity: SeverityError,
				Message: fmt.Sprintf("capability %s accepts %q but no transition binds it",
					definition.Name, attribute),
			})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Entity == issues[j].Entity {
			return issues[i].Message < issues[j].Message
		}
		return issues[i].Entity < issues[j].Entity
	})
	return issues
}

// stateInput reports the state attribute a capability accepts as input. A
// capability that takes a state field can move the entity to any of those
// values, so it must appear in the graph.
func stateInput(entity contracts.Entity, input *contracts.Schema) (string, bool) {
	if input == nil || len(input.Properties) == 0 {
		return "", false
	}
	for _, attribute := range entity.Attributes {
		if attribute.Kind != contracts.AttributeState {
			continue
		}
		for _, key := range []string{attribute.Name, attribute.DatabaseColumn()} {
			if _, ok := input.Properties[key]; ok {
				return attribute.Name, true
			}
		}
	}
	return "", false
}

func touches(definition contracts.Definition, entity string) bool {
	for _, name := range definition.EntityNames() {
		if name == entity {
			return true
		}
	}
	return false
}

func describeFrom(from []string) string {
	if len(from) == 0 {
		return "new"
	}
	return strings.Join(from, "|")
}

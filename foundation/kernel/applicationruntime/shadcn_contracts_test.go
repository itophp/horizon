package applicationruntime

import (
	"encoding/json"
	"testing"
)

func TestShadcnContractExamples(t *testing.T) {
	for _, item := range UIContracts() {
		if _, ok := shadcnContracts[item.Name]; !ok || item.Category != "element" {
			continue
		}
		t.Run(item.Name, func(t *testing.T) {
			var node Node
			if err := json.Unmarshal([]byte(item.Example), &node); err != nil {
				t.Fatal(err)
			}
			if node.Type != "component" || node.Component == nil {
				t.Fatal("missing component example")
			}
			if err := validateComponent(node.Component, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestShadcnValidationRejectsUnsupportedConfiguration(t *testing.T) {
	for _, component := range []ComponentSpec{
		{Kind: "checkbox", Props: map[string]any{"labelKey": "common:enabled", "value": "true"}},
		{Kind: "input", Props: map[string]any{"labelKey": "common:enabled", "onChange": "script"}},
		{Kind: "badge", Props: map[string]any{"labelKey": "common:enabled"}, Events: []EventBinding{{Event: "click", Action: "invoke", Target: "test"}}},
		{Kind: "input-otp", Props: map[string]any{"labelKey": "common:enabled"}, Events: []EventBinding{{Event: "complete", Action: "setData", Target: "_code"}}},
		{Kind: "input", Props: map[string]any{"labelKey": "common:enabled"}, Bindings: []Mapping{{Target: "onChange", Value: Expression{Kind: "literal", Literal: "true"}}}},
	} {
		component.ID, component.Version = "fixture", "1.0"
		if err := validateComponent(&component, nil); err == nil {
			t.Errorf("accepted invalid %s configuration", component.Kind)
		}
	}
}

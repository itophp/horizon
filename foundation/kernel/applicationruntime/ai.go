package applicationruntime

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"horizon/foundation/kernel/capability/contracts"
)

const (
	AnalyzeCapability = "application.runtime.ai.analyze"
	SuggestCapability = "application.runtime.ai.suggest"
)

type AIDataInput struct {
	Source string   `json:"source"`
	Fields []string `json:"fields"`
}

type AIAnalysisSpec struct {
	Action string        `json:"action"`
	Prompt string        `json:"prompt"`
	Input  []AIDataInput `json:"input"`
}

type AIColumn struct {
	Name     string         `json:"name"`
	LabelKey string         `json:"labelKey"`
	Display  string         `json:"display"`
	Analysis AIAnalysisSpec `json:"analysis"`
}

type AIScenario struct {
	ID       string         `json:"id"`
	LabelKey string         `json:"labelKey"`
	Analysis AIAnalysisSpec `json:"analysis"`
}

type AIAnalysisPanelProps struct {
	Scenarios []AIScenario `json:"scenarios"`
}

type AIResultProps struct {
	Variant string `json:"variant,omitempty"`
}

type AIFormAssistant struct {
	LabelKey     string   `json:"labelKey"`
	Action       string   `json:"action"`
	InputFields  []string `json:"inputFields"`
	TargetFields []string `json:"targetFields"`
	Prompt       string   `json:"prompt"`
	CreateScope  string   `json:"createScope,omitempty"`
	EditScope    string   `json:"editScope,omitempty"`
}

type AIResult struct {
	Status   string      `json:"status"`
	Summary  string      `json:"summary,omitempty"`
	Label    string      `json:"label,omitempty"`
	Evidence []string    `json:"evidence,omitempty"`
	Coverage *AICoverage `json:"coverage,omitempty"`
}

type AICoverage struct {
	Analyzed int `json:"analyzed"`
	Total    int `json:"total"`
}

type AIFieldSuggestion struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type AISuggestions struct {
	Suggestions []AIFieldSuggestion `json:"suggestions"`
}

func decodeAI[T any](value any) (T, error) {
	var out T
	raw, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return out, decoder.Decode(&out)
}

func AIAnalyses(component *ComponentSpec) ([]AIScenario, error) {
	switch component.Kind {
	case "ai-analysis-panel":
		props, err := decodeAI[AIAnalysisPanelProps](component.Props)
		if err != nil || len(props.Scenarios) < 1 || len(props.Scenarios) > 3 || !strings.HasPrefix(component.Data, "$") {
			return nil, fmt.Errorf("invalid AI analysis panel")
		}
		return props.Scenarios, nil
	case "table-panel":
		raw, err := json.Marshal(component.Props["columns"])
		if err != nil {
			return nil, err
		}
		var columns []map[string]any
		if err := json.Unmarshal(raw, &columns); err != nil {
			return nil, err
		}
		var scenarios []AIScenario
		for _, column := range columns {
			if column["display"] != "ai" {
				continue
			}
			col, err := decodeAI[AIColumn](column)
			if err != nil {
				return nil, err
			}
			scenarios = append(scenarios, AIScenario{ID: col.Name, LabelKey: col.LabelKey, Analysis: col.Analysis})
		}
		return scenarios, nil
	}
	return nil, nil
}

func AIAction(component *ComponentSpec, event, capability string) bool {
	count := 0
	for _, binding := range component.Events {
		if binding.Event == event {
			if binding.Action != "invoke" || binding.Target != capability+"@1.0.0" || binding.When != nil ||
				binding.Payload.Kind != "" || binding.Reauthenticate != nil || binding.Disclosure != nil ||
				binding.Download != nil || binding.ResultTarget != "" || binding.Result != nil {
				return false
			}
			count++
		}
	}
	return event != "" && count == 1
}

// AIForms returns the form declarations supported by AI-enabled components.
func AIForms(component *ComponentSpec) map[string]FormV2 {
	forms := map[string]FormV2{}
	if component.Kind != "table-panel" && component.Kind != "detail-panel" && component.Kind != "record-form-panel" {
		return forms
	}
	if raw, err := json.Marshal(component.Props["forms"]); err == nil {
		_ = json.Unmarshal(raw, &forms)
	}
	if forms == nil {
		forms = map[string]FormV2{}
	}
	if raw, err := json.Marshal(component.Props["form"]); err == nil {
		var form FormV2
		if json.Unmarshal(raw, &form) == nil && form.ID != "" {
			forms["form"] = form
		}
	}
	return forms
}

func aiFieldsValid(fields []string) bool {
	seen := map[string]bool{}
	if len(fields) < 1 || len(fields) > 12 {
		return false
	}
	for _, field := range fields {
		if field == "" || seen[field] || strings.Contains(field, ".") ||
			slices.Contains([]string{"__proto__", "constructor", "prototype"}, field) {
			return false
		}
		seen[field] = true
	}
	return true
}

func AIPromptValid(prompt string) bool {
	return strings.TrimSpace(prompt) != "" && len(prompt) <= 4096
}

func AIFormField(command Command, name string, target bool) (FieldV2, bool) {
	for _, field := range command.Fields {
		if field.Name != name || field.Hidden || field.Sensitive || field.Derived ||
			field.Fixed || field.Multiple || field.VisibleWhen.Kind != "" || field.DisabledWhen.Kind != "" {
			continue
		}
		if target && (field.Type != "string" || field.ReadOnly) {
			continue
		}
		if !slices.Contains([]string{"string", "enum", "date", "integer", "number", "boolean"}, field.Type) {
			continue
		}
		if slices.ContainsFunc(command.Input, func(mapping Mapping) bool {
			return mapping.Value.Kind == "path" && NormalizePath(mapping.Value.Path) == "form."+name
		}) {
			return field, true
		}
	}
	return FieldV2{}, false
}

// FindAIComponent resolves a component and its inherited data bindings. IDs
// must be unique for a component to be addressed by a Runtime AI action.
func FindAIComponent(root Node, id string) (*ComponentSpec, map[string]DataSpec) {
	var found *ComponentSpec
	var bindings map[string]DataSpec
	count := 0
	var walk func(*LayoutSpec, map[string]DataSpec)
	walk = func(layout *LayoutSpec, inherited map[string]DataSpec) {
		if layout == nil {
			return
		}
		scope := make(map[string]DataSpec, len(inherited)+len(layout.Data))
		for k, v := range inherited {
			scope[k] = v
		}
		for _, binding := range layout.Data {
			scope["$"+binding.ID] = binding
		}
		for _, child := range layout.Children {
			walk(child.Layout, scope)
			if child.Component != nil {
				if child.Component.ID == id {
					found, bindings = child.Component, scope
					count++
				}
				walk(child.Component.Content, scope)
			}
		}
	}
	walk(root.Layout, nil)
	if count != 1 {
		return nil, nil
	}
	return found, bindings
}

func validateAIPage(root Node, providers map[string]*Provider, definitions map[string]contracts.Definition) error {
	var visit func(*LayoutSpec) error
	visit = func(layout *LayoutSpec) error {
		if layout == nil {
			return nil
		}
		for _, child := range layout.Children {
			if err := visit(child.Layout); err != nil {
				return err
			}
			if component := child.Component; component != nil {
				if err := visit(component.Content); err != nil {
					return err
				}
				if err := validateAIComponent(root, component, providers, definitions); err != nil {
					return fmt.Errorf("component %s: %w", component.ID, err)
				}
			}
		}
		return nil
	}
	return visit(root.Layout)
}

func validateAIComponent(root Node, component *ComponentSpec, providers map[string]*Provider, definitions map[string]contracts.Definition) error {
	if component.Kind == "ai-result" {
		props, err := decodeAI[AIResultProps](component.Props)
		if err != nil || (props.Variant != "" && props.Variant != "compact" && props.Variant != "detail") {
			return fmt.Errorf("invalid AI result variant")
		}
	}
	scenarios, err := AIAnalyses(component)
	if err != nil {
		return err
	}
	forms := AIForms(component)
	hasAssistant := false
	for _, form := range forms {
		hasAssistant = hasAssistant || form.AIAssistant != nil
	}
	if len(scenarios) == 0 && !hasAssistant {
		return nil
	}
	resolved, bindings := FindAIComponent(root, component.ID)
	output := func(source string, related bool) *contracts.Schema {
		provider := providers[bindings[source].Provider]
		if provider == nil || provider.Capability.Name == "" {
			return nil
		}
		def := definitions[provider.Capability.Name+"@"+provider.Capability.Version]
		if !def.ReadOnly || def.Output == nil || def.Input == nil {
			return nil
		}
		mappings := bindings[source].Input
		if len(mappings) == 0 {
			mappings = provider.Input
		}
		foundRecord := false
		for _, mapping := range mappings {
			if mapping.Value.Kind == "path" && NormalizePath(mapping.Value.Path) == "route.id" {
				target := def.Input.Properties[mapping.Target]
				foundRecord = target != nil && target.Type == "integer"
			} else if slices.Contains(def.Input.Required, mapping.Target) && mapping.Value.Kind != "literal" {
				return nil
			}
		}
		if !foundRecord {
			return nil
		}
		projected, err := providerSchema(def.Output, *provider)
		if err != nil {
			return nil
		}
		if !related {
			return projected
		}
		if len(provider.Select) > 0 && provider.ResultPath != "items" {
			return nil
		}
		items := projected.Properties["items"]
		if items == nil || def.Output.Properties["total"] == nil || def.Input == nil ||
			def.Input.Properties["page"] == nil || def.Input.Properties["pageSize"] == nil {
			return nil
		}
		return items.Items
	}
	ids, events := map[string]bool{}, map[string]bool{}
	for _, scene := range scenarios {
		analysis := scene.Analysis
		if resolved == nil || scene.ID == "" || scene.LabelKey == "" || ids[scene.ID] ||
			events[analysis.Action] || !AIPromptValid(analysis.Prompt) || !AIAction(component, analysis.Action, AnalyzeCapability) ||
			len(analysis.Input) < 1 || len(analysis.Input) > 2 ||
			(component.Kind == "table-panel" && len(analysis.Input) != 1) ||
			(component.Kind == "ai-analysis-panel" && analysis.Input[0].Source != component.Data) {
			return fmt.Errorf("invalid AI analysis scene")
		}
		ids[scene.ID], events[analysis.Action] = true, true
		for i, input := range analysis.Input {
			schema := output(input.Source, i > 0)
			if schema == nil || !aiFieldsValid(input.Fields) {
				return fmt.Errorf("invalid AI input provider")
			}
			for _, field := range input.Fields {
				property := schema.Properties[field]
				if property == nil || !slices.Contains([]string{"string", "number", "integer", "boolean"}, property.Type) {
					return fmt.Errorf("invalid AI input field %s", field)
				}
			}
		}
	}
	for _, form := range forms {
		spec := form.AIAssistant
		if spec == nil {
			continue
		}
		if resolved == nil || spec.LabelKey == "" || !AIPromptValid(spec.Prompt) ||
			!aiFieldsValid(spec.InputFields) || !aiFieldsValid(spec.TargetFields) || !AIAction(component, spec.Action, SuggestCapability) {
			return fmt.Errorf("invalid AI form assistant")
		}
		for _, pair := range []struct {
			command Command
			scope   string
		}{{form.Create, spec.CreateScope}, {form.Edit, spec.EditScope}} {
			if pair.command.Capability.Name == "" {
				continue
			}
			if pair.command.When.Kind != "" || output(pair.scope, false) == nil {
				return fmt.Errorf("invalid AI form scope")
			}
			for _, field := range spec.InputFields {
				if _, ok := AIFormField(pair.command, field, false); !ok {
					return fmt.Errorf("invalid AI form input %s", field)
				}
			}
			for _, field := range spec.TargetFields {
				if _, ok := AIFormField(pair.command, field, true); !ok {
					return fmt.Errorf("invalid AI form target %s", field)
				}
			}
		}
	}
	return nil
}

func aiScenarioSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id": contracts.Str(1, 100), "labelKey": contracts.Str(1, 160),
		"analysis": contracts.Object(map[string]*contracts.Schema{
			"action": contracts.Str(1, 100), "prompt": contracts.Str(1, 4096),
			"input": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"source": contracts.Str(2, 100), "fields": {Type: "array", Items: contracts.Str(1, 100)},
			}, "source", "fields")},
		}, "action", "prompt", "input"),
	}, "id", "labelKey", "analysis")
}

package spec

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"encoding/json"
	"fmt"
	js "github.com/google/jsonschema-go/jsonschema"
	"reflect"
	"strings"
	"sync"
)

// SpecificationSchema derives field names, types and required fields from the
// actual Go runtime types. Only constraints that reflection cannot infer are
// added here. This schema describes our supported DSL, not Eino's internals.
func SpecificationSchema() (*js.Schema, error) {
	refs := map[reflect.Type]*js.Schema{}
	types := map[string]reflect.Type{
		"Mapping": reflect.TypeFor[Mapping](), "Poll": reflect.TypeFor[Poll](), "Agent": reflect.TypeFor[Agent](),
		"Compensation": reflect.TypeFor[Compensation](), "Step": reflect.TypeFor[Step](),
	}
	for name, t := range types {
		refs[t] = &js.Schema{Type: "object", Ref: "#/$defs/" + name}
	}
	refs[reflect.TypeFor[contracts.Schema]()] = &js.Schema{Type: "object", Ref: "#/$defs/CapabilitySchema"}
	infer := func(t reflect.Type) (*js.Schema, error) {
		overrides := map[reflect.Type]*js.Schema{}
		for typ, s := range refs {
			if typ != t {
				overrides[typ] = s
			}
		}
		return js.ForType(t, &js.ForOptions{TypeSchemas: overrides})
	}
	root, err := infer(reflect.TypeFor[Definition]())
	if err != nil {
		return nil, err
	}
	root.Schema = "https://json-schema.org/draft/2020-12/schema"
	root.ID = "urn:baseline:workflow:1"
	root.Title = "Baseline workflow definition v1"
	root.Comment = "Generated from Go runtime types; do not edit. Capability resolution, graph semantics and authorization require separate validation."
	root.Defs = map[string]*js.Schema{}
	for name, t := range types {
		s, err := infer(t)
		if err != nil {
			return nil, err
		}
		root.Defs[name] = s
	}
	// Infer recursive capability-schema fields individually. For v0.4.3 rejects
	// recursive root types even with an override; local $defs preserve recursion.
	contract := &js.Schema{Type: "object", Properties: map[string]*js.Schema{}, AdditionalProperties: &js.Schema{Not: &js.Schema{}}}
	contractType := reflect.TypeFor[contracts.Schema]()
	for i := 0; i < contractType.NumField(); i++ {
		field := contractType.Field(i)
		parts := strings.Split(field.Tag.Get("json"), ",")
		name := parts[0]
		property, err := js.ForType(field.Type, &js.ForOptions{TypeSchemas: refs})
		if err != nil {
			return nil, err
		}
		contract.Properties[name] = property
		if len(parts) == 1 {
			contract.Required = append(contract.Required, name)
		}
	}
	root.Defs["CapabilitySchema"] = contract
	contract.Properties["type"].Enum = []any{"object", "array", "string", "integer", "number", "boolean"}
	// The workflow's input contract must describe an object, just like Gateway input.
	root.Properties["input"] = &js.Schema{AllOf: []*js.Schema{{Ref: "#/$defs/CapabilitySchema"}, {Properties: map[string]*js.Schema{"type": {Const: constant("object")}}}}}
	root.Properties["name"].Pattern = IdentifierPattern
	boundedText(root.Properties["version"], 1, MaxVersionLength)
	root.Properties["steps"].Type = "array"
	root.Properties["steps"].Types = nil
	root.Properties["steps"].MinItems = integer(1)
	root.Properties["steps"].MaxItems = integer(MaxWorkflowSteps)
	mapping := root.Defs["Mapping"]
	mapping.Properties["from"].MinLength = integer(1)
	mapping.Properties["field"].MinLength = integer(1)
	step := root.Defs["Step"]
	step.Properties["id"].Pattern = IdentifierPattern
	step.Properties["id"].Not = &js.Schema{Const: constant("input")}
	boundedText(step.Properties["capability"], 1, 80)
	boundedText(step.Properties["version"], 1, MaxVersionLength)
	step.Properties["maxAttempts"].Minimum = number(0)
	step.Properties["maxAttempts"].Maximum = number(MaxReadAttempts)
	step.OneOf = []*js.Schema{
		{Required: []string{"capability", "version"}, Not: &js.Schema{Required: []string{"agent"}}},
		{Required: []string{"agent"}, Not: &js.Schema{AnyOf: []*js.Schema{{Required: []string{"capability"}}, {Required: []string{"version"}}, {Required: []string{"poll"}}, {Required: []string{"maxAttempts"}}, {Required: []string{"compensation"}}}}},
	}
	step.If = &js.Schema{Required: []string{"poll"}}
	step.Then = &js.Schema{Properties: map[string]*js.Schema{"maxAttempts": {Maximum: number(1)}}}
	poll := root.Defs["Poll"]
	poll.Properties["doneField"].MinLength = integer(1)
	poll.Properties["maxSteps"].Minimum = number(1)
	poll.Properties["maxSteps"].Maximum = number(MaxPollSteps)
	agent := root.Defs["Agent"]
	agent.Properties["instruction"].MinLength = integer(1)
	agent.Properties["tools"].Type = "array"
	agent.Properties["tools"].Types = nil
	agent.Properties["tools"].MinItems = integer(1)
	agent.Properties["tools"].UniqueItems = true
	agent.Properties["tools"].Items.Pattern = `^[^@\s]+@[^@\s]+$`
	agent.Properties["maxIterations"].Minimum = number(1)
	agent.Properties["maxIterations"].Maximum = number(MaxAgentIterations)
	compensation := root.Defs["Compensation"]
	boundedText(compensation.Properties["capability"], 1, 80)
	boundedText(compensation.Properties["version"], 1, MaxVersionLength)
	return root, nil
}
func integer(v int) *int                     { return &v }
func number(v int) *float64                  { n := float64(v); return &n }
func constant(v any) *any                    { return &v }
func boundedText(s *js.Schema, min, max int) { s.MinLength = integer(min); s.MaxLength = integer(max) }

var specification = sync.OnceValues(func() (*js.Resolved, error) {
	s, err := SpecificationSchema()
	if err != nil {
		return nil, err
	}
	return s.Resolve(nil)
})

// DecodeDefinition rejects unknown keywords rather than silently dropping them.
// It does not register, authorize or execute the supplied definition.
func DecodeDefinition(raw []byte) (Definition, error) {
	var d Definition
	if len(raw) > MaxDefinitionBytes {
		return d, fmt.Errorf("workflow definition exceeds size limit")
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return d, fmt.Errorf("invalid workflow JSON")
	}
	schema, err := specification()
	if err != nil {
		return d, err
	}
	if schema.Validate(value) != nil {
		return d, fmt.Errorf("workflow JSON does not match generated specification")
	}
	if json.Unmarshal(raw, &d) != nil {
		return d, fmt.Errorf("invalid workflow field types")
	}
	return d, nil
}
func validateShape(d Definition) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("workflow is not JSON serializable")
	}
	_, err = DecodeDefinition(raw)
	return err
}

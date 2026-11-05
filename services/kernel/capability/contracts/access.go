package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
)

func accessSchema(root *Schema, path string) (*Schema, string, error) {
	parts := strings.Split(path, ".")
	s := root
	for i, part := range parts {
		if part == "" || part == "constructor" || part == "prototype" || part == "__proto__" {
			return nil, "", fmt.Errorf("invalid field access path")
		}
		if s != nil && s.Type == "array" {
			s = s.Items
		}
		if s == nil || s.Type != "object" || s.Properties[part] == nil {
			return nil, "", fmt.Errorf("unknown field access path")
		}
		if i == len(parts)-1 {
			return s, part, nil
		}
		s = s.Properties[part]
	}
	return nil, "", fmt.Errorf("invalid field access path")
}
func (d Definition) validateAccess() error {
	for _, spec := range []struct {
		schema *Schema
		rules  []FieldAccess
		output bool
	}{{d.Input, d.InputAccess, false}, {d.Output, d.OutputAccess, true}} {
		for _, rule := range spec.rules {
			parent, key, err := accessSchema(spec.schema, rule.Path)
			if err != nil || rule.Permission == "" {
				return fmt.Errorf("invalid field policy for %s", d.Name)
			}
			if spec.output {
				for _, required := range parent.Required {
					if required == key {
						return fmt.Errorf("conditionally readable field must be optional: %s", rule.Path)
					}
				}
			}
		}
	}
	return nil
}

// AuthorizedSchema is a derived projection. The declaration remains immutable.
// A nil projection means an inaccessible required input makes the operation unavailable.
func AuthorizedSchema(source *Schema, rules []FieldAccess, allowed func(string) bool) *Schema {
	raw, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var out Schema
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	for _, rule := range rules {
		if allowed(rule.Permission) {
			continue
		}
		parent, key, err := accessSchema(&out, rule.Path)
		if err != nil {
			continue
		}
		for _, required := range parent.Required {
			if required == key {
				return nil
			}
		}
		delete(parent.Properties, key)
	}
	return &out
}

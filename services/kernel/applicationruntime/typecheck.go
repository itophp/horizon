package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"encoding/json"
	"fmt"
	"strings"
)

func expressionType(e Expression, roots map[string]*contracts.Schema) (*contracts.Schema, error) {
	switch e.Kind {
	case "path":
		parts := strings.Split(e.Path, ".")
		s := roots[parts[0]]
		if s == nil {
			return nil, fmt.Errorf("field_not_allowed: undeclared expression root")
		}
		return schemaPath(s, strings.Join(parts[1:], "."))
	case "literal":
		var v any
		if err := json.Unmarshal([]byte(e.Literal), &v); err != nil {
			return nil, err
		}
		return valueType(v), nil
	case "object":
		fields := map[string]*contracts.Schema{}
		for _, m := range e.Fields {
			s, err := expressionType(m.Value, roots)
			if err != nil {
				return nil, err
			}
			fields[m.Target] = s
		}
		return contracts.Object(fields), nil
	case "array":
		var item *contracts.Schema
		for _, v := range e.Items {
			s, err := expressionType(v, roots)
			if err != nil {
				return nil, err
			}
			if item != nil && item.Type != s.Type {
				return nil, fmt.Errorf("schema_invalid: mixed expression array")
			}
			item = s
		}
		if item == nil {
			item = &contracts.Schema{Type: "string"}
		}
		return &contracts.Schema{Type: "array", Items: item}, nil
	case "", "eq", "in", "and", "or", "not", "exists":
		for _, v := range e.Items {
			if _, err := expressionType(v, roots); err != nil {
				return nil, err
			}
		}
		if (e.Kind == "eq" || e.Kind == "in") && len(e.Items) != 2 || (e.Kind == "not" || e.Kind == "exists") && len(e.Items) != 1 {
			return nil, fmt.Errorf("schema_invalid: expression arity")
		}
		return &contracts.Schema{Type: "boolean"}, nil
	default:
		return nil, fmt.Errorf("schema_invalid: expression kind")
	}
}
func valueType(v any) *contracts.Schema {
	switch x := v.(type) {
	case string:
		return &contracts.Schema{Type: "string"}
	case bool:
		return &contracts.Schema{Type: "boolean"}
	case float64:
		if x == float64(int64(x)) {
			return &contracts.Schema{Type: "integer"}
		}
		return &contracts.Schema{Type: "number"}
	case []any:
		s := &contracts.Schema{Type: "array", Items: &contracts.Schema{Type: "string"}}
		if len(x) > 0 {
			s.Items = valueType(x[0])
		}
		return s
	case map[string]any:
		props := map[string]*contracts.Schema{}
		for k, v := range x {
			props[k] = valueType(v)
		}
		return contracts.Object(props)
	default:
		return &contracts.Schema{Type: "null"}
	}
}
func compatible(source, target *contracts.Schema) bool {
	if source == nil || target == nil {
		return false
	}
	if source.Type != target.Type && !(source.Type == "integer" && target.Type == "number") {
		return false
	}
	if target.Type == "object" {
		for k, s := range source.Properties {
			t, ok := target.Properties[k]
			if !ok {
				if !target.AdditionalProperties {
					return false
				}
				continue
			}
			if !compatible(s, t) {
				return false
			}
		}
		for _, key := range target.Required {
			if source.Properties[key] == nil {
				return false
			}
		}
	}
	if target.Type == "array" {
		return compatible(source.Items, target.Items)
	}
	return true
}
func typedMappings(mappings []Mapping, target *contracts.Schema, roots map[string]*contracts.Schema) error {
	for _, m := range mappings {
		s, err := expressionType(m.Value, roots)
		if err != nil {
			return err
		}
		if !compatible(s, target.Properties[m.Target]) {
			return fmt.Errorf("schema_invalid: mapping type for %s", m.Target)
		}
	}
	return nil
}

// NormalizePath accepts documented shorthand at authoring time only.
func NormalizePath(path string) string {
	aliases := map[string]string{"$context.record.": "record.", "$form.values.": "form.", "$context.selection.": "selection.", "$event.": "event.", "$context.page.id": "context.pageId", "$context.user.id": "context.accountId", "$context.application.id": "context.applicationId"}
	for source, target := range aliases {
		if strings.HasSuffix(source, ".") && strings.HasPrefix(path, source) {
			return target + strings.TrimPrefix(path, source)
		}
		if path == source {
			return target
		}
	}
	return path
}

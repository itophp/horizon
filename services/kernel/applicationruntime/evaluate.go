package applicationruntime

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Evaluate interprets bounded data expressions. Missing and unsafe paths fail closed.
func Evaluate(e Expression, scope map[string]any) (any, error) { return evaluate(e, scope, 0) }
func evaluate(e Expression, scope map[string]any, depth int) (any, error) {
	if depth > 12 {
		return nil, fmt.Errorf("schema_invalid")
	}
	switch e.Kind {
	case "":
		return true, nil
	case "literal":
		var v any
		err := json.Unmarshal([]byte(e.Literal), &v)
		return v, err
	case "path":
		var v any = scope
		for _, part := range strings.Split(NormalizePath(e.Path), ".") {
			if part == "__proto__" || part == "constructor" || part == "prototype" || part == "" {
				return nil, fmt.Errorf("field_not_allowed")
			}
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("field_not_allowed")
			}
			v, ok = m[part]
			if !ok {
				return nil, fmt.Errorf("field_not_allowed")
			}
		}
		return v, nil
	case "object":
		return mapAt(e.Fields, scope, depth+1)
	case "array":
		out := []any{}
		for _, item := range e.Items {
			v, err := evaluate(item, scope, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case "exists":
		if len(e.Items) != 1 {
			return nil, fmt.Errorf("schema_invalid")
		}
		v, err := evaluate(e.Items[0], scope, depth+1)
		return err == nil && v != nil, nil
	case "not", "and", "or", "eq", "in":
		values := []any{}
		for _, item := range e.Items {
			v, err := evaluate(item, scope, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		switch e.Kind {
		case "not":
			if len(values) == 1 {
				return values[0] != true, nil
			}
		case "and":
			for _, v := range values {
				if v != true {
					return false, nil
				}
			}
			return true, nil
		case "or":
			for _, v := range values {
				if v == true {
					return true, nil
				}
			}
			return false, nil
		case "eq":
			if len(values) == 2 {
				return reflect.DeepEqual(values[0], values[1]), nil
			}
		case "in":
			if len(values) == 2 {
				if list, ok := values[1].([]any); ok {
					for _, v := range list {
						if reflect.DeepEqual(v, values[0]) {
							return true, nil
						}
					}
					return false, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("schema_invalid")
}
func Map(mappings []Mapping, scope map[string]any) (map[string]any, error) {
	return mapAt(mappings, scope, 0)
}
func mapAt(mappings []Mapping, scope map[string]any, depth int) (map[string]any, error) {
	out := map[string]any{}
	for _, m := range mappings {
		if m.Target == "__proto__" || m.Target == "constructor" || m.Target == "prototype" || strings.Contains(m.Target, ".") {
			return nil, fmt.Errorf("field_not_allowed")
		}
		v, err := evaluate(m.Value, scope, depth+1)
		if err != nil {
			return nil, err
		}
		out[m.Target] = v
	}
	return out, nil
}

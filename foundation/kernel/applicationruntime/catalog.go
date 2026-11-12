package applicationruntime

import (
	"encoding/json"
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
	"reflect"
	"strings"
)

// Canonical deep-copies contributions. Empty arrays always serialize as arrays.
func Canonical[T any](source T) T {
	raw, err := json.Marshal(source)
	if err != nil {
		panic(err)
	}
	var out T
	if err = json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	out = normalizeValue(reflect.ValueOf(out)).Interface().(T)
	return out
}

func normalizeValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		out.Set(value)
		for i := 0; i < value.NumField(); i++ {
			out.Field(i).Set(normalizeValue(value.Field(i)))
		}
		return out
	case reflect.Slice:
		out := value
		if value.IsNil() {
			out = reflect.MakeSlice(value.Type(), 0, 0)
		}
		for i := 0; i < out.Len(); i++ {
			out.Index(i).Set(normalizeValue(out.Index(i)))
		}
		return out
	case reflect.Map:
		out := reflect.MakeMap(value.Type())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), normalizeValue(iter.Value()))
		}
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(normalizeValue(value.Elem()))
		return out
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		return normalizeValue(value.Elem())
	default:
		return value
	}
}
func JSONSchema(s *contracts.Schema) string {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// Assemble validates protocol contributions against the immutable capability
// catalog. The entity parameter remains for the stable assembly API.
func Assemble(sources []ApplicationV2, _ []contracts.Entity, definitions []contracts.Definition) ([]ApplicationV2, error) {
	defs := map[string]contracts.Definition{}
	for _, d := range definitions {
		defs[d.Key()] = d
	}
	apps := Canonical(sources)
	ids := map[string]bool{}
	for ai := range apps {
		normalized, err := NormalizeV21(apps[ai])
		if err != nil {
			return nil, err
		}
		apps[ai] = normalized
		app := &apps[ai]
		if ids[app.ID] || app.ID == "" || app.Source == "" || app.Permission == "" || app.Protocol == nil {
			return nil, fmt.Errorf("schema_invalid: application %s", app.ID)
		}
		ids[app.ID] = true
		if err := assembleProtocol(app, defs); err != nil {
			return nil, fmt.Errorf("schema_invalid: application %s: %w", app.ID, err)
		}
		if err := validateReferences(*app, defs); err != nil {
			return nil, err
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("schema_invalid: dependency cycle")
		}
		if done[id] {
			return nil
		}
		if !ids[id] {
			return fmt.Errorf("schema_invalid: missing dependency")
		}
		visiting[id] = true
		for _, a := range apps {
			if a.ID == id {
				for _, d := range a.Dependencies {
					if err := visit(d); err != nil {
						return err
					}
				}
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return Canonical(apps), nil
}
func schemaPath(s *contracts.Schema, path string) (*contracts.Schema, error) {
	if path == "" {
		return s, nil
	}
	for _, part := range strings.Split(path, ".") {
		if s == nil {
			return nil, fmt.Errorf("field_not_allowed")
		}
		s = s.Properties[part]
	}
	if s == nil {
		return nil, fmt.Errorf("field_not_allowed")
	}
	return s, nil
}
func checkMappings(m []Mapping, s *contracts.Schema) error {
	seen := map[string]bool{}
	for _, b := range m {
		if seen[b.Target] || s.Properties[b.Target] == nil {
			return fmt.Errorf("field_not_allowed: mapping %s", b.Target)
		}
		seen[b.Target] = true
		if err := ValidateExpression(b.Value, 0); err != nil {
			return err
		}
		if b.Value.Kind == "literal" {
			var v any
			if err := json.Unmarshal([]byte(b.Value.Literal), &v); err != nil {
				return err
			}
			raw := contracts.Input{b.Target: v}
			test := contracts.Object(map[string]*contracts.Schema{b.Target: s.Properties[b.Target]}, b.Target)
			if err := test.Validate(raw); err != nil {
				return fmt.Errorf("schema_invalid: mapping type")
			}
		}
	}
	for _, r := range s.Required {
		if !seen[r] {
			return fmt.Errorf("schema_invalid: missing mapping %s", r)
		}
	}
	return nil
}
func ValidateExpression(e Expression, depth int) error {
	if depth > 12 {
		return fmt.Errorf("schema_invalid: expression depth")
	}
	switch e.Kind {
	case "":
		return nil
	case "literal":
		var v any
		if err := json.Unmarshal([]byte(e.Literal), &v); err != nil {
			return fmt.Errorf("schema_invalid: literal")
		}
	case "path":
		if !strings.HasPrefix(e.Path, "record.") && !strings.HasPrefix(e.Path, "form.") && !strings.HasPrefix(e.Path, "route.") && !strings.HasPrefix(e.Path, "query.") && !strings.HasPrefix(e.Path, "selection.") && !strings.HasPrefix(e.Path, "event.") && !strings.HasPrefix(e.Path, "context.") && !strings.HasPrefix(e.Path, "data.") {
			return fmt.Errorf("field_not_allowed: expression root")
		}
		for _, part := range strings.Split(e.Path, ".") {
			if part == "__proto__" || part == "constructor" || part == "prototype" || part == "" {
				return fmt.Errorf("field_not_allowed: expression path")
			}
		}
	case "object", "array", "eq", "in", "and", "or", "not", "exists", "coalesce", "concat", "count", "find", "filter", "get", "if", "encode", "replace":
	default:
		return fmt.Errorf("schema_invalid: expression kind")
	}
	for _, i := range e.Items {
		if err := ValidateExpression(i, depth+1); err != nil {
			return err
		}
	}
	fieldNames := map[string]bool{}
	for _, f := range e.Fields {
		if fieldNames[f.Target] {
			return fmt.Errorf("schema_invalid: duplicate expression field")
		}
		fieldNames[f.Target] = true
		if f.Target == "__proto__" || f.Target == "constructor" || f.Target == "prototype" {
			return fmt.Errorf("field_not_allowed")
		}
		if err := ValidateExpression(f.Value, depth+1); err != nil {
			return err
		}
	}
	return nil
}

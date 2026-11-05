package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"encoding/json"
	"fmt"
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
	var normalize func(reflect.Value)
	normalize = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				normalize(v.Field(i))
			}
		case reflect.Slice:
			if v.IsNil() {
				v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			}
			for i := 0; i < v.Len(); i++ {
				normalize(v.Index(i))
			}
		case reflect.Pointer:
			if !v.IsNil() {
				normalize(v.Elem())
			}
		}
	}
	normalize(reflect.ValueOf(&out).Elem())
	return out
}
func JSONSchema(s *contracts.Schema) string {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// Assemble consumes the same entity and capability catalogs as the Gateway.
func Assemble(sources []ApplicationV2, entities []contracts.Entity, definitions []contracts.Definition) ([]ApplicationV2, error) {
	defs := map[string]contracts.Definition{}
	for _, d := range definitions {
		defs[d.Key()] = d
	}
	es := map[string]contracts.Entity{}
	for _, e := range entities {
		es[e.Name] = e
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
		if ids[app.ID] || app.ID == "" || app.Source == "" || app.Permission == "" || app.SchemaVersion != "2.2" || app.RuntimeCompatibility != "2.2" {
			return nil, fmt.Errorf("schema_invalid: application %s", app.ID)
		}
		ids[app.ID] = true
		providers := map[string]*Provider{}
		for pi := range app.Providers {
			p := &app.Providers[pi]
			d, ok := defs[p.Capability.Name+"@"+p.Capability.Version]
			if !ok || !d.ReadOnly || providers[p.ID] != nil || p.KeyField == "" {
				return nil, fmt.Errorf("schema_invalid: provider %s", p.ID)
			}
			p.Permission = d.Permission
			p.InputSchema = JSONSchema(d.Input)
			projected, e := providerSchema(d.Output, *p)
			if e != nil {
				return nil, e
			}
			p.OutputSchema = JSONSchema(projected)
			if err := checkMappings(p.Input, d.Input); err != nil {
				return nil, err
			}
			providers[p.ID] = p
		}
		pageIDs := map[string]bool{}
		actionIDs := map[string]bool{}
		for pi := range app.Pages {
			p := &app.Pages[pi]
			if pageIDs[p.ID] || p.ID == "" || p.SchemaVersion != "2.2" || p.Permission == "" {
				return nil, fmt.Errorf("schema_invalid: page %s", p.ID)
			}
			pageIDs[p.ID] = true
			if p.Type != "list" && p.Type != "detail" && p.Type != "form" && p.Type != "custom" {
				return nil, fmt.Errorf("schema_invalid: page type")
			}
			if p.Mode != "metadata" && p.Mode != "hybrid" && p.Mode != "custom" {
				return nil, fmt.Errorf("schema_invalid: page mode")
			}
			if p.Layout.Type != "standard" && p.Layout.Type != "section" && p.Layout.Type != "grid" && p.Layout.Type != "tabs" && p.Layout.Type != "split" && p.Layout.Type != "master-detail" {
				return nil, fmt.Errorf("schema_invalid: layout")
			}
			provider := providers[p.Provider]
			e, ok := es[p.EntityRef]
			if !ok || provider == nil {
				return nil, fmt.Errorf("schema_invalid: entity/provider")
			}
			read := defs[provider.Capability.Name+"@"+provider.Capability.Version].Output
			output, err := schemaPath(read, provider.ResultPath)
			if err != nil {
				return nil, err
			}
			if output.Type == "array" {
				output = output.Items
			}
			if output == nil || output.Type != "object" {
				return nil, fmt.Errorf("schema_invalid: result projection")
			}
			if output.Properties[provider.KeyField] == nil {
				return nil, fmt.Errorf("field_not_allowed: provider key")
			}
			if provider.LabelField != "" && output.Properties[provider.LabelField] == nil {
				return nil, fmt.Errorf("field_not_allowed: provider label")
			}
			queryProps := map[string]*contracts.Schema{"page": contracts.Int(1, 1000000), "pageSize": contracts.Int(1, 100), "sort": {Type: "string"}}
			for _, filter := range p.Table.Filters {
				typ := filter.Type
				if typ == "enum" {
					typ = "string"
				}
				if typ != "string" && typ != "integer" && typ != "number" && typ != "boolean" {
					return nil, fmt.Errorf("schema_invalid: filter type")
				}
				queryProps[filter.Name] = &contracts.Schema{Type: typ}
			}
			roots := map[string]*contracts.Schema{"record": output, "query": contracts.Object(queryProps), "route": contracts.Object(map[string]*contracts.Schema{"id": output.Properties[provider.KeyField]}), "context": contracts.Object(map[string]*contracts.Schema{"operationId": {Type: "string"}, "accountId": {Type: "integer"}, "applicationId": {Type: "string"}, "pageId": {Type: "string"}, "locale": {Type: "string"}, "timezone": {Type: "string"}, "canUseAssistant": {Type: "boolean"}}), "selection": contracts.Object(map[string]*contracts.Schema{"ids": {Type: "array", Items: output.Properties[provider.KeyField]}})}
			if err := typedMappings(provider.Input, defs[provider.Capability.Name+"@"+provider.Capability.Version].Input, roots); err != nil {
				return nil, err
			}
			for _, fields := range []*[]FieldV2{&p.Fields, &p.Table.Columns} {
				if err := adaptFields(fields, e, output, nil, p.Type == "list"); err != nil {
					return nil, err
				}
			}
			p.Relations = []Relation{}
			for _, r := range e.Relations {
				p.Relations = append(p.Relations, Relation{Name: r.Name, Target: r.Target, Kind: r.Kind})
			}
			for fi := range p.Forms {
				f := &p.Forms[fi]
				for _, c := range []*Command{&f.Create, &f.Edit} {
					if c.Capability.Name == "" {
						continue
					}
					d, ok := defs[c.Capability.Name+"@"+c.Capability.Version]
					if !ok || d.ReadOnly {
						return nil, fmt.Errorf("schema_invalid: command")
					}
					if c.ActionType == "" {
						c.ActionType = "capability"
					}
					if c.ActionType != "capability" && c.ActionType != "batch" {
						return nil, fmt.Errorf("schema_invalid: command action type")
					}
					c.Permission = d.Permission
					c.InputSchema = JSONSchema(d.Input)
					formProperties := map[string]*contracts.Schema{}
					for _, field := range c.Fields {
						formProperties[field.Name] = d.Input.Properties[field.Name]
					}
					roots["form"] = contracts.Object(formProperties)
					if err := typedMappings(c.Input, d.Input, roots); err != nil {
						return nil, err
					}
					if condition, err := expressionType(c.When, roots); err != nil || condition.Type != "boolean" {
						return nil, fmt.Errorf("schema_invalid: command condition")
					}
					for _, field := range c.Fields {
						if field.Default.Kind != "" {
							value, err := expressionType(field.Default, roots)
							if err != nil || !compatible(value, d.Input.Properties[field.Name]) {
								return nil, fmt.Errorf("schema_invalid: field default")
							}
						}
						if condition, err := expressionType(field.VisibleWhen, roots); err != nil || condition.Type != "boolean" {
							return nil, fmt.Errorf("schema_invalid: field condition")
						}
					}

					if err := checkMappings(c.Input, d.Input); err != nil {
						return nil, err
					}
					if err := adaptFields(&c.Fields, e, nil, d.Input, false); err != nil {
						return nil, err
					}
					for _, field := range c.Fields {
						if field.Reference != "" && providers[field.Reference] == nil {
							return nil, fmt.Errorf("schema_invalid: reference provider")
						}
					}
				}
			}
			for i := range p.Actions {
				a := &p.Actions[i]
				delete(roots, "event")
				if a.EventSchema != "" {
					var event contracts.Schema
					if err := json.Unmarshal([]byte(a.EventSchema), &event); err != nil || event.Type != "object" {
						return nil, fmt.Errorf("schema_invalid: event contract")
					}
					roots["event"] = &event
				}

				if actionIDs[a.ID] || a.ID == "" {
					return nil, fmt.Errorf("schema_invalid: duplicate action")
				}
				actionIDs[a.ID] = true
				switch a.Type {
				case "capability", "workflow", "batch", "custom", "agent":
					d, ok := defs[a.Capability.Name+"@"+a.Capability.Version]
					if !ok {
						return nil, fmt.Errorf("schema_invalid: action capability")
					}
					a.Permission = d.Permission
					a.InputSchema = JSONSchema(d.Input)
					if err := typedMappings(a.Input, d.Input, roots); err != nil {
						return nil, err
					}

					if err := checkMappings(a.Input, d.Input); err != nil {
						return nil, err
					}
				case "navigation", "dialog":
					if a.Target == "" {
						return nil, fmt.Errorf("schema_invalid: target")
					}
				default:
					return nil, fmt.Errorf("schema_invalid: action type")
				}
				if condition, err := expressionType(a.When, roots); err != nil || condition.Type != "boolean" {
					return nil, fmt.Errorf("schema_invalid: action condition")
				}
				for _, mapping := range a.Input {
					if err := ValidateExpression(mapping.Value, 0); err != nil {
						return nil, err
					}
				}
				if err := ValidateExpression(a.When, 0); err != nil {
					return nil, err
				}
			}
		}
		for _, p := range app.Pages {
			for _, a := range p.Actions {
				if a.Presentation != "" && a.Presentation != "dialog" && a.Presentation != "drawer" {
					return nil, fmt.Errorf("schema_invalid: presentation")
				}
				if a.Type == "navigation" && !pageIDs[a.Target] {
					return nil, fmt.Errorf("schema_invalid: navigation")
				}
				if a.Type == "dialog" {
					found := a.Mode == "detail" && pageIDs[a.Target]
					for _, f := range p.Forms {
						if f.ID == a.Target {
							found = true
						}
					}
					if !found {
						return nil, fmt.Errorf("schema_invalid: form target")
					}
				}
			}
			for _, f := range p.Table.Columns {
				if f.LinkPage != "" && !pageIDs[f.LinkPage] {
					return nil, fmt.Errorf("schema_invalid: linked page")
				}
			}
		}
		extIDs := map[string]bool{}
		for _, x := range app.Extensions {
			if extIDs[x.ID] || (x.Source != app.Source && app.Source != "builtin:runtime-host") || x.Version == "" || (x.RuntimeCompatibility != "2.0" && x.RuntimeCompatibility != "2.1" && x.RuntimeCompatibility != "2.2") {
				return nil, fmt.Errorf("schema_invalid: extension")
			}
			extIDs[x.ID] = true
		}
		for _, page := range app.Pages {
			entity := es[page.EntityRef]
			for _, binding := range page.RelationBindings {
				found := false
				for _, relation := range entity.Relations {
					if relation.Target == binding.EntityRef {
						found = true
					}
				}
				if !found {
					return nil, fmt.Errorf("schema_invalid: undeclared relation binding")
				}
			}
		}
		if err := validateAdvanced(*app, defs); err != nil {
			return nil, err
		}
		if err := validateReferences(*app, defs); err != nil {
			return nil, err
		}
		app.MetadataVersion = ""
		app.MetadataVersion = contracts.Digest(contracts.Input{"definition": Canonical(*app)})
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
func adaptFields(fields *[]FieldV2, e contracts.Entity, output, input *contracts.Schema, list bool) error {
	seen := map[string]bool{}
	for i := range *fields {
		f := &(*fields)[i]
		if seen[f.Name] || f.Name == "" || f.LabelKey == "" {
			return fmt.Errorf("field_not_allowed")
		}
		seen[f.Name] = true
		a, exists := e.Attribute(f.Name)
		var s *contracts.Schema
		if input != nil {
			s = input.Properties[f.Name]
			if !exists || a.Kind == contracts.AttributeSystem || a.Kind == contracts.AttributeState || s == nil {
				return fmt.Errorf("field_not_allowed: input %s", f.Name)
			}
		} else {
			s = output.Properties[f.Name]
			if s == nil || !exists && !f.Derived {
				return fmt.Errorf("field_not_allowed: output %s", f.Name)
			}
			f.ReadOnly = true
		}
		if exists {
			f.Sensitive = a.Sensitive
		}
		if list && f.Sensitive {
			return fmt.Errorf("field_not_allowed: sensitive list")
		}
		if f.Type == "" {
			f.Type = s.Type
		}
		if len(s.Enum) > 0 {
			f.Enum = append([]string{}, s.Enum...)
			if f.Reference == "" {
				f.Type = "enum"
			}
		}
		if s.MaxLength != nil {
			f.MaxLength = *s.MaxLength
		}
		if s.MinLength != nil {
			f.MinLength = *s.MinLength
		}
		f.Minimum = s.Minimum
		f.Maximum = s.Maximum
		f.Required = false
		if input != nil {
			for _, r := range input.Required {
				if r == f.Name {
					f.Required = true
				}
			}
		}
		if err := normalizeNestedField(f, s, 0); err != nil {
			return err
		}
		if f.Component == "" {
			f.Component = f.Type
		}
		if f.ComponentVersion == "" {
			f.ComponentVersion = "2.0"
		}
		if f.Component == "" {
			return fmt.Errorf("component_not_registered")
		}
		if err := ValidateExpression(f.VisibleWhen, 0); err != nil {
			return err
		}
		if err := ValidateExpression(f.Default, 0); err != nil {
			return err
		}
	}
	return nil
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
		if !strings.HasPrefix(e.Path, "record.") && !strings.HasPrefix(e.Path, "form.") && !strings.HasPrefix(e.Path, "route.") && !strings.HasPrefix(e.Path, "query.") && !strings.HasPrefix(e.Path, "selection.") && !strings.HasPrefix(e.Path, "event.") && !strings.HasPrefix(e.Path, "context.") {
			return fmt.Errorf("field_not_allowed: expression root")
		}
		for _, part := range strings.Split(e.Path, ".") {
			if part == "__proto__" || part == "constructor" || part == "prototype" || part == "" {
				return fmt.Errorf("field_not_allowed: expression path")
			}
		}
	case "object", "array", "eq", "in", "and", "or", "not", "exists":
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

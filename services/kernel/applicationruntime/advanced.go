package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"encoding/json"
	"fmt"
	"strings"
)

func validateAdvanced(app ApplicationV2, defs map[string]contracts.Definition) error {
	if !appVersionPattern.MatchString(app.PluginVersion) {
		return fmt.Errorf("schema_invalid: plugin version")
	}
	for _, dep := range app.PluginDependencies {
		if dep.ID == app.Source || dep.ID == "" || !appVersionPattern.MatchString(dep.Version) {
			return fmt.Errorf("schema_invalid: plugin dependency")
		}
	}
	for _, ref := range app.AgentTools {
		if _, ok := defs[ref.Name+"@"+ref.Version]; !ok {
			return fmt.Errorf("schema_invalid: agent tool")
		}
	}
	pages := map[string]PageV2{}
	for _, p := range app.Pages {
		pages[p.ID] = p
	}

	for _, extension := range app.Extensions {
		switch extension.Type {
		case "application", "navigation", "page", "component", "renderer", "custom-view", "action", "provider", "context", "theme", "locale":
		case "shell":
			switch extension.Slot {
			case "navigation", "breadcrumb", "toolbar", "commands":
			default:
				return fmt.Errorf("schema_invalid: shell slot")
			}
		default:
			return fmt.Errorf("schema_invalid: extension type")
		}
	}
	for _, nav := range app.Navigation {
		if _, ok := pages[nav.PageID]; !ok || nav.LabelKey == "" {
			return fmt.Errorf("schema_invalid: navigation")
		}
	}
	visiting, donePages := map[string]bool{}, map[string]bool{}
	var visitPage func(string) error
	visitPage = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("schema_invalid: embedded page cycle")
		}
		if donePages[id] {
			return nil
		}
		visiting[id] = true
		for _, section := range pages[id].Layout.Sections {
			if section.PageID != "" {
				if err := visitPage(section.PageID); err != nil {
					return err
				}
			}
		}
		visiting[id] = false
		donePages[id] = true
		return nil
	}
	for id := range pages {
		if err := visitPage(id); err != nil {
			return err
		}
	}
	for _, p := range app.Providers {
		if p.TimeoutMs < 0 || p.TimeoutMs > 5000 || p.StaleTimeMs < 0 || p.StaleTimeMs > 300000 || p.MaxDepth > 2 || len(p.Select) > 50 || len(p.QueryBindings) > 20 {
			return fmt.Errorf("schema_invalid: query budget")
		}
		d := defs[p.Capability.Name+"@"+p.Capability.Version]
		if p.FilterTarget != "" {
			target := d.Input.Properties[p.FilterTarget]
			if target == nil || target.Type != "object" {
				return fmt.Errorf("schema_invalid: native filter target")
			}
		}
		s, err := schemaPath(d.Output, p.ResultPath)
		if err != nil {
			return err
		}
		if s.Type == "array" {
			s = s.Items
		}
		for _, field := range p.Select {
			if s.Properties[field] == nil {
				return fmt.Errorf("field_not_allowed: selection")
			}
		}
		seen := map[string]bool{}
		for _, b := range p.QueryBindings {
			if b.Field == "" || seen[b.Field+":"+b.Operator] || d.Input.Properties[b.Target] == nil || !strings.Contains("|eq|in|contains|range|", "|"+b.Operator+"|") {
				return fmt.Errorf("schema_invalid: filter binding")
			}
			seen[b.Field+":"+b.Operator] = true
			if b.Operator == "in" && d.Input.Properties[b.Target].Type != "array" {
				return fmt.Errorf("schema_invalid: filter type")
			}
		}
	}
	for _, p := range app.Pages {
		for _, binding := range p.RelationBindings {
			if binding.MaxItems < 1 || binding.MaxItems > 10 {
				return fmt.Errorf("schema_invalid: relation budget")
			}
			found := false
			for _, provider := range app.Providers {
				if provider.ID == binding.ProviderID {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("schema_invalid: relation provider")
			}
		}
		if p.Table.RowPage != "" {
			if _, ok := pages[p.Table.RowPage]; !ok {
				return fmt.Errorf("schema_invalid: row page")
			}
		}
		for _, section := range p.Layout.Sections {
			if section.PageID != "" {
				if _, ok := pages[section.PageID]; !ok || section.PageID == p.ID {
					return fmt.Errorf("schema_invalid: layout page")
				}
			}
		}
		for _, form := range p.Forms {
			groups := map[string]bool{}
			for _, g := range form.Groups {
				if groups[g.ID] || g.ID == "" || g.LabelKey == "" || !strings.Contains("|section|collapse|tabs|", "|"+g.Type+"|") {
					return fmt.Errorf("schema_invalid: form group")
				}
				groups[g.ID] = true
			}
			for _, c := range []Command{form.Create, form.Edit} {
				if c.Capability.Name == "" {
					continue
				}
				d := defs[c.Capability.Name+"@"+c.Capability.Version]
				names := map[string]FieldV2{}
				for _, f := range c.Fields {
					names[f.Name] = f
					if f.Group != "" && !groups[f.Group] {
						return fmt.Errorf("schema_invalid: field group")
					}
				}
				visiting, done := map[string]bool{}, map[string]bool{}
				var visit func(string) error
				visit = func(name string) error {
					if visiting[name] {
						return fmt.Errorf("schema_invalid: field dependency cycle")
					}
					if done[name] {
						return nil
					}
					f, ok := names[name]
					if !ok {
						return fmt.Errorf("field_not_allowed: field dependency")
					}
					visiting[name] = true
					dependencies := append([]string{}, f.DependsOn...)
					var collect func(Expression)
					collect = func(expr Expression) {
						if expr.Kind == "path" && strings.HasPrefix(expr.Path, "form.") {
							dependencies = append(dependencies, strings.Split(strings.TrimPrefix(expr.Path, "form."), ".")[0])
						}
						for _, item := range expr.Items {
							collect(item)
						}
						for _, field := range expr.Fields {
							collect(field.Value)
						}
					}
					collect(f.Compute)
					for _, dep := range dependencies {
						if err := visit(dep); err != nil {
							return err
						}
					}
					visiting[name] = false
					done[name] = true
					return nil
				}
				for _, f := range c.Fields {
					if err := visit(f.Name); err != nil {
						return err
					}
					if err := validateFieldTree(f, d.Input.Properties[f.Name], 0); err != nil {
						return err
					}
					for _, expr := range []Expression{f.DisabledWhen, f.Compute} {
						if err := ValidateExpression(expr, 0); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	for _, c := range app.Components {
		if c.Source != app.Source || c.Name == "" || c.Version == "" {
			return fmt.Errorf("plugin_incompatible")
		}
		var schema contracts.Schema
		if json.Unmarshal([]byte(c.PropsSchema), &schema) != nil || schema.Type != "object" {
			return fmt.Errorf("schema_invalid: component props")
		}
		for _, ref := range c.Capabilities {
			if _, ok := defs[ref.Name+"@"+ref.Version]; !ok {
				return fmt.Errorf("schema_invalid: component capability")
			}
		}
	}
	return nil
}
func validateFieldTree(f FieldV2, s *contracts.Schema, depth int) error {
	if s == nil || depth > 2 || f.MaxItems > 100 {
		return fmt.Errorf("schema_invalid: field nesting")
	}
	if len(f.Children) > 0 {
		if s.Type != "array" || s.Items == nil || s.Items.Type != "object" {
			return fmt.Errorf("schema_invalid: subtable")
		}
		seen := map[string]bool{}
		for _, child := range f.Children {
			if seen[child.Name] {
				return fmt.Errorf("schema_invalid: child field")
			}
			seen[child.Name] = true
			if err := validateFieldTree(child, s.Items.Properties[child.Name], depth+1); err != nil {
				return err
			}
		}
	}
	switch f.Type {
	case "money", "time", "datetime":
		if s.Type != "string" {
			return fmt.Errorf("schema_invalid: precise field representation")
		}
	case "array":
		if s.Type != "array" {
			return fmt.Errorf("schema_invalid: array field")
		}
	}
	return nil
}

func normalizeNestedField(field *FieldV2, schema *contracts.Schema, depth int) error {
	if schema == nil || depth > 2 {
		return fmt.Errorf("schema_invalid: field nesting")
	}
	if field.Type == "" {
		field.Type = schema.Type
	}
	if field.Component == "" {
		if schema.Type == "array" {
			if schema.Items != nil && schema.Items.Type == "object" {
				field.Component = "subtable"
			} else {
				field.Component = "multiselect"
				field.Multiple = true
			}
		} else {
			field.Component = field.Type
		}
	}
	if field.ComponentVersion == "" {
		field.ComponentVersion = "2.0"
	}
	if schema.MinItems != nil {
		field.MinItems = *schema.MinItems
	}
	if schema.MaxItems != nil {
		field.MaxItems = *schema.MaxItems
	}
	if schema.Type == "array" && schema.Items != nil && len(schema.Items.Enum) > 0 {
		field.Enum = append([]string{}, schema.Items.Enum...)
	}
	for i := range field.Children {
		child := &field.Children[i]
		if schema.Items == nil {
			return fmt.Errorf("schema_invalid: subtable schema")
		}
		childSchema := schema.Items.Properties[child.Name]
		if childSchema == nil {
			return fmt.Errorf("field_not_allowed: child")
		}
		child.ReadOnly = child.ReadOnly || field.ReadOnly
		child.Sensitive = child.Sensitive || field.Sensitive
		child.Required = false
		for _, name := range schema.Items.Required {
			if name == child.Name {
				child.Required = true
			}
		}
		child.Minimum = childSchema.Minimum
		child.Maximum = childSchema.Maximum
		if childSchema.MaxLength != nil {
			child.MaxLength = *childSchema.MaxLength
		}
		if childSchema.MinLength != nil {
			child.MinLength = *childSchema.MinLength
		}
		child.Enum = append([]string{}, childSchema.Enum...)
		if err := normalizeNestedField(child, childSchema, depth+1); err != nil {
			return err
		}
	}
	return nil
}
func providerSchema(source *contracts.Schema, provider Provider) (*contracts.Schema, error) {
	result := Canonical(*source)
	if len(provider.Select) == 0 {
		return &result, nil
	}
	row, err := schemaPath(&result, provider.ResultPath)
	if err != nil {
		return nil, err
	}
	if row.Type == "array" {
		row = row.Items
	}
	if row == nil || row.Type != "object" {
		return nil, fmt.Errorf("schema_invalid")
	}
	keep := map[string]bool{provider.KeyField: true}
	for _, name := range provider.Select {
		keep[name] = true
	}
	for name := range row.Properties {
		if !keep[name] {
			delete(row.Properties, name)
		}
	}
	required := []string{}
	for _, name := range row.Required {
		if keep[name] {
			required = append(required, name)
		}
	}
	row.Required = required
	return &result, nil
}

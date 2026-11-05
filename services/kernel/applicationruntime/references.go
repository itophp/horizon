package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"fmt"
	"regexp"
	"strings"
)

var routePattern = regexp.MustCompile(`^/[a-zA-Z0-9_:/-]*$`)
var appVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// CoreFieldComponents is shared by validation and the authorized design catalog.
func CoreFieldComponents() []string {
	return []string{"text", "textarea", "string", "integer", "number", "boolean", "enum", "date", "reference", "timestamp", "money", "time", "datetime", "multiselect", "file", "subtable", "switch"}
}

func validateReferences(app ApplicationV2, definitions map[string]contracts.Definition) error {
	if !appVersionPattern.MatchString(app.Version) {
		return fmt.Errorf("schema_invalid: application version")
	}
	extensions := map[string]ExtensionV2{}
	for _, x := range app.Extensions {
		if x.Entry == "" || x.RuntimeCompatibility != "2.0" && x.RuntimeCompatibility != "2.1" && x.RuntimeCompatibility != "2.2" {
			return fmt.Errorf("plugin_incompatible")
		}
		extensions[x.ID] = x
	}
	tags := map[string]bool{}
	for _, p := range app.Providers {
		if p.Adapter != "" {
			x, ok := extensions[p.Adapter]
			if !ok || x.Type != "provider" {
				return fmt.Errorf("component_not_registered: provider adapter")
			}
		}
		for _, tag := range p.Tags {
			if tag == "" {
				return fmt.Errorf("schema_invalid: resource tag")
			}
			tags[tag] = true
		}
	}
	checkTags := func(values []string) error {
		for _, tag := range values {
			if !tags[tag] {
				return fmt.Errorf("schema_invalid: unknown resource tag")
			}
		}
		return nil
	}
	knownPermissions := map[string]bool{}
	for _, d := range definitions {
		if d.Permission != "" {
			knownPermissions[d.Permission] = true
		}
	}
	if !knownPermissions[app.Permission] {
		return fmt.Errorf("schema_invalid: unknown application permission")
	}
	for _, p := range app.Providers {
		if p.Pagination != "" && p.Pagination != "none" && p.Pagination != "server" {
			return fmt.Errorf("schema_invalid: pagination")
		}
	}
	for _, p := range app.Pages {
		if p.Type == "form" {
			count := 0
			for _, action := range p.Actions {
				if action.Type == "dialog" && (action.Mode == "create" || action.Mode == "edit") {
					count++
				}
			}
			if count != 1 {
				return fmt.Errorf("schema_invalid: form page requires one explicit command action")
			}
		}
		var binding Provider
		for _, provider := range app.Providers {
			if provider.ID == p.Provider {
				binding = provider
			}
		}
		capability := definitions[binding.Capability.Name+"@"+binding.Capability.Version]
		for _, m := range binding.Input {
			target := capability.Input.Properties[m.Target]
			if m.Value.Kind != "path" {
				continue
			}
			if m.Value.Path == "query.pageSize" {
				if binding.PageSize < 1 || binding.PageSize > 100 || target.Minimum != nil && float64(binding.PageSize) < *target.Minimum || target.Maximum != nil && float64(binding.PageSize) > *target.Maximum {
					return fmt.Errorf("schema_invalid: page size")
				}
			}
			if m.Value.Path == "query.sort" {
				for _, sort := range p.Table.Sorts {
					if len(target.Enum) > 0 && !includes(target.Enum, sort.Value) {
						return fmt.Errorf("schema_invalid: sort")
					}
				}
				if p.Table.DefaultSort != "" && len(target.Enum) > 0 && !includes(target.Enum, p.Table.DefaultSort) {
					return fmt.Errorf("schema_invalid: default sort")
				}
			}
			for _, filter := range p.Table.Filters {
				if m.Value.Path == "query."+filter.Name && len(target.Enum) > 0 {
					for _, value := range filter.Enum {
						if !includes(target.Enum, value) {
							return fmt.Errorf("schema_invalid: filter enum")
						}
					}
				}
			}
		}
		if binding.PollInterval < 0 || binding.PollInterval > 0 && binding.PollInterval < 1000 {
			return fmt.Errorf("schema_invalid: polling interval")
		}

		if !knownPermissions[p.Permission] {
			return fmt.Errorf("schema_invalid: unknown page permission")
		}
		allFields := append(append([]FieldV2{}, p.Fields...), p.Table.Columns...)
		for _, form := range p.Forms {
			allFields = append(allFields, form.Create.Fields...)
			allFields = append(allFields, form.Edit.Fields...)
		}
		for _, f := range allFields {
			core := includes(CoreFieldComponents(), f.Component)
			if core {
				if f.ComponentVersion != "2.0" {
					return fmt.Errorf("schema_version_unsupported: component")
				}
			} else {
				found := false
				for _, x := range extensions {
					if x.Type == "component" && x.Entry == f.Component && x.Version == f.ComponentVersion {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("component_not_registered")
				}
			}
		}

		if p.Route != "" && (!routePattern.MatchString(p.Route) || strings.HasPrefix(p.Route, "//") || strings.Contains(p.Route, "..")) {
			return fmt.Errorf("schema_invalid: route")
		}
		for _, section := range p.Layout.Sections {
			if section.Slot != "" {
				x, ok := extensions[section.Slot]
				if !ok || (x.Type != "component" && x.Type != "renderer" && x.Type != "custom-view") {
					return fmt.Errorf("component_not_registered: slot")
				}
			}
			for _, name := range section.Fields {
				found := false
				for _, f := range p.Fields {
					if f.Name == name {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("field_not_allowed: layout field")
				}
			}
		}
		if p.Mode == "custom" {
			x, ok := extensions[p.View]
			if !ok || (x.Type != "component" && x.Type != "renderer" && x.Type != "custom-view") {
				return fmt.Errorf("component_not_registered: custom view")
			}
		}
		forms := map[string]bool{}
		for _, f := range p.Forms {
			if forms[f.ID] || f.ID == "" {
				return fmt.Errorf("schema_invalid: duplicate form")
			}
			forms[f.ID] = true
			if err := checkTags(f.Tags); err != nil {
				return err
			}
		}
		for _, a := range p.Actions {
			if err := checkTags(a.Tags); err != nil {
				return err
			}
			if (a.Type == "workflow" || a.Type == "agent") && a.Capability.Name != "orchestration.workflow.run" {
				return fmt.Errorf("schema_invalid: workflow adapter")
			}
			if a.Type == "batch" {
				d := definitions[a.Capability.Name+"@"+a.Capability.Version]
				if d.Layer != contracts.LayerOrchestration || d.ReadOnly {
					return fmt.Errorf("schema_invalid: batch adapter")
				}
			}
			if a.Type == "custom" {
				x, ok := extensions[a.Target]
				if !ok || x.Type != "action" {
					return fmt.Errorf("action_not_found")
				}
			}
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("plugin_incompatible: extension cycle")
		}
		if done[id] {
			return nil
		}
		x, ok := extensions[id]
		if !ok {
			return fmt.Errorf("plugin_incompatible: missing extension dependency")
		}
		visiting[id] = true
		for _, dep := range x.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(visiting, id)
		done[id] = true
		return nil
	}
	for id := range extensions {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func includes(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

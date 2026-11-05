package applicationruntime

import (
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"
	"encoding/json"
	"strings"
)

func projectFields(fields []ar.FieldV2, p *identity.Principal, readable *contracts.Schema) []ar.FieldV2 {
	out := []ar.FieldV2{}
	for _, field := range fields {
		if field.Permission != "" && !p.Can(field.Permission) {
			continue
		}
		if readable != nil && readable.Properties[field.Name] == nil {
			continue
		}
		if field.WritePermission != "" && !p.Can(field.WritePermission) {
			field.ReadOnly = true
		}
		var child *contracts.Schema
		if readable != nil {
			child = readable.Properties[field.Name]
			if child != nil && child.Type == "array" {
				child = child.Items
			}
		}
		field.Children = projectFields(field.Children, p, child)
		out = append(out, field)
	}
	return out
}
func (m *Module) projectCommand(command ar.Command, p *identity.Principal) ar.Command {
	definition, ok := m.Definitions[command.Capability.Name+"@"+command.Capability.Version]
	if !ok {
		return command
	}
	input := contracts.AuthorizedSchema(definition.Input, definition.InputAccess, p.Can)
	if input == nil {
		return ar.Command{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ar.Command{}
	}
	command.InputSchema = string(raw)
	denied := map[string]bool{}
	for _, mapping := range command.Input {
		if input.Properties[mapping.Target] == nil && mapping.Value.Kind == "path" && strings.HasPrefix(mapping.Value.Path, "form.") {
			denied[strings.TrimPrefix(mapping.Value.Path, "form.")] = true
		}
	}
	kept := []ar.Mapping{}
	for _, mapping := range command.Input {
		if input.Properties[mapping.Target] != nil {
			kept = append(kept, mapping)
		}
	}
	command.Input = kept
	fields := []ar.FieldV2{}
	for _, field := range projectFields(command.Fields, p, nil) {
		if !denied[field.Name] {
			fields = append(fields, field)
		}
	}
	command.Fields = fields
	return command
}
func (m *Module) projectPageFields(app ar.ApplicationV2, page *ar.PageV2, p *identity.Principal) {
	var readable *contracts.Schema
	for _, provider := range app.Providers {
		if provider.ID != page.Provider {
			continue
		}
		definition, ok := m.Definitions[provider.Capability.Name+"@"+provider.Capability.Version]
		if !ok {
			continue
		}
		readable = contracts.AuthorizedSchema(definition.Output, definition.OutputAccess, p.Can)
		for _, part := range strings.Split(provider.ResultPath, ".") {
			if part != "" && readable != nil {
				readable = readable.Properties[part]
			}
		}
		if readable != nil && readable.Type == "array" {
			readable = readable.Items
		}
	}
	page.Fields = projectFields(page.Fields, p, readable)
	page.Table.Columns = projectFields(page.Table.Columns, p, readable)
	for i := range page.Forms {
		page.Forms[i].Create = m.projectCommand(page.Forms[i].Create, p)
		page.Forms[i].Edit = m.projectCommand(page.Forms[i].Edit, p)
	}
}

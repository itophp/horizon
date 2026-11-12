package applicationruntime

import (
	"encoding/json"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
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

package applicationruntime

import (
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"
	"encoding/json"
	"slices"
	"sort"
)

// This catalog describes trusted deployed contracts, not an execution grant.
// Installation, channel and current authorization are checked again by Gateway.
func (m *Module) designContracts(p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	if !p.CanPermanent("applications:design") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	definitions := map[string]any{}
	entities := map[string]bool{}
	for _, d := range m.Definitions {
		if !p.CanPermanent(d.Permission) || slices.Contains(d.Tags, "internal-only") || slices.Contains(d.Tags, "system-only") || slices.Contains(d.Tags, "manual-only") {
			continue
		}
		definitions[d.Key()] = map[string]any{"name": d.Name, "version": d.Version, "entity": d.Entity, "permission": d.Permission, "readOnly": d.ReadOnly, "risk": d.Risk, "input": d.Input, "output": d.Output}
		entities[d.Entity] = true
		for _, entity := range d.Entities {
			entities[entity] = true
		}
	}
	items := map[string]any{}
	switch in["kind"] {
	case "capability":
		items = definitions
	case "entity":
		for _, entity := range m.Entities {
			if !entities[entity.Name] {
				continue
			}
			fields := []map[string]any{}
			for _, field := range entity.Attributes {
				if !field.Sensitive {
					fields = append(fields, map[string]any{"name": field.Name, "type": field.Type, "kind": field.Kind, "required": field.Required, "enum": field.Enum})
				}
			}
			items[entity.Name] = map[string]any{"name": entity.Name, "key": entity.Key, "fields": fields}
		}
	case "component":
		for _, name := range ar.CoreFieldComponents() {
			items[name+"@2.0"] = map[string]any{"name": name, "version": "2.0", "source": "core", "configuration": "Runtime FieldV2"}
		}
		for _, app := range m.contributions() {
			if !p.CanPermanent(app.Permission) {
				continue
			}
			for _, component := range app.Components {
				allowed := true
				for _, permission := range component.Permissions {
					allowed = allowed && p.CanPermanent(permission)
				}
				if allowed {
					items[component.Source+":"+component.Name+"@"+component.Version] = component
				}
			}
		}
	}
	if name, ok := in["name"].(string); ok {
		item, exists := items[name]
		if !exists {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		raw, err := json.Marshal(item)
		if err != nil || len(raw) > 20000 {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		return contracts.Output{Data: map[string]any{"contract": item}}, nil
	}
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	offset := min(int(inputInt(in["offset"])), len(names))
	end := min(offset+50, len(names))
	return contracts.Output{Data: map[string]any{"names": names[offset:end], "nextOffset": end, "hasMore": end < len(names)}}, nil
}

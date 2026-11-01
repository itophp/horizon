// Package metadata derives entity metadata from the capability catalog. It is
// the machine readable answer to "what business objects exist and what can be
// done with them", shared by the console, the agent context and future UI
// generators.
package metadata

import (
	"sort"

	"baseline/services/api/internal/platform/capability/contracts"
)

// Build groups definitions into entities with their operations and events.
// A capability contributes to every entity it declares, not only the primary
// one, so an object that is written as a side effect still lists the capability.
func Build(definitions []contracts.Definition) []contracts.Entity {
	index := map[string]*contracts.Entity{}
	for _, definition := range definitions {
		for _, name := range definition.EntityNames() {
			if name == "" {
				continue
			}
			entity, ok := index[name]
			if !ok {
				entity = &contracts.Entity{
					Name:        name,
					Domain:      definition.Domain,
					Description: name,
					Key:         "id",
				}
				index[name] = entity
			}
			entity.Capabilities = append(entity.Capabilities, definition.Name)
			for _, event := range definition.SuccessEvents {
				if !contains(entity.Events, event) {
					entity.Events = append(entity.Events, event)
				}
			}
		}
	}
	out := make([]contracts.Entity, 0, len(index))
	for _, entity := range index {
		sort.Strings(entity.Capabilities)
		sort.Strings(entity.Events)
		out = append(out, *entity)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

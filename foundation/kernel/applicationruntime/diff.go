package applicationruntime

import (
	"reflect"
	"sort"
)

// DefinitionImpacts describes changed declarations without publishing their values
// into history, Outbox or tracing. Nested actions/forms have stable page-scoped IDs.
func DefinitionImpacts(base, candidate ApplicationV2) []DefinitionImpact {
	out := []DefinitionImpact{}
	compare := func(section string, before, after map[string]any) {
		ids := map[string]bool{}
		for id := range before {
			ids[id] = true
		}
		for id := range after {
			ids[id] = true
		}
		for id := range ids {
			left, was := before[id]
			right, now := after[id]
			if reflect.DeepEqual(left, right) {
				continue
			}
			kind := "updated"
			if !was {
				kind = "added"
			} else if !now {
				kind = "removed"
			}
			out = append(out, DefinitionImpact{Section: section, ID: id, Change: kind})
		}
	}
	index := func(app ApplicationV2) map[string]map[string]any {
		sections := map[string]map[string]any{}
		put := func(section, id string, value any) {
			if sections[section] == nil {
				sections[section] = map[string]any{}
			}
			sections[section][id] = value
		}
		pages := []PageV3{}
		providers := []Provider{}
		navigation := []Navigation{}
		if app.Protocol != nil {
			pages = app.Protocol.Pages
			providers = app.Protocol.Providers
			navigation = app.Protocol.Navigation
		}
		for _, page := range pages {
			put("pages", page.ID, page)
		}
		for _, provider := range providers {
			put("providers", provider.ID, provider)
		}
		for _, nav := range navigation {
			put("navigation", nav.PageID, nav)
		}
		for _, message := range app.Messages {
			put("messages", message.Key, message)
		}
		for _, theme := range app.Theme {
			put("theme", theme.Name, theme)
		}
		// Ordering is itself meaningful even when all declaration bodies are equal.
		order := []string{}
		for _, nav := range navigation {
			order = append(order, nav.PageID)
		}
		put("navigation", "$order", order)
		return sections
	}
	left, right := index(Canonical(base)), index(Canonical(candidate))
	for _, section := range []string{"pages", "providers", "navigation", "messages", "theme"} {
		compare(section, left[section], right[section])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Section == out[j].Section {
			return out[i].ID < out[j].ID
		}
		return out[i].Section < out[j].Section
	})
	return out
}

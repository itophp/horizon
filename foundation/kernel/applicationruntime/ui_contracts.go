package applicationruntime

import "encoding/json"

type UIContract struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	PropsSchema string   `json:"propsSchema"`
	Slots       []string `json:"slots"`
	Events      []string `json:"events"`
	Binding     string   `json:"binding"`
	ReadOnly    string   `json:"readOnly"`
	Example     string   `json:"example"`
}

func UIContracts() []UIContract {
	layoutProps := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"columns": map[string]any{"type": "string"},
			"rows":    map[string]any{"type": "string"},
			"gap":     map[string]any{"type": "string"},
			"areas":   map[string]any{"type": "string"},
			"justify": map[string]any{"type": "string", "enum": []string{"start", "center", "end", "stretch"}},
			"align":   map[string]any{"type": "string", "enum": []string{"start", "center", "end", "stretch"}},
			"padding": map[string]any{"type": "string"},
			"width":   map[string]any{"type": "string", "enum": []string{"full", "reading", "form", "auto"}},
			"responsive": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sm": map[string]any{"type": "object"},
					"md": map[string]any{"type": "object"},
					"lg": map[string]any{"type": "object"},
					"xl": map[string]any{"type": "object"},
				},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
	objectProps := map[string]any{"type": "object", "additionalProperties": true}
	marshal := func(value any) string {
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	result := []UIContract{}
	add := func(name, category, binding string, events []string, schema string) {
		if events == nil {
			events = []string{}
		}
		result = append(result, UIContract{
			Name: name, Version: "1.0", Category: category,
			Description: "Trusted ApplicationRuntime protocol " + category + " contract.",
			PropsSchema: schema, Events: events, Binding: binding,
			Slots:    []string{},
			ReadOnly: "Preserve presentation; Gateway rechecks every provider and action.",
			Example:  `{"type":"` + category + `","id":"` + name + `","` + category + `":{"kind":"` + name + `","version":"1.0"}}`,
		})
	}
	for _, name := range []string{"grid", "stack", "sidebar", "tabs", "split"} {
		add(name, "layout", "", nil, marshal(layoutProps))
	}
	for _, item := range []struct {
		name   string
		events []string
	}{
		{"page-heading", nil}, {"search", []string{"change"}}, {"data-table", nil},
		{"text", nil}, {"table-panel", nil}, {"tree-table-panel", nil},
		{"card-grid-panel", nil}, {"master-detail-panel", nil}, {"page-tabs", nil},
		{"button", []string{"click", "action"}}, {"markdown", nil}, {"detail-panel", nil},
		{"record-form-panel", nil},
		{"search-picker-panel", nil}, {"item-list", []string{"action"}},
		{"content-heading", []string{"action"}}, {"content-fields", nil}, {"content-notice", []string{"dismiss"}},
		{"content-table", []string{"action"}}, {"inline-input", []string{"submit"}},
		{"metric-value", nil}, {"time-series", nil}, {"horizontal-bars", nil}, {"distribution-pie", nil},
	} {
		add(item.name, "element", "data", item.events, marshal(objectProps))
	}
	stringProp := map[string]any{"type": "string"}
	object := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	list := func(items map[string]any) map[string]any {
		return map[string]any{"type": "array", "items": items}
	}
	changeField := object(map[string]any{
		"key": stringProp, "group": stringProp, "labelKey": stringProp,
		"type":    map[string]any{"type": "string", "enum": []string{"text", "integer", "select"}},
		"options": list(object(map[string]any{"value": stringProp, "labelKey": stringProp}, "value", "labelKey")),
	}, "key", "group", "labelKey", "type")
	add("change-list-form", "element", "data", []string{"submit"}, marshal(object(map[string]any{
		"fields":            list(changeField),
		"groups":            list(object(map[string]any{"id": stringProp, "labelKey": stringProp}, "id", "labelKey")),
		"operationLabelKey": stringProp, "resetLabelKey": stringProp, "setLabelKey": stringProp,
		"saveLabelKey": stringProp, "noChangesKey": stringProp, "successKey": stringProp,
	}, "fields", "operationLabelKey", "resetLabelKey", "setLabelKey", "saveLabelKey", "noChangesKey", "successKey")))
	add("settings-panel", "element", "", nil, marshal(map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"personal": map[string]any{"type": "boolean"}},
	}))
	add("webmcp-panel", "element", "", nil, marshal(map[string]any{
		"type": "object", "additionalProperties": false,
	}))
	for _, name := range []string{"dictionary-panel", "remote-mcp-panel"} {
		add(name, "element", "data", []string{"refresh"}, marshal(map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"table": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{"columns": map[string]any{
					"type": "array", "items": objectProps,
				}},
			}},
		}))
	}
	for _, name := range []string{"navigate", "invoke", "setData", "refresh", "system"} {
		add(name, "interaction", "event", []string{name}, marshal(objectProps))
	}
	add("card", "element", "data", nil, marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"className":      map[string]any{"type": "string", "pattern": "^[A-Za-z0-9 _-]{0,160}$"},
			"titleKey":       map[string]any{"type": "string"},
			"descriptionKey": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}))
	result[len(result)-1].Slots = []string{"content"}
	add("dialog", "element", "", []string{"close"}, marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"open":           map[string]any{"type": "boolean"},
			"titleKey":       map[string]any{"type": "string"},
			"descriptionKey": map[string]any{"type": "string"},
			"wide":           map[string]any{"type": "boolean"},
		},
		"required":             []string{"titleKey"},
		"additionalProperties": false,
	}))
	result[len(result)-1].Slots = []string{"content"}
	for _, name := range []string{"layout-tabs", "layout-tab-panel"} {
		add(name, "element", "data", nil, marshal(objectProps))
		result[len(result)-1].Slots = []string{"content"}
	}
	return result
}

var trustedUIContracts = func() map[string]UIContract {
	result := map[string]UIContract{}
	for _, item := range UIContracts() {
		if item.Category == "element" {
			result[item.Name+"@"+item.Version] = item
		}
	}
	return result
}()

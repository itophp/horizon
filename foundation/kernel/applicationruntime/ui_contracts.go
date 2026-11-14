package applicationruntime

import (
	"encoding/json"
	"horizon/foundation/kernel/capability/contracts"
)

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

const batchImportTemplatePathPattern = `^/([A-Za-z0-9_-]+/)*[A-Za-z0-9_-]+[.]xlsx$`

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
		{"page-heading", nil}, {"search", []string{"change"}},
		{"text", nil}, {"table-panel", nil}, {"tree-table-panel", nil},
		{"card-grid-panel", nil}, {"master-detail-panel", nil}, {"page-tabs", nil},
		{"markdown", nil}, {"detail-panel", nil},
		{"record-form-panel", nil},
		{"search-picker-panel", nil}, {"item-list", []string{"action"}},
		{"content-heading", []string{"action"}}, {"content-fields", nil}, {"content-notice", []string{"dismiss"}},
		{"content-table", []string{"action"}}, {"inline-input", []string{"submit"}},
		{"metric-value", nil}, {"time-series", nil}, {"horizontal-bars", nil}, {"distribution-pie", nil},
	} {
		add(item.name, "element", "data", item.events, marshal(objectProps))
	}
	add("ai-analysis-panel", "element", "data", nil, marshal(contracts.Object(map[string]*contracts.Schema{"scenarios": {Type: "array", Items: aiScenarioSchema()}}, "scenarios")))
	add("ai-result", "element", "data", nil, marshal(contracts.Object(map[string]*contracts.Schema{"variant": contracts.Enum("compact", "detail")})))
	stringProp := map[string]any{"type": "string"}
	object := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	list := func(items map[string]any) map[string]any {
		return map[string]any{"type": "array", "items": items}
	}
	booleanProp := map[string]any{"type": "boolean"}
	enum := func(values ...string) map[string]any { return map[string]any{"type": "string", "enum": values} }
	action := object(map[string]any{
		"id": stringProp, "labelKey": stringProp, "pendingKey": stringProp,
		"iconOnly": booleanProp, "payloadLabelKey": stringProp, "className": stringProp, "icon": stringProp,
		"linkArrow": booleanProp, "permission": stringProp, "visible": objectProps, "disabled": objectProps, "href": objectProps,
	}, "id", "labelKey")
	add("button", "element", "data", []string{"click", "action"}, marshal(object(map[string]any{
		"labelKey": stringProp, "labelArgs": objectProps, "disabled": booleanProp, "hideInPreview": booleanProp,
		"variant": enum("default", "outline", "secondary", "ghost", "destructive", "link"),
		"size":    enum("default", "xs", "sm", "lg"), "payload": map[string]any{}, "action": action,
	})))
	result[len(result)-1].Example = `{"type":"component","component":{"id":"refresh","kind":"button","version":"1.0","props":{"labelKey":"runtime:refresh","variant":"outline"}}}`
	result[len(result)-1].Description = "Translated button. click emits payload. An action descriptor emits action with the action ID and current record."
	column := object(map[string]any{
		"name": stringProp, "labelKey": stringProp, "display": enum("text", "entity", "mono", "status", "date", "badges"),
		"width": map[string]any{"type": "number"}, "className": stringProp,
	}, "name")
	add("batch-import", "element", "", []string{"parse", "extract", "submit", "cancel", "success"}, marshal(object(map[string]any{
		"fields": list(objectProps), "uniqueField": stringProp,
		"templatePath":  map[string]any{"type": "string", "pattern": batchImportTemplatePathPattern},
		"headers":       map[string]any{"type": "object", "additionalProperties": stringProp},
		"maxRows":       map[string]any{"type": "integer", "minimum": 1},
		"maxFileBytes":  map[string]any{"type": "integer", "minimum": 1},
		"commitHintKey": stringProp, "conflictKey": stringProp,
	}, "fields", "templatePath")))
	result[len(result)-1].Description = "Download an Excel template and review and submit workbook drafts. The application supplies field definitions, optional header-to-field mappings and capability event bindings."
	result[len(result)-1].Example = `{"type":"component","component":{"id":"import","kind":"batch-import","version":"1.0","props":{"fields":[],"templatePath":"/templates/import.xlsx"}}}`
	add("data-table", "element", "data", []string{"rowClick"}, marshal(object(map[string]any{
		"items": list(objectProps), "columns": list(column), "keyField": stringProp, "selectable": booleanProp,
		"serverPage": booleanProp, "rowEvent": stringProp,
	}, "columns")))
	result[len(result)-1].Example = `{"type":"component","component":{"id":"rows","kind":"data-table","version":"1.0","props":{"columns":[{"name":"name","labelKey":"common:echo.name"}],"rowEvent":"rowClick"}}}`
	result[len(result)-1].Description = "Rows come from data (array or {items}) or items. Set rowEvent to rowClick and bind rowClick to receive the selected record. serverPage uses externally paged data."
	changeField := object(map[string]any{
		"key": stringProp, "group": stringProp, "labelKey": stringProp,
		"type":         map[string]any{"type": "string", "enum": []string{"text", "integer", "select", "json", "password"}},
		"options":      list(object(map[string]any{"value": stringProp, "labelKey": stringProp, "label": stringProp}, "value")),
		"optionRows":   object(map[string]any{"source": stringProp, "valuePath": stringProp, "labelPath": stringProp}, "source", "valuePath", "labelPath"),
		"defaultValue": stringProp, "clearable": map[string]any{"type": "boolean"},
		"min": map[string]any{"type": "number"}, "max": map[string]any{"type": "number"},
	}, "key", "group", "labelKey", "type")
	add("change-list-form", "element", "data", []string{"submit"}, marshal(object(map[string]any{
		"fields":            list(changeField),
		"groups":            list(object(map[string]any{"id": stringProp, "labelKey": stringProp}, "id", "labelKey")),
		"operationLabelKey": stringProp, "resetLabelKey": stringProp, "setLabelKey": stringProp,
		"saveLabelKey": stringProp, "noChangesKey": stringProp, "successKey": stringProp,
		"entriesPath": stringProp, "reasonKey": stringProp, "invalidKey": stringProp,
		"configuredKey": stringProp, "secretHintKey": stringProp, "effectiveKey": stringProp, "clearLabelKey": stringProp,
		"optionData": map[string]any{"type": "object"}, "disabled": map[string]any{"type": "boolean"},
	}, "fields", "operationLabelKey", "resetLabelKey", "setLabelKey", "saveLabelKey", "noChangesKey", "successKey")))
	numberProp := map[string]any{"type": "number"}
	actionField := object(map[string]any{
		"name": stringProp, "labelKey": stringProp, "invalidKey": stringProp,
		"format":   map[string]any{"type": "string", "enum": []string{"hex"}},
		"type":     map[string]any{"type": "string", "enum": []string{"text", "integer", "password", "select", "datetime-local", "boolean", "records"}},
		"required": map[string]any{"type": "boolean"},
		"min":      numberProp, "max": numberProp, "minLength": numberProp, "maxLength": numberProp,
		"defaultValue": map[string]any{"type": []string{"string", "number", "boolean"}},
		"timeZoneKey":  stringProp, "defaultOffsetMinutes": numberProp,
		"pattern": stringProp, "valuePath": stringProp, "visibleWhen": map[string]any{"type": "object"},
		"locked": map[string]any{"type": "boolean"}, "trim": map[string]any{"type": "boolean"},
		"rowFields": list(map[string]any{"type": "object"}), "rowStablePath": stringProp,
		"rowTitlePath": stringProp, "rowSubtitlePath": stringProp, "addKey": stringProp, "removeKey": stringProp, "emptyKey": stringProp,
		"lockStable": map[string]any{"type": "boolean"}, "allowAdd": map[string]any{"type": "boolean"},
		"options": list(object(map[string]any{
			"value":    map[string]any{"type": []string{"string", "number"}},
			"labelKey": stringProp, "label": stringProp, "disabled": map[string]any{"type": "boolean"},
		}, "value")),
	}, "name", "labelKey", "type")
	add("action-form", "element", "", []string{"submit", "success", "cancel"}, marshal(object(map[string]any{
		"fields": list(actionField), "submitKey": stringProp, "cancelKey": stringProp, "errorPrefix": stringProp,
		"disabled":        map[string]any{"type": "boolean"},
		"browserDefaults": map[string]any{"type": "string", "enum": []string{"webmcp-pairing"}},
		"prefilledKey":    stringProp,
		"initialValues":   map[string]any{"type": "object"}, "instanceKey": stringProp,
		"showInPreview": map[string]any{"type": "boolean"}, "successKey": stringProp,
	}, "fields", "submitKey")))
	add("file-review-form", "element", "", []string{"review", "submit", "success", "cancel"}, marshal(object(map[string]any{
		"formats": list(stringProp), "maxBytes": numberProp, "fileKey": stringProp,
		"reviewKey": stringProp, "submitKey": stringProp, "cancelKey": stringProp, "reasonKey": stringProp,
		"invalidFileKey": stringProp, "invalidReasonKey": stringProp, "reviewAccepted": map[string]any{"type": "boolean"},
	}, "formats", "maxBytes", "fileKey", "reviewKey", "submitKey", "cancelKey", "reasonKey", "invalidFileKey", "invalidReasonKey")))
	result[len(result)-1].Slots = []string{"content"}
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
	for _, name := range []string{"navigate", "invoke", "setData", "refresh", "system", "browser"} {
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
	add("dialog", "element", "", []string{"close", "openChange"}, marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"open":            map[string]any{"type": "boolean"},
			"titleKey":        map[string]any{"type": "string"},
			"titleArgs":       map[string]any{"type": "object"},
			"descriptionArgs": map[string]any{"type": "object"},
			"descriptionKey":  map[string]any{"type": "string"},
			"wide":            map[string]any{"type": "boolean"},
		},
		"required":             []string{"titleKey"},
		"additionalProperties": false,
	}))
	result[len(result)-1].Slots = []string{"content"}
	result[len(result)-1].Description = "Externally controlled dialog. A user dismissal emits close and openChange with false."
	for _, name := range []string{"layout-tabs", "layout-tab-panel"} {
		add(name, "element", "data", nil, marshal(objectProps))
		result[len(result)-1].Slots = []string{"content"}
	}
	return appendShadcnUIContracts(result)
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

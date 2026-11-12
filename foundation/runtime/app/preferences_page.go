package app

import (
	"encoding/json"
	"strings"

	ar "horizon/foundation/kernel/applicationruntime"
)

func personalPreferencesApplication() ar.ApplicationV2 {
	const dataID = "effective-preferences"
	field := func(key, kind string, options ...viewProps) viewProps {
		group, _, _ := strings.Cut(key, ".")
		result := viewProps{
			"key": key, "group": group, "type": kind,
			"labelKey": "governance:labels." + strings.ReplaceAll(key, ".", "_"),
		}
		if len(options) > 0 {
			result["options"] = options
		}
		return result
	}
	choices := func(values ...string) []viewProps {
		options := make([]viewProps, 0, len(values))
		for _, value := range values {
			encoded, _ := json.Marshal(value)
			options = append(options, viewProps{"value": string(encoded), "labelKey": "governance:choices." + value})
		}
		return options
	}
	toggle := func(key string) viewProps {
		return field(key, "select",
			viewProps{"value": "true", "labelKey": "governance:on"},
			viewProps{"value": "false", "labelKey": "governance:off"})
	}
	form := componentNode("preferences-editor", "change-list-form", "$"+dataID, viewProps{
		"groups": []viewProps{
			{"id": "display", "labelKey": "governance:groups.display"},
			{"id": "notifications", "labelKey": "governance:groups.notifications"},
		},
		"fields": []viewProps{
			field("display.language", "select", choices("en", "zh-CN")...),
			field("display.theme", "select", choices("dark", "light")...),
			field("display.timezone", "text"),
			field("display.dateStyle", "select", choices("short", "medium", "long")...),
			toggle("display.hour24"), toggle("display.grouping"),
			field("display.pageSize", "integer"),
			toggle("notifications.inApp"), toggle("notifications.email"),
		},
		"operationLabelKey": "governance:fieldOperation",
		"resetLabelKey":     "governance:useSystemDefault",
		"setLabelKey":       "governance:set",
		"saveLabelKey":      "governance:save",
		"noChangesKey":      "governance:noChanges",
		"successKey":        "governance:saved",
	}, ar.EventBinding{
		Event: "submit", Action: "invoke", Target: "settings.preferences.update@1.0.0",
		Payload: objectValue(mapping("changes", fieldPath("event.changes"))),
	})
	form.Layout.Data = []ar.DataSpec{{ID: dataID, Provider: dataID, NonBlocking: true}}
	return platformPageVersion("personal-preferences", "/preferences", "governance:preferences", "governance:preferencesHint", "1.0.1",
		[]ar.Provider{{ID: dataID, Capability: ar.Ref("settings.effective.read"), KeyField: "key", Pagination: "none", ResultPath: "entries"}}, form)
}

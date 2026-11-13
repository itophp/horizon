package app

import (
	"encoding/json"
	"slices"
	"strings"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/governance"
)

const workspaceSettingsData = "_workspaceSettings"
const workspaceSchedulesData = "_workspaceSchedules"
const workspaceLogoData = "_workspaceLogos"

func workspaceTarget() ar.Expression {
	return objectValue(mapping("kind", constant("workspace")), mapping("id", constant(0)))
}

func workspaceInputs() []ar.Mapping {
	return []ar.Mapping{mapping("targetKind", constant("workspace")), mapping("targetId", constant(0))}
}

func workspaceSettingsProviders() []ar.Provider {
	return []ar.Provider{
		{ID: "workspace-settings", Capability: ar.Ref("settings.workspace.read"), KeyField: "revision", Pagination: "none", Input: workspaceInputs()},
		{ID: "workspace-schedules", Capability: ar.Ref("settings.schedule.list"), KeyField: "id", Pagination: "none"},
		{ID: "workspace-logos", Capability: ar.Ref("file.list.read"), KeyField: "id", Pagination: "none"},
	}
}

func workspaceSettingsEditor(active ar.Expression) ar.Node {
	groups := []viewProps{}
	fields := []viewProps{}
	seen := map[string]bool{}
	for _, definition := range governance.Registry {
		if !slices.Contains(definition.Scopes, "workspace") {
			continue
		}
		if !seen[definition.Group] {
			groups = append(groups, viewProps{"id": definition.Group, "labelKey": "governance:groups." + definition.Group})
			seen[definition.Group] = true
		}
		fields = append(fields, workspaceSettingField(definition))
	}
	editAllowed := permits("settings:update")
	editor := componentNode("workspace-config-editor", "change-list-form", "$"+workspaceSettingsData, viewProps{
		"fields": fields, "groups": groups, "entriesPath": "entries",
		"operationLabelKey": "governance:fieldOperation", "resetLabelKey": "governance:resetDefault",
		"setLabelKey": "governance:set", "saveLabelKey": "governance:draft", "clearLabelKey": "governance:clear",
		"noChangesKey": "governance:noChanges", "successKey": "governance:saved", "invalidKey": "governance:invalid",
		"configuredKey": "governance:configured", "secretHintKey": "governance:secretHint",
		"effectiveKey": "governance:effective", "reasonKey": "governance:reason",
	}, ar.EventBinding{
		Event: "submit", Action: "invoke", Target: "settings.draft.create@1.0.0", When: &editAllowed,
		Payload: objectValue(
			mapping("target", workspaceTarget()),
			mapping("baseRevision", fieldPath("data."+workspaceSettingsData+".revision")),
			mapping("changes", fieldPath("event.changes")),
			mapping("reason", fieldPath("event.reason")),
		),
	})
	componentOf(editor).Bindings = []ar.Mapping{
		mapping("disabled", operation("not", editAllowed)),
		mapping("optionData", objectValue(mapping("brandingLogo", operation("if", permits("files:read"),
			operation("filter", fieldPath("data."+workspaceLogoData), same("record.purpose", "logo")),
			constant(nil),
		)))),
	}
	editor.Layout.Data = []ar.DataSpec{{
		ID: workspaceLogoData, Provider: "workspace-logos", EnabledWhen: &active, NonBlocking: true,
	}}
	return editor
}

func workspaceSettingField(definition governance.Definition) viewProps {
	field := viewProps{
		"key": definition.Key, "group": definition.Group, "defaultValue": definition.Default,
		"labelKey": "governance:labels." + strings.ReplaceAll(definition.Key, ".", "_"),
	}
	switch {
	case definition.Key == "branding.logo":
		field["type"] = "select"
		field["options"] = []viewProps{{"value": "0", "labelKey": "governance:noLogo"}}
		field["optionRows"] = viewProps{"source": "brandingLogo", "valuePath": "id", "labelPath": "name"}
	case definition.Secret:
		field["type"], field["clearable"] = "password", true
	case definition.Kind == "string":
		field["type"], field["clearable"] = "text", true
	case definition.Kind == "integer":
		field["type"], field["min"], field["max"] = "integer", definition.Min, definition.Max
	case definition.Kind == "boolean":
		field["type"] = "select"
		field["options"] = []viewProps{
			{"value": "true", "labelKey": "governance:on"},
			{"value": "false", "labelKey": "governance:off"},
		}
	case definition.Kind == "enum":
		options := []viewProps{}
		for _, choice := range definition.Choices {
			value, _ := json.Marshal(choice)
			options = append(options, viewProps{"value": string(value), "labelKey": "governance:choices." + choice})
		}
		field["type"], field["options"] = "select", options
	default:
		field["type"] = "json"
	}
	return field
}

const workspaceActionData = "_workspaceAction"

func workspaceSettingsContent(active ar.Expression) ar.Node {
	revision := componentNode("workspace-settings-revision", "text", "$"+workspaceSettingsData, viewProps{"textKey": "governance:revision", "className": "muted-text"})
	componentOf(revision).Bindings = []ar.Mapping{mapping("values", objectValue(
		mapping("version", fieldPath("record.revision")),
	))}
	exportAllowed := permits("settings:export")
	export := componentNode("workspace-settings-export", "button", "", viewProps{
		"labelKey": "governance:export", "variant": "outline", "hideInPreview": true,
	}, ar.EventBinding{
		Event: "click", Action: "invoke", Target: "settings.export.read@1.0.0", Download: &ar.DownloadResult{Filename: "settings.json"},
		Payload: objectValue(workspaceInputs()...),
	})
	componentOf(export).Visible = &exportAllowed
	toolbar := layoutNode("workspace-settings-toolbar", ar.LayoutProps{
		ClassName: "gov-toolbar", Columns: "1fr max-content", Gap: "8px",
	}, revision, export)
	root := layoutNode("workspace-config-content", ar.LayoutProps{Gap: "16px"},
		toolbar, workspaceSettingsEditor(active), workspaceDrafts(), workspaceHistory(),
		workspaceScheduleQueue(), workspaceActionDialog("publish"), workspaceActionDialog("rollback"),
		workspaceActionDialog("schedule"),
	)
	allowed := permits("settings:read")
	root.Layout.Visible = &allowed
	root.Layout.Data = []ar.DataSpec{
		{ID: workspaceSettingsData, Provider: "workspace-settings", EnabledWhen: &active, NonBlocking: true, PollMs: 5000},
		{ID: workspaceSchedulesData, Provider: "workspace-schedules", EnabledWhen: &active, NonBlocking: true, PollMs: 5000},
		{ID: workspaceActionData},
	}
	return root
}

func workspaceDrafts() ar.Node {
	pending := operation("exists", operation("find", fieldPath("data."+workspaceSchedulesData),
		operation("and", same("record.status", "pending"), operation("eq", fieldPath("record.draftId"), fieldPath("event.id"))),
	))
	table := workspaceChangeTable("drafts", "baseRevision", []viewProps{
		{"id": "publish", "labelKey": "governance:publish", "permission": "settings:publish", "disabled": pending, "className": "ai-button is-ghost"},
		{"id": "schedule", "labelKey": "governance:schedulePublish", "permission": "settings:publish", "disabled": pending, "className": "ai-button is-ghost"},
	}, workspaceSelectAction("publish"), workspaceSelectAction("schedule"))
	return browserCard("workspace-drafts", "governance:drafts", "", table)
}

func workspaceHistory() ar.Node {
	table := workspaceChangeTable("history", "revision", []viewProps{
		{"id": "rollback", "labelKey": "governance:rollback", "permission": "settings:rollback", "className": "ai-button is-ghost"},
	}, workspaceSelectAction("rollback"))
	return browserCard("workspace-history", "governance:history", "", table)
}

func workspaceChangeTable(path, revisionField string, actions []viewProps, events ...ar.EventBinding) ar.Node {
	return componentNode("workspace-"+path+"-table", "content-table", "$"+workspaceSettingsData, viewProps{
		"rowsPath": path, "emptyKey": "governance:empty",
		"columns": []viewProps{
			{"labelKey": "governance:reason", "fields": []viewProps{
				{"path": "reason", "format": "strong"},
				{"path": "changes", "format": "details", "labelKey": "governance:difference", "itemFields": []viewProps{
					{"value": operation("replace", fieldPath("record.key"), constant("."), constant("_")), "labelPrefix": "governance:labels.", "format": "strong"},
					{"path": "operation", "labelPrefix": "governance:"},
					{"path": "value", "format": "code"},
				}},
			}},
			{"labelKey": "governance:history", "fields": []viewProps{plain(revisionField)}},
			{"labelKey": "admin:fields.createdAt", "fields": []viewProps{{"path": "createdAt", "format": "dateTime"}}},
			{"labelKey": "admin:fields.actions", "actions": actions},
		},
	}, events...)
}

func workspaceSelectAction(kind string) ar.EventBinding {
	when := same("event.action", kind)
	base := fieldPath("event.item.baseRevision")
	if kind == "rollback" {
		base = fieldPath("data." + workspaceSettingsData + ".revision")
	}
	return ar.EventBinding{
		Event: "action", Action: "setData", Target: workspaceActionData, When: &when,
		Payload: objectValue(
			mapping("kind", constant(kind)), mapping("id", fieldPath("event.item.id")),
			mapping("baseRevision", base), mapping("sensitive", fieldPath("event.item.sensitive")),
		),
	}
}

func workspaceScheduleQueue() ar.Node {
	cancel := operation("and", same("event.action", "cancel"), permits("settings:publish"), same("event.item.status", "pending"))
	key := fieldPath("event.operationKey")
	table := componentNode("workspace-schedules-table", "content-table", "$"+workspaceSchedulesData, viewProps{
		"emptyKey": "governance:empty",
		"columns": []viewProps{
			{"labelKey": "governance:reason", "fields": []viewProps{plain("reason")}},
			{"labelKey": "governance:scheduledBase", "fields": []viewProps{plain("baseRevision")}},
			{"labelKey": "governance:scheduledAt", "fields": []viewProps{{"path": "scheduledAt", "format": "dateTime"}}},
			{"labelKey": "governance:status", "fields": []viewProps{{"path": "status", "labelPrefix": "governance:scheduleStatuses.", "className": "scope-badge"}}},
			{"labelKey": "governance:scheduleFinished", "fields": []viewProps{{"path": "finishedAt", "format": "dateTime"}}},
			{"labelKey": "governance:scheduleResult", "fields": []viewProps{{"path": "errorCode", "labelPrefix": "governance:scheduleErrors."}}},
			{"labelKey": "governance:cancelSchedule", "actions": []viewProps{
				{"id": "cancel", "labelKey": "governance:cancelSchedule", "permission": "settings:publish",
					"visible": same("record.status", "pending"), "className": "ai-button is-ghost"},
			}},
		},
	}, ar.EventBinding{
		Event: "action", Action: "invoke", Target: "settings.schedule.cancel@1.0.0", When: &cancel,
		IdempotencyKey: &key, Payload: objectValue(mapping("id", fieldPath("event.item.id"))),
	})
	return browserCard("workspace-schedules", "governance:scheduledReleases", "governance:scheduleHint", table)
}

func workspaceActionDialog(kind string) ar.Node {
	capability := "settings.draft.publish"
	title := "governance:publish"
	hint := "governance:publishHint"
	actionPath := "data." + workspaceActionData
	requiresPassword := same(actionPath+".sensitive", true)
	input := []ar.Mapping{
		mapping("id", fieldPath(actionPath+".id")),
		mapping("baseRevision", fieldPath(actionPath+".baseRevision")),
	}
	fields := []ar.Expression{}
	switch kind {
	case "schedule":
		capability, title, hint = "settings.schedule.create", "governance:schedulePublish", "governance:scheduleHint"
		input = append(input, mapping("scheduledAt", fieldPath("event.values.scheduledAt")))
		fields = append(fields, constant(viewProps{
			"name": "scheduledAt", "type": "datetime-local", "labelKey": "governance:scheduledAt",
			"timeZoneKey": "governance:scheduleZone", "invalidKey": "governance:invalid",
			"defaultOffsetMinutes": 60, "required": true,
		}))
	case "rollback":
		capability, title, hint = "settings.history.rollback", "governance:rollback", "governance:reauthHint"
		requiresPassword = constant(true)
		input = append(input, mapping("reason", fieldPath("event.values.reason")))
		fields = append(fields, constant(viewProps{
			"name": "reason", "type": "text", "labelKey": "governance:reason", "required": true, "minLength": 5, "maxLength": 500,
		}))
	}
	password := viewProps{"name": "password", "type": "password", "labelKey": "governance:password", "required": true, "maxLength": 128}
	fields = append(fields, operation("if", requiresPassword, constant(password), constant(nil)))
	key := fieldPath("event.operationKey")
	close := func(event string) ar.EventBinding {
		return ar.EventBinding{Event: event, Action: "setData", Target: workspaceActionData, Payload: constant(nil)}
	}
	form := componentNode("workspace-"+kind+"-form", "action-form", "", viewProps{
		"submitKey": title, "cancelKey": "admin:actions.cancel",
	}, ar.EventBinding{
		Event: "submit", Action: "invoke", Target: capability + "@1.0.0",
		IdempotencyKey: &key, Payload: objectValue(input...),
		Reauthenticate: &ar.Reauthentication{
			Capability: ar.Ref("settings.password.verify"), When: &requiresPassword, Operation: "settings." + kind,
		},
	}, close("success"), close("cancel"))
	componentOf(form).Bindings = []ar.Mapping{
		mapping("fields", operation("filter", operation("array", fields...), operation("exists", fieldPath("record.name")))),
	}
	dialog := componentNode("workspace-"+kind+"-dialog", "dialog", "", viewProps{"titleKey": title, "descriptionKey": hint}, close("close"))
	componentOf(dialog).Bindings = []ar.Mapping{mapping("open", same(actionPath+".kind", kind))}
	componentOf(dialog).Content = layoutNode("workspace-"+kind+"-dialog-content", ar.LayoutProps{}, form).Layout
	return dialog
}

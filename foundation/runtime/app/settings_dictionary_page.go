package app

import (
	"sort"
	"strings"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/identity"
)

const (
	dictionaryListData     = "_dictionaryList"
	dictionaryDetailData   = "_dictionaryDetail"
	dictionaryOptionsData  = "_dictionaryOptions"
	dictionarySelectedData = "_dictionarySelected"
	dictionaryActionData   = "_dictionaryAction"
	dictionaryImportData   = "_dictionaryImport"
)

func workspaceDictionaryProviders() []ar.Provider {
	return []ar.Provider{
		{ID: "dictionary-list", Capability: ar.Ref("dictionary.list.read"), KeyField: "id", Pagination: "none", ResultPath: "definitions", Input: workspaceInputs()},
		{ID: "dictionary-detail", Capability: ar.Ref("dictionary.detail.read"), KeyField: "id", Pagination: "none",
			Input: append(workspaceInputs(), mapping("id", fieldPath("data."+dictionarySelectedData+".id")))},
		{ID: "dictionary-options", Capability: ar.Ref("dictionary.options.read"), KeyField: "id", Pagination: "none",
			Input: []ar.Mapping{
				mapping("code", fieldPath("data."+dictionarySelectedData+".code")),
				mapping("language", operation("if", same("context.locale", "zh-CN"), constant("zh-CN"), constant("en"))),
			}},
	}
}

func workspaceDictionaryContent(active ar.Expression) ar.Node {
	listing := operation("not", operation("exists", fieldPath("data."+dictionarySelectedData+".id")))
	detail := operation("and", active, operation("not", listing))
	canOptions := operation("and", detail, permits("dictionaries:use"), operation("exists", fieldPath("data."+dictionarySelectedData+".code")))
	canCreate := operation("and", permits("dictionaries:create"), permits("dictionaries:update"))
	fields := platformColumn("nameEn", "governance:dictionaries", "entity-link")
	fields["subtitleField"] = "code"
	fields["languageFields"] = viewProps{"zh-CN": "nameZh"}
	kind := platformColumn("kind", "governance:kind", "enumBadge")
	kind["prefixKey"] = "governance:"
	status := platformColumn("status", "governance:status", "enumBadge")
	status["prefixKey"] = "governance:"
	open := ar.EventBinding{Event: "open", Action: "setData", Target: dictionarySelectedData,
		Payload: objectValue(mapping("id", fieldPath("event.id")), mapping("code", fieldPath("event.code")))}
	revisionZero := same("record.revision", 0)
	remove := ar.EventBinding{
		Event: "remove", Action: "setData", Target: dictionaryActionData,
		Payload: objectValue(mapping("kind", constant("remove")), mapping("id", fieldPath("event.id"))),
	}
	list := componentNode("workspace-dictionary-table", "table-panel", "$"+dictionaryListData, viewProps{
		"searchFields": []string{"code", "nameEn", "nameZh", "category"}, "openEvent": "open",
		"columns": []viewProps{fields, platformColumn("category", "governance:category", ""), kind, status, platformColumn("revision", "governance:history", "")},
		"rowActions": []viewProps{
			{"mode": "event", "event": "open", "labelKey": "governance:edit", "icon": "eye"},
			{"mode": "event", "event": "remove", "labelKey": "governance:remove", "icon": "trash", "permission": "dictionaries:delete", "when": revisionZero},
		},
	}, open, remove)
	create := componentNode("create-dictionary", "button", "", viewProps{"labelKey": "governance:create", "hideInPreview": true},
		ar.EventBinding{Event: "click", Action: "setData", Target: dictionaryActionData, Payload: objectValue(mapping("kind", constant("create")))})
	componentOf(create).Visible = &canCreate
	listPage := layoutNode("dictionary-list-content", ar.LayoutProps{Gap: "16px"},
		browserText("dictionary-list-intro", "governance:dictionariesHint"), create, list,
	)
	listPage.Layout.Visible = &listing

	back := componentNode("dictionary-back", "button", "", viewProps{"labelKey": "governance:dictionaries", "variant": "outline"},
		ar.EventBinding{Event: "click", Action: "setData", Target: dictionarySelectedData, Payload: constant(nil)})
	detailPage := layoutNode("dictionary-detail-content", ar.LayoutProps{Gap: "16px"}, back, dictionaryTools(), dictionaryEditor(false),
		dictionaryOptions(), dictionaryHistory(false), dictionaryHistory(true))
	notListing := operation("not", listing)
	detailPage.Layout.Visible = &notListing

	root := layoutNode("workspace-dictionary-content", ar.LayoutProps{Gap: "16px"}, listPage, detailPage,
		dictionaryCreateDialog(), dictionaryRemoveDialog(), dictionaryImportDialog(),
		dictionaryReleaseDialog(false), dictionaryReleaseDialog(true))
	root.Layout.Data = []ar.DataSpec{
		{ID: dictionaryListData, Provider: "dictionary-list", EnabledWhen: &active, NonBlocking: true},
		{ID: dictionarySelectedData}, {ID: dictionaryActionData}, {ID: dictionaryImportData},
		{ID: dictionaryDetailData, Provider: "dictionary-detail", EnabledWhen: &detail, NonBlocking: true},
		{ID: dictionaryOptionsData, Provider: "dictionary-options", EnabledWhen: &canOptions, NonBlocking: true},
	}
	return root
}

func dictionaryTools() ar.Node {
	record := "data." + dictionaryDetailData + ".document.definition"
	available := operation("and", operation("exists", fieldPath(record+".id")), operation("not", same(record+".kind", "dynamic")))
	refs := componentNode("dictionary-references", "text", "$"+dictionaryDetailData, viewProps{"textKey": "governance:references", "className": "muted-text"})
	componentOf(refs).Bindings = []ar.Mapping{mapping("values", objectValue(mapping("count", fieldPath("record.references"))))}
	importAllowed := operation("and", available, permits("dictionaries:import"))
	button := componentNode("dictionary-import-button", "button", "", viewProps{"labelKey": "governance:import", "variant": "outline", "hideInPreview": true},
		ar.EventBinding{Event: "click", Action: "setData", Target: dictionaryActionData, Payload: objectValue(mapping("kind", constant("import")))})
	componentOf(button).Visible = &importAllowed
	nodes := []ar.Node{refs, button}
	for _, format := range []string{"csv", "json"} {
		exportAllowed := operation("and", available, permits("dictionaries:export"))
		node := componentNode("dictionary-export-"+format, "button", "", viewProps{
			"labelKey": "governance:exportFormat", "labelArgs": viewProps{"format": strings.ToUpper(format)}, "variant": "outline", "hideInPreview": true,
		}, ar.EventBinding{
			Event: "click", Action: "invoke", Target: "dictionary.export.read@1.0.0",
			Payload:  objectValue(mapping("id", fieldPath("data."+dictionarySelectedData+".id")), mapping("format", constant(format))),
			Download: &ar.DownloadResult{Filename: "dictionary." + format, FilenameField: "filename", ContentField: "content"},
		})
		componentOf(node).Visible = &exportAllowed
		nodes = append(nodes, node)
	}
	return layoutNode("dictionary-toolbar", ar.LayoutProps{ClassName: "gov-toolbar", Gap: "8px"}, nodes...)
}

func dictionaryOptions() ar.Node {
	table := componentNode("dictionary-options-table", "content-table", "$"+dictionaryOptionsData, viewProps{
		"emptyKey": "governance:empty",
		"columns": []viewProps{
			{"labelKey": "governance:code", "fields": []viewProps{plain("code")}},
			{"labelKey": "governance:labelEn", "fields": []viewProps{plain("label")}},
			{"labelKey": "governance:parentCode", "fields": []viewProps{plain("parentCode")}},
			{"labelKey": "governance:status", "fields": []viewProps{
				{"when": same("record.disabled", true), "labelKey": "governance:inactive"},
				{"when": same("record.disabled", false), "labelKey": "governance:active"},
			}},
		},
	})
	card := browserCard("dictionary-options-card", "governance:options", "", table)
	allowed := permits("dictionaries:use")
	card.Layout.Visible = &allowed
	return card
}

func dictionaryClose(event string) ar.EventBinding {
	return ar.EventBinding{Event: event, Action: "setData", Target: dictionaryActionData, Payload: constant(nil)}
}

func dictionaryDialog(kind, title, description string, content ar.Node) ar.Node {
	dialog := componentNode("dictionary-"+kind+"-dialog", "dialog", "", viewProps{
		"titleKey": "governance:" + title, "descriptionKey": "governance:" + description, "wide": true,
	}, dictionaryClose("close"))
	componentOf(dialog).Bindings = []ar.Mapping{mapping("open", same("data."+dictionaryActionData+".kind", kind))}
	componentOf(dialog).Content = layoutNode("dictionary-"+kind+"-dialog-content", ar.LayoutProps{}, content).Layout
	return dialog
}

func dictionaryCreateDialog() ar.Node {
	return dictionaryDialog("create", "create", "dictionariesHint", dictionaryEditor(true))
}

func dictionaryRemoveDialog() ar.Node {
	key := fieldPath("event.operationKey")
	form := componentNode("dictionary-remove-form", "action-form", "", viewProps{
		"fields": []viewProps{}, "submitKey": "governance:remove", "cancelKey": "governance:cancel",
	}, ar.EventBinding{Event: "submit", Action: "invoke", Target: "dictionary.delete@1.0.0", IdempotencyKey: &key,
		Payload: objectValue(mapping("id", fieldPath("data."+dictionaryActionData+".id")))},
		dictionaryClose("success"), dictionaryClose("cancel"))
	return dictionaryDialog("remove", "remove", "dictionariesHint", form)
}

func dictionaryEditor(create bool) ar.Node {
	kind := "edit"
	source := "$" + dictionaryDetailData
	record := "data." + dictionaryDetailData + ".document.definition"
	revision := fieldPath(record + ".revision")
	id := fieldPath(record + ".id")
	if create {
		kind, source = "create", ""
		revision, id = constant(0), constant(0)
	}
	defaultFields := []ar.Expression{}
	for _, field := range dictionaryFields() {
		if !create {
			var locked *ar.Expression
			switch field["name"] {
			case "code":
				value := constant(true)
				locked = &value
			case "kind", "provider":
				value := operation("not", same(record+".revision", 0))
				locked = &value
			}
			if locked != nil {
				mappings := []ar.Mapping{mapping("locked", *locked)}
				for name, value := range field {
					mappings = append(mappings, mapping(name, constant(value)))
				}
				// Keep generated definitions stable across executions.
				sort.Slice(mappings, func(i, j int) bool { return mappings[i].Target < mappings[j].Target })
				defaultFields = append(defaultFields, objectValue(mappings...))
				continue
			}
		}
		defaultFields = append(defaultFields, constant(field))
	}
	definition := []ar.Mapping{
		mapping("id", id), mapping("target", workspaceTarget()), mapping("revision", revision),
		mapping("allowExtensions", constant(false)),
	}
	for _, name := range []string{"code", "nameEn", "nameZh", "category", "description", "kind", "provider", "status", "visibility", "consumerPermission"} {
		definition = append(definition, mapping(name, fieldPath("event.values."+name)))
	}
	key := fieldPath("event.operationKey")
	submit := ar.EventBinding{
		Event: "submit", Action: "invoke", Target: "dictionary.draft.create@1.0.0", IdempotencyKey: &key,
		Payload: objectValue(
			mapping("document", objectValue(
				mapping("definition", objectValue(definition...)),
				mapping("items", operation("coalesce", fieldPath("event.values.items"), constant([]any{}))),
			)),
			mapping("baseRevision", revision),
			mapping("reason", fieldPath("event.values.reason")),
		),
	}
	if create {
		submit.ResultTarget = dictionarySelectedData
		result := objectValue(mapping("id", fieldPath("event.data.document.definition.id")), mapping("code", fieldPath("event.data.document.definition.code")))
		submit.Result = &result
	}
	success := ar.EventBinding{Event: "success", Action: "refresh", Target: dictionaryListData}
	props := viewProps{"submitKey": "governance:draft", "showInPreview": !create}
	events := []ar.EventBinding{submit, success}
	if create {
		props["cancelKey"] = "governance:cancel"
		events = append(events, dictionaryClose("success"), dictionaryClose("cancel"))
	} else {
		props["successKey"] = "governance:saved"
	}
	form := componentNode("dictionary-"+kind+"-editor", "action-form", source, props, events...)
	componentOf(form).Bindings = []ar.Mapping{mapping("fields", operation("array", defaultFields...))}
	if !create {
		componentOf(form).Bindings = append(componentOf(form).Bindings,
			mapping("initialValues", objectValue(mapping("definition", fieldPath(record)), mapping("items", fieldPath("data."+dictionaryDetailData+".document.items")))),
			mapping("instanceKey", operation("concat", id, constant(":"), revision)),
			mapping("disabled", operation("or", operation("not", permits("dictionaries:update")),
				operation("not", operation("eq", id, fieldPath("data."+dictionarySelectedData+".id"))))),
		)
	}
	return form
}

func dictionaryFields() []viewProps {
	text := func(name string, value any, required bool) viewProps {
		return viewProps{"name": name, "labelKey": "governance:" + name, "type": "text", "defaultValue": value, "required": required, "trim": required, "valuePath": "definition." + name}
	}
	choices := func(name, value string, options ...string) viewProps {
		choices := []viewProps{}
		for _, option := range options {
			choices = append(choices, viewProps{"value": option, "labelKey": "governance:" + option})
		}
		field := text(name, value, false)
		field["type"], field["options"] = "select", choices
		return field
	}
	code := text("code", "", true)
	code["pattern"], code["maxLength"] = "[A-Za-z0-9][A-Za-z0-9_.-]{0,79}", 80
	provider := choices("provider", "")
	provider["required"] = true
	provider["visibleWhen"] = viewProps{"field": "kind", "equals": "dynamic"}
	provider["options"] = []viewProps{
		{"value": "", "labelKey": "governance:select"},
		{"value": "organizations", "labelKey": "admin:nav.organizations"},
		{"value": "departments", "labelKey": "admin:nav.departments"},
		{"value": "roles", "labelKey": "admin:nav.roles"},
		{"value": "accounts", "labelKey": "admin:nav.accounts"},
	}
	permission := choices("consumerPermission", "")
	permissions := []viewProps{{"value": "", "labelKey": "governance:default"}}
	for _, definition := range identity.PermissionCatalog() {
		permissions = append(permissions, viewProps{"value": definition.Code})
	}
	permission["options"] = permissions
	items := []viewProps{}
	for _, name := range []string{"code", "labelEn", "labelZh", "parentCode", "sort", "startsAt", "endsAt", "color", "icon", "descriptionEn", "descriptionZh", "status", "isDefault", "id"} {
		field := text(name, "", name == "code" || name == "labelEn")
		if name == "code" {
			field["pattern"], field["lockStable"] = "[A-Za-z0-9_.-]{1,80}", true
		}
		switch name {
		case "status":
			field = choices(name, "active", "active", "inactive", "archived")
		case "sort", "id":
			field["type"], field["defaultValue"] = "integer", 0
			if name == "id" {
				field["hidden"] = true
			}
		case "isDefault":
			field["type"], field["defaultValue"] = "boolean", false
		case "descriptionEn":
			field["labelKey"] = "governance:nameEn"
		case "descriptionZh":
			field["labelKey"] = "governance:nameZh"
		}
		delete(field, "valuePath")
		items = append(items, field)
	}
	reason := text("reason", "", true)
	reason["minLength"], reason["maxLength"] = 5, 500
	return []viewProps{code, text("nameEn", "", true), text("nameZh", "", false), text("category", "general", false),
		choices("kind", "flat", "flat", "tree", "dynamic"), choices("status", "active", "active", "inactive", "archived"),
		provider, choices("visibility", "shared", "shared", "management"), permission, text("description", "", false),
		{
			"name": "items", "type": "records", "labelKey": "governance:items", "rowFields": items,
			"rowStablePath": "id", "rowTitlePath": "code", "rowSubtitlePath": "labelEn",
			"addKey": "governance:addItem", "removeKey": "governance:remove", "emptyKey": "governance:empty",
			"visibleWhen": viewProps{"field": "kind", "equals": "dynamic", "invert": true},
		}, reason}
}

func dictionaryHistory(history bool) ar.Node {
	path, kind, permission, revision := "drafts", "publish", "dictionaries:publish", "baseRevision"
	if history {
		path, kind, permission, revision = "history", "rollback", "dictionaries:rollback", "revision"
	}
	base := fieldPath("event.item.baseRevision")
	if history {
		base = operation("if", same("event.item.domain", "override"),
			fieldPath("data."+dictionaryDetailData+".overrideRevision"),
			fieldPath("data."+dictionaryDetailData+".document.definition.revision"))
	}
	when := same("event.action", kind)
	table := componentNode("dictionary-"+path+"-table", "content-table", "$"+dictionaryDetailData, viewProps{
		"rowsPath": path, "emptyKey": "governance:empty",
		"columns": []viewProps{
			{"labelKey": "governance:reason", "fields": []viewProps{
				{"path": "reason", "format": "strong"},
				{"path": "document.items", "format": "details", "labelKey": "governance:difference", "itemFields": []viewProps{
					{"path": "code", "format": "code"}, plain("labelEn"), plain("labelZh"), {"path": "status", "labelPrefix": "governance:"},
				}},
				{"path": "overrides", "format": "details", "labelKey": "governance:difference", "itemFields": []viewProps{
					{"labelKey": "governance:overrideFields", "args": viewProps{"id": fieldPath("record.itemId"), "fields": fieldPath("record.fields")}},
				}},
			}},
			{"labelKey": "governance:history", "fields": []viewProps{plain(revision)}},
			{"labelKey": "admin:fields.createdAt", "fields": []viewProps{{"path": "createdAt", "format": "dateTime"}}},
			{"labelKey": "admin:fields.actions", "actions": []viewProps{
				{"id": kind, "labelKey": "governance:" + kind, "permission": permission, "className": "ai-button is-ghost"},
			}},
		},
	}, ar.EventBinding{Event: "action", Action: "setData", Target: dictionaryActionData, When: &when,
		Payload: objectValue(mapping("id", fieldPath("event.item.id")), mapping("kind", constant(kind)), mapping("baseRevision", base)),
	})
	return browserCard("dictionary-"+path, "governance:"+path, "", table)
}

func dictionaryReleaseDialog(rollback bool) ar.Node {
	kind, capability, hint := "publish", "dictionary.draft.publish", "publishHint"
	action := "data." + dictionaryActionData
	key := fieldPath("event.operationKey")
	input := []ar.Mapping{mapping("id", fieldPath(action+".id")), mapping("baseRevision", fieldPath(action+".baseRevision"))}
	fields := []viewProps{}
	event := ar.EventBinding{Event: "submit", Action: "invoke", IdempotencyKey: &key}
	if rollback {
		kind, capability, hint = "rollback", "dictionary.history.rollback", "reauthHint"
		fields = append(fields,
			viewProps{"name": "password", "type": "password", "labelKey": "governance:password", "required": true, "maxLength": 128},
			viewProps{"name": "reason", "type": "text", "labelKey": "governance:reason", "required": true, "trim": true, "minLength": 5, "maxLength": 500},
		)
		input = append(input, mapping("reason", fieldPath("event.values.reason")))
		event.Reauthenticate = &ar.Reauthentication{Capability: ar.Ref("settings.password.verify"), Operation: "dictionaries.rollback"}
	} else {
		input = append(input, mapping("proof", constant("")))
	}
	event.Target, event.Payload = capability+"@1.0.0", objectValue(input...)
	form := componentNode("dictionary-"+kind+"-form", "action-form", "", viewProps{
		"fields": fields, "submitKey": "governance:" + kind, "cancelKey": "governance:cancel",
	}, event, dictionaryClose("success"), dictionaryClose("cancel"))
	return dictionaryDialog(kind, kind, hint, form)
}

func dictionaryImportDialog() ar.Node {
	data := "data." + dictionaryImportData
	id := fieldPath("data." + dictionarySelectedData + ".id")
	result := fieldPath("event.data")
	key := fieldPath("event.operationKey")
	input := []ar.Mapping{
		mapping("id", id), mapping("format", fieldPath("event.format")), mapping("content", fieldPath("event.content")),
	}
	form := componentNode("dictionary-import-form", "file-review-form", "", viewProps{
		"formats": []string{"csv", "json"}, "maxBytes": 10 << 20, "fileKey": "governance:filename",
		"reviewKey": "governance:preview", "submitKey": "governance:importDraft", "cancelKey": "governance:cancel",
		"reasonKey": "governance:reason", "invalidFileKey": "governance:fileLimit", "invalidReasonKey": "governance:invalid",
	}, ar.EventBinding{
		Event: "review", Action: "invoke", Target: "dictionary.import.preview@1.0.0",
		Payload: objectValue(input...), ResultTarget: dictionaryImportData, Result: &result,
	}, ar.EventBinding{
		Event: "submit", Action: "invoke", Target: "dictionary.import.draft@1.0.0", IdempotencyKey: &key,
		Payload: objectValue(append(input,
			mapping("baseRevision", fieldPath("data."+dictionaryDetailData+".document.definition.revision")),
			mapping("reason", fieldPath("event.reason")),
		)...),
	}, dictionaryClose("success"), dictionaryClose("cancel"))
	accepted := operation("and", operation("exists", fieldPath(data)), operation("eq", operation("count", fieldPath(data+".errors")), constant(0)))
	componentOf(form).Bindings = []ar.Mapping{mapping("reviewAccepted", accepted)}
	summary := componentNode("dictionary-import-summary", "text", "$"+dictionaryImportData, viewProps{"textKey": "governance:addedUpdated", "role": "status"})
	componentOf(summary).Bindings = []ar.Mapping{mapping("values", objectValue(mapping("added", fieldPath("record.added")), mapping("updated", fieldPath("record.updated"))))}
	rows := componentNode("dictionary-import-rows", "content-table", "$"+dictionaryImportData, viewProps{"rowsPath": "document.items", "limit": 100,
		"columns": []viewProps{
			{"labelKey": "governance:code", "fields": []viewProps{{"path": "code", "format": "code"}}},
			{"labelKey": "governance:labelEn", "fields": []viewProps{plain("labelEn")}},
			{"labelKey": "governance:labelZh", "fields": []viewProps{plain("labelZh")}},
		},
	})
	componentOf(rows).Visible = &accepted
	errors := componentNode("dictionary-import-errors", "content-table", "$"+dictionaryImportData, viewProps{"rowsPath": "errors",
		"columns": []viewProps{{"labelKey": "governance:invalid", "fields": []viewProps{{
			"format": "issue", "labelKey": "governance:importError", "labelPrefix": "governance:importErrors.", "fallbackKey": "governance:invalid",
		}}}},
	})
	componentOf(form).Content = layoutNode("dictionary-import-result", ar.LayoutProps{Gap: "8px"}, summary, errors, rows).Layout
	return dictionaryDialog("import", "import", "importHint", form)
}

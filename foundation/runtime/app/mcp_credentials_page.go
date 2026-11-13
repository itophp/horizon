package app

import ar "horizon/foundation/kernel/applicationruntime"

const (
	mcpCredentialSelection = "_credentialSelection"
	mcpCredentialNotice    = "_credentialRevoked"
	mcpRevokePermission    = "mcp:revoke"
)

// mcpCredentialPanel is the declarative credential section of the next Agent
// collaboration definition. The local connection sections can be composed next
// to it without giving them access to passwords or issued secrets.
func mcpCredentialPanel(provider string, enabledWhen *ar.Expression) ar.Node {
	return mcpCredentialPanelVersion(provider, enabledWhen, false)
}

func mcpCredentialPanelVersion(provider string, enabledWhen *ar.Expression, legacy bool) ar.Node {
	const tokenData = "credentials"
	open := func(event, kind string) ar.EventBinding {
		return ar.EventBinding{
			Event: event, Action: "setData", Target: mcpCredentialSelection,
			Payload: objectValue(mapping("kind", constant(kind))),
		}
	}
	allowed := permits(mcpRevokePermission)
	add := componentNode("add-credential", "button", "", viewProps{
		"labelKey": "mcp:addToken", "variant": "default",
	}, open("click", "create"))
	emergency := componentNode("emergency-revoke-credential", "button", "", viewProps{
		"labelKey": "mcp:emergencyRevoke", "variant": "outline",
	}, open("click", "emergency"))
	componentOf(emergency).Visible = &allowed
	actions := layoutNode("credential-toolbar", ar.LayoutProps{
		ClassName: "webmcp-actions", Columns: "minmax(0, 1fr)", Gap: "8px",
		Responsive: map[string]ar.LayoutProps{"md": {Columns: "repeat(2, max-content)"}},
	}, add, emergency)

	active := same("record.revokedAt", "")
	revocation := platformColumn("revokedBy", "mcp:revokedBy", "")
	revocation["fields"] = []viewProps{
		{"when": active, "labelKey": "mcp:notRevoked"},
		{"when": operation("and", operation("not", active), same("record.revokedBy", 0)), "labelKey": "mcp:systemActor"},
		{"when": operation("and", operation("not", active), operation("not", same("record.revokedBy", 0))), "path": "revokedBy"},
	}
	table := componentNode("credential-table", "table-panel", "$"+tokenData, viewProps{
		"columns": []viewProps{
			platformColumn("name", "mcp:name", ""), platformColumn("prefix", "mcp:prefix", "mono"),
			platformColumn("departmentId", "mcp:department", ""), platformColumn("expiresAt", "mcp:expires", "date"), revocation,
		},
		"eventActions": []viewProps{
			{"id": "rotate", "labelKey": "mcp:rotate", "visible": active, "permission": mcpRevokePermission, "className": "ai-button is-ghost"},
			{"id": "revoke", "labelKey": "mcp:revoke", "visible": active, "permission": mcpRevokePermission, "className": "ai-button is-ghost"},
		},
		"search": false,
	}, credentialRowAction("rotate"), credentialRowAction("revoke"))
	if legacy {
		legacyColumn := func(label, path, format string) viewProps {
			field := viewProps{"path": path}
			if format != "" {
				field["format"] = format
			}
			return viewProps{"labelKey": label, "fields": []viewProps{field}}
		}
		actions := componentOf(table).Props["eventActions"]
		componentOf(table).Kind = "content-table"
		componentOf(table).Props = viewProps{"emptyKey": "runtime:protocol.empty", "columns": []viewProps{
			legacyColumn("mcp:name", "name", ""), legacyColumn("mcp:prefix", "prefix", "code"),
			legacyColumn("mcp:department", "departmentId", ""), legacyColumn("mcp:expires", "expiresAt", "dateTime"),
			{"labelKey": "mcp:revokedBy", "fields": revocation["fields"]},
			{"labelKey": "mcp:actions", "actions": actions},
		}}
	}
	table.Layout.Data = []ar.DataSpec{{ID: tokenData, Provider: provider, EnabledWhen: enabledWhen, NonBlocking: true}}

	revoked := same("data."+mcpCredentialNotice, true)
	notice := componentNode("credential-revoked-notice", "content-notice", "", viewProps{"labelKey": "mcp:emergencyDone"},
		ar.EventBinding{Event: "dismiss", Action: "setData", Target: mcpCredentialNotice, Payload: constant(false)})
	componentOf(notice).Visible = &revoked

	create := credentialDialog("create", "createTitle", "tokenLimits", "create", "mcp.token.create", true)
	componentOf(create).Props["wide"] = true
	form := componentOf(componentOf(create).Content.Children[0])
	form.Bindings = []ar.Mapping{mapping("fields", operation("array",
		constant(credentialField(legacy, viewProps{"name": "name", "type": "text", "labelKey": "mcp:name", "required": true, "maxLength": 100, "invalidKey": "mcp:validation.name"})),
		constant(credentialField(legacy, viewProps{"name": "days", "type": "integer", "labelKey": "mcp:days", "required": true, "defaultValue": 30, "min": 1, "max": 90, "invalidKey": "mcp:validation.days"})),
		objectValue(
			mapping("name", constant("departmentId")), mapping("type", constant("select")),
			mapping("labelKey", constant("mcp:department")), mapping("invalidKey", constant("mcp:validation.department")), mapping("required", constant(true)),
			mapping("defaultValue", fieldPath("context.activeDepartmentId")),
			mapping("options", fieldPath("context.departmentOptions")),
		),
		constant(credentialField(legacy, credentialPassword("password"))),
	))}
	if legacy {
		department := &form.Bindings[0].Value.Items[2]
		department.Fields = append(department.Fields[:3], department.Fields[4:]...)
	}
	form.Events[0].Payload = objectValue(
		mapping("name", fieldPath("event.values.name")),
		mapping("days", fieldPath("event.values.days")),
		mapping("departmentId", fieldPath("event.values.departmentId")),
		mapping("password", fieldPath("event.values.password")),
	)

	rotate := credentialDialog("rotate", "rotateTitle", "rotateHelp", "confirmRotate", "mcp.token.rotate", true)
	revoke := credentialDialog("revoke", "revokeTitle", "revokeHelp", "confirmRevoke", "mcp.token.revoke", false)
	for _, dialog := range []ar.Node{rotate, revoke} {
		componentOf(dialog).Bindings = append(componentOf(dialog).Bindings,
			mapping("titleArgs", objectValue(mapping("name", fieldPath("data."+mcpCredentialSelection+".name")))))
	}

	emergencyDialog := credentialDialog("emergency", "emergencyTitle", "emergencyHelp", "emergencySubmit", "mcp.token.admin_revoke", false)
	emergencyForm := componentOf(componentOf(emergencyDialog).Content.Children[0])
	emergencyForm.Props["fields"] = []viewProps{
		{"name": "id", "type": "integer", "labelKey": "mcp:emergencyId", "required": true, "min": 1, "max": 1 << 40},
		credentialPassword("adminPassword"),
	}
	emergencyForm.Events[0].Payload = objectValue(mapping("id", fieldPath("event.values.id")), mapping("password", fieldPath("event.values.password")))
	emergencyForm.Events = append(emergencyForm.Events, ar.EventBinding{Event: "success", Action: "setData", Target: mcpCredentialNotice, Payload: constant(true)})
	for _, dialog := range []ar.Node{rotate, revoke, emergencyDialog} {
		componentOf(dialog).Visible = &allowed
	}

	if legacy {
		for _, dialog := range []ar.Node{create, rotate, revoke, emergencyDialog} {
			form := componentOf(componentOf(dialog).Content.Children[0])
			delete(form.Props, "errorPrefix")
			for _, field := range form.Props["fields"].([]viewProps) {
				credentialField(true, field)
			}
		}
	}
	root := panel("credential-management", "webmcp-section", actions, table, notice, create, rotate, revoke, emergencyDialog)
	componentOf(root).Props["titleKey"] = "mcp:tokens"
	componentOf(root).Props["descriptionKey"] = "mcp:tokenLimits"
	root.Layout.Data = []ar.DataSpec{
		{ID: mcpCredentialSelection},
		{ID: mcpCredentialNotice, Initial: false},
	}
	return root
}

func credentialPassword(label string) viewProps {
	return viewProps{"name": "password", "type": "password", "labelKey": "mcp:" + label, "required": true, "minLength": 1, "maxLength": 128, "invalidKey": "mcp:validation.password"}
}

func credentialRowAction(kind string) ar.EventBinding {
	when := same("event.action", kind)
	return ar.EventBinding{
		Event: "action", Action: "setData", Target: mcpCredentialSelection, When: &when,
		Payload: objectValue(mapping("kind", constant(kind)), mapping("id", fieldPath("event.item.id")), mapping("name", fieldPath("event.item.name"))),
	}
}

func credentialDialog(kind, title, help, submit, capability string, disclose bool) ar.Node {
	close := func(event string) ar.EventBinding {
		return ar.EventBinding{Event: event, Action: "setData", Target: mcpCredentialSelection, Payload: constant(nil)}
	}
	key := fieldPath("event.operationKey")
	invoke := ar.EventBinding{
		Event: "submit", Action: "invoke", Target: capability + "@1.0.0", IdempotencyKey: &key,
		Payload: objectValue(mapping("id", fieldPath("data."+mcpCredentialSelection+".id")), mapping("password", fieldPath("event.values.password"))),
	}
	if disclose {
		invoke.Disclosure = &ar.Disclosure{
			Field: "secret", TitleKey: "mcp:secretOnce", CopyLabelKey: "mcp:copyToken",
			DismissKey: "mcp:dismiss", CopySuccessKey: "mcp:tokenCopied", CopyErrorKey: "mcp:copyTokenFailed",
		}
	}
	form := componentNode(kind+"-credential-form", "action-form", "", viewProps{
		"fields": []viewProps{credentialPassword("password")}, "submitKey": "mcp:" + submit, "cancelKey": "mcp:cancel", "errorPrefix": "mcp:failure",
	}, invoke, close("success"), close("cancel"))
	dialog := componentNode(kind+"-credential-dialog", "dialog", "", viewProps{
		"titleKey": "mcp:" + title, "descriptionKey": "mcp:" + help,
	}, close("close"))
	componentOf(dialog).Bindings = []ar.Mapping{mapping("open", same("data."+mcpCredentialSelection+".kind", kind))}
	componentOf(dialog).Content = layoutNode(kind+"-credential-content", ar.LayoutProps{}, form).Layout
	return dialog
}

func credentialField(legacy bool, field viewProps) viewProps {
	if legacy {
		delete(field, "invalidKey")
	}
	return field
}

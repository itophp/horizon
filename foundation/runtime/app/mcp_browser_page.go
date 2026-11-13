package app

import ar "horizon/foundation/kernel/applicationruntime"

func browserAction(event, target string, input ar.Expression) ar.EventBinding {
	return ar.EventBinding{Event: event, Action: "browser", Target: target, Payload: input}
}

func browserText(id, key string) ar.Node {
	return componentNode(id, "text", "", viewProps{"textKey": key})
}

func browserMarkdown(id string, parts ...ar.Expression) ar.Node {
	node := componentNode(id, "markdown", "", nil)
	componentOf(node).Bindings = []ar.Mapping{
		mapping("value", operation("concat", parts...)),
	}
	return node
}

func browserCard(id, title, description string, nodes ...ar.Node) ar.Node {
	node := panel(id, "webmcp-section", nodes...)
	componentOf(node).Props["titleKey"] = title
	if description != "" {
		componentOf(node).Props["descriptionKey"] = description
	}
	return node
}

func browserDisabled(node ar.Node, expression ar.Expression) ar.Node {
	componentOf(node).Bindings = append(componentOf(node).Bindings, mapping("disabled", expression))
	return node
}

func webMcpContent(legacy bool) []ar.Node {
	connected := same("context.browser.status", "connected")
	disconnected := same("context.browser.status", "disconnected")
	busy := same("context.browser.busy", true)
	unavailable := operation("not", operation("exists", fieldPath("context.browser.status")))

	connectForm := componentNode("page-connect-form", "action-form", "", viewProps{
		"fields": []viewProps{
			{"name": "port", "type": "integer", "labelKey": "webmcp:port", "invalidKey": "webmcp:invalidPort", "required": true, "min": 1024, "max": 65535, "defaultValue": 18793},
			{"name": "token", "type": "password", "labelKey": "webmcp:pairingKey", "invalidKey": "webmcp:invalidKey", "required": true, "minLength": 64, "maxLength": 64, "format": "hex"},
		},
		"submitKey": "webmcp:connect", "browserDefaults": "webmcp-pairing", "prefilledKey": "webmcp:prefilled",
	}, browserAction("submit", "webmcp.connect", objectValue(
		mapping("port", fieldPath("event.values.port")),
		mapping("token", fieldPath("event.values.token")),
	)))
	browserDisabled(connectForm, operation("or", unavailable, busy, connected))
	disconnect := browserDisabled(
		componentNode("page-disconnect", "button", "", viewProps{"labelKey": "webmcp:disconnect", "variant": "outline"},
			browserAction("click", "webmcp.disconnect", objectValue())),
		operation("or", unavailable, busy, disconnected),
	)
	state := componentNode("page-connection-status", "text", "", viewProps{"role": "status", "className": "neutral-badge"})
	componentOf(state).Bindings = []ar.Mapping{mapping("textKey", operation("concat",
		constant("webmcp:status."), operation("coalesce", fieldPath("context.browser.status"), constant("disconnected"))))}
	count := browserText("page-connection-tool-count", "webmcp:toolCount")
	componentOf(count).Bindings = []ar.Mapping{
		mapping("values", objectValue(mapping("count", operation("coalesce", fieldPath("context.browser.toolCount"), constant(0))))),
	}
	layerCounts := browserText("page-layer-counts", "webmcp:layerCounts")
	componentOf(layerCounts).Bindings = []ar.Mapping{mapping("values", fieldPath("context.browser.scopes"))}
	connection := browserCard("page-connection", "webmcp:connection", "webmcp:connectionHint",
		state,
		browserText("page-layer-hint", "webmcp:layersHint"),
		browserText("page-coverage-hint", "webmcp:coverageHint"),
		layerCounts,
		componentNode("page-start-command", "markdown", "", viewProps{"textKey": "webmcp:startCommand", "code": true}),
		connectForm, disconnect, count,
	)

	clientCard := func(client string) ar.Node {
		install := browserDisabled(
			componentNode("install-page-"+client, "button", "", viewProps{"labelKey": "webmcp:install", "variant": "outline", "hideInPreview": true},
				browserAction("click", "webmcp.install", objectValue(mapping("client", constant(client))))),
			operation("or", unavailable, busy, operation("not", connected)),
		)
		if legacy {
			delete(componentOf(install).Props, "hideInPreview")
		}
		return browserCard("page-client-"+client, "webmcp:client."+client, "webmcp:clientHint."+client, install)
	}
	clientGrid := layoutNode("page-clients-grid", ar.LayoutProps{
		Gap: "12px", Columns: "minmax(0, 1fr)",
		Responsive: map[string]ar.LayoutProps{"md": {Columns: "repeat(2, minmax(0, 1fr))"}},
	}, clientCard("codex"), clientCard("workbuddy"))
	clients := browserCard("page-client-setup", "webmcp:clients", "webmcp:clientsHint", clientGrid,
		browserText("page-other-clients", "webmcp:otherClients"),
		componentNode("page-generic-command", "markdown", "", viewProps{"textKey": "webmcp:genericCommand", "code": true}),
	)
	feedback := componentNode("page-connection-feedback", "text", "", nil)
	componentOf(feedback).Bindings = []ar.Mapping{
		mapping("textKey", operation("concat", constant("webmcp:"), fieldPath("context.browser.webFeedback"))),
		mapping("role", operation("if", same("context.browser.webFeedback", "setupDone"), constant("status"), constant("alert"))),
	}
	feedbackVisible := operation("exists", fieldPath("context.browser.webFeedback"))
	componentOf(feedback).Visible = &feedbackVisible
	return []ar.Node{
		browserText("page-connect-intro", "webmcp:subtitle"), connection, feedback, clients,
		browserCard("page-assistant", "webmcp:assistant", "webmcp:assistantHint", browserText("page-assistant-reserved", "webmcp:reserved")),
	}
}

func remoteMcpClientContent() []ar.Node {
	origin := fieldPath("context.browser.origin")
	code := func(id, before, after string) ar.Node {
		return browserMarkdown(id, constant("```\n"+before), origin, constant(after+"\n```"))
	}
	busy := same("context.browser.busy", true)
	unavailable := operation("not", operation("exists", fieldPath("context.browser.origin")))
	disabled := operation("or", unavailable, busy)
	token := viewProps{"name": "token", "type": "password", "labelKey": "mcp:credential", "required": true, "maxLength": 4096}
	diagnose := browserDisabled(componentNode("remote-diagnose-form", "action-form", "", viewProps{
		"fields": []viewProps{token}, "submitKey": "mcp:diagnose",
	}, browserAction("submit", "remote-mcp.diagnose", objectValue(mapping("token", fieldPath("event.values.token"))))), disabled)

	prepare := componentNode("remote-install-form", "action-form", "", viewProps{"submitKey": "mcp:prepareInstall"},
		browserAction("submit", "remote-mcp.prepare", objectValue(
			mapping("client", fieldPath("event.values.client")),
			mapping("key", fieldPath("event.values.key")),
			mapping("token", fieldPath("event.values.token")),
			mapping("departmentId", fieldPath("event.values.departmentId")),
		)))
	componentOf(prepare).Bindings = []ar.Mapping{mapping("fields", operation("array",
		constant(viewProps{"name": "client", "type": "select", "labelKey": "mcp:installClient", "required": true,
			"options": []viewProps{{"value": "codex", "labelKey": "webmcp:client.codex"}, {"value": "workbuddy", "labelKey": "webmcp:client.workbuddy"}},
		}),
		objectValue(mapping("name", constant("departmentId")), mapping("type", constant("select")),
			mapping("labelKey", constant("mcp:department")), mapping("required", constant(true)),
			mapping("defaultValue", fieldPath("context.activeDepartmentId")),
			mapping("options", fieldPath("context.departmentOptions"))),
		constant(viewProps{"name": "key", "type": "password", "labelKey": "mcp:pairing", "required": true, "minLength": 64, "maxLength": 64, "format": "hex"}),
		constant(token),
	))}
	browserDisabled(prepare, disabled)
	confirm := browserDisabled(componentNode("remote-install-confirm", "button", "", viewProps{
		"labelKey": "mcp:confirmInstall", "variant": "default",
	}, browserAction("click", "remote-mcp.install", objectValue())), disabled)
	cancel := browserDisabled(componentNode("remote-install-cancel", "button", "", viewProps{
		"labelKey": "mcp:cancel", "variant": "outline",
	}, browserAction("click", "remote-mcp.cancel", objectValue())), disabled)
	dialog := componentNode("remote-install-dialog", "dialog", "", viewProps{
		"titleKey": "mcp:confirmInstall", "descriptionKey": "mcp:installTarget",
	}, browserAction("close", "remote-mcp.cancel", objectValue()))
	componentOf(dialog).Bindings = []ar.Mapping{
		mapping("open", operation("exists", fieldPath("context.browser.prepared"))),
		mapping("descriptionArgs", fieldPath("context.browser.prepared")),
	}
	componentOf(dialog).Content = layoutNode("remote-install-actions", ar.LayoutProps{ClassName: "form-actions", Columns: "repeat(2, max-content)", Gap: "8px"}, confirm, cancel).Layout
	status := componentNode("remote-helper-status", "text", "", viewProps{"role": "status"})
	componentOf(status).Bindings = []ar.Mapping{mapping("textKey", operation("concat", constant("mcp:helperStatus."),
		operation("if", busy, constant("busy"), operation("coalesce", fieldPath("context.browser.helperStatus"), constant("ready")))))}
	return []ar.Node{
		browserText("remote-credential-intro", "mcp:description"),
		browserText("remote-capability-intro", "mcp:dynamicTools"),
		browserCard("remote-codex-instructions", "mcp:connect", "mcp:import",
			code("remote-codex-config", "[mcp_servers.baseline-business]\nurl = \"", "/mcp\"\nbearer_token_env_var = \"BASELINE_MCP_TOKEN\"")),
		browserCard("remote-workbuddy-instructions", "mcp:workbuddy", "mcp:workbuddyHelp",
			code("remote-workbuddy-config", "{\n  \"mcpServers\": {\n    \"baseline-business\": {\n      \"type\": \"streamableHttp\",\n      \"url\": \"", "/mcp\",\n      \"headers\": { \"Authorization\": \"Bearer ${BASELINE_MCP_TOKEN}\" }\n    }\n  }\n}")),
		browserCard("remote-local-helper", "mcp:helper", "mcp:helperHelp",
			code("remote-helper-command", "MCP_BUSINESS_URL=", "/mcp pnpm mcp:helper"),
			diagnose, prepare, status, dialog),
	}
}

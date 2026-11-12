package app

import ar "horizon/foundation/kernel/applicationruntime"

func platformPage(id, route, title, subtitle string, providers []ar.Provider, content ...ar.Node) ar.ApplicationV2 {
	return platformPageVersion(id, route, title, subtitle, "1.0.0", providers, content...)
}

func platformPageVersion(id, route, title, subtitle, version string, providers []ar.Provider, content ...ar.Node) ar.ApplicationV2 {
	pageID := id + ".home"
	head := componentNode(id+"-heading", "page-heading", "", viewProps{"titleKey": title, "subtitleKey": subtitle})
	root := layoutNode(id+"-root", ar.LayoutProps{Gap: "16px", Width: "full"}, append([]ar.Node{head}, content...)...)
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", RuntimeCompatibility: "3.0",
		ID: id, Version: version, Source: "builtin:" + id, Route: route,
		LabelKey: title, Permission: ar.AuthenticatedPermission, Providers: providers,
		Pages: []ar.PageV3{{
			SchemaVersion: "3.0", ID: pageID, Route: route, TitleKey: title,
			SubtitleKey: subtitle, Permission: ar.AuthenticatedPermission, Root: root,
		}},
	}
	return ar.ApplicationV2{
		SchemaVersion: "3.0", RuntimeCompatibility: "3.0", Protocol: protocol,
		ID: id, Version: version, PluginVersion: version, Source: protocol.Source,
		LabelKey: title, Permission: protocol.Permission, Route: route,
	}
}

func platformTabs(id string, props viewProps, children ...ar.Node) ar.Node {
	tabs := componentNode(id, "layout-tabs", "", props)
	componentOf(tabs).Content = layoutNode(id+"-content", ar.LayoutProps{Gap: "16px"}, children...).Layout
	return tabs
}

func platformTab(id string, children ...ar.Node) ar.Node {
	tab := componentNode(id+"-tab", "layout-tab-panel", "", viewProps{"tab": id})
	componentOf(tab).Content = layoutNode(id+"-content", ar.LayoutProps{Gap: "16px"}, children...).Layout
	return tab
}

func platformColumn(name, label, display string) viewProps {
	return viewProps{"name": name, "labelKey": label, "display": display}
}

func agentCollaborationApplication() ar.ApplicationV2 {
	const canAudit = "mcp:revoke"
	const tokens = "tokens"
	const audits = "_audits"
	remoteActive := same("query.tab", "remote")
	tokensPanel := componentNode("remote-mcp", "remote-mcp-panel", "$"+tokens, viewProps{"table": viewProps{
		"columns": []viewProps{
			platformColumn("name", "mcp:name", ""),
			platformColumn("prefix", "mcp:prefix", "mono"),
			platformColumn("departmentId", "mcp:department", ""),
			platformColumn("expiresAt", "mcp:expires", "date"),
			platformColumn("revocation", "mcp:revokedBy", ""),
		},
	}}, ar.EventBinding{Event: "refresh", Action: "refresh", Target: tokens})
	tokensPanel.Layout.Data = []ar.DataSpec{{ID: tokens, Provider: tokens, EnabledWhen: &remoteActive, NonBlocking: true}}
	auditAllowed := permits(canAudit)
	auditActive := operation("and", remoteActive, auditAllowed)
	auditTable := componentNode("mcp-audit-table", "table-panel", "$"+audits, viewProps{"columns": []viewProps{
		platformColumn("tool", "mcp:auditTool", "mono"),
		platformColumn("resultCode", "mcp:auditResult", ""),
		platformColumn("createdAt", "admin:fields.createdAt", "date"),
	}})
	auditHeading := componentNode("audit-heading", "content-heading", "", viewProps{
		"titleKey": "mcp:audit", "action": viewProps{"id": "refresh", "labelKey": "mcp:refresh", "icon": "refresh"},
	}, ar.EventBinding{Event: "action", Action: "refresh", Target: audits})
	auditPanel := panel("mcp-audit", "admin-panel webmcp-section", auditHeading, auditTable)
	auditPanel.Layout.Visible = &auditAllowed
	auditPanel.Layout.Data = []ar.DataSpec{{ID: audits, Provider: "audits", EnabledWhen: &auditActive, NonBlocking: true}}
	tabs := platformTabs("agent-collaboration-tabs", viewProps{
		"param": "tab", "sessionKey": "baseline.mcp.tab", "labelKey": "mcp:hub.tabsLabel",
		"tabs": []viewProps{
			{"id": "remote", "labelKey": "mcp:hub.tabs.remote", "icon": "command"},
			{"id": "page", "labelKey": "mcp:hub.tabs.page", "icon": "plug", "webMcpCount": true},
			{"id": "skills", "labelKey": "mcp:hub.tabs.skills", "icon": "sparkles"},
		},
	}, platformTab("remote", tokensPanel, auditPanel),
		platformTab("page", componentNode("webmcp", "webmcp-panel", "", nil)),
		platformTab("skills", webSkillsContent()...))
	return platformPage("agent-collaboration", "/mcp", "mcp:hub.title", "mcp:hub.subtitle", []ar.Provider{
		{ID: tokens, Capability: ar.Ref("mcp.token.list"), KeyField: "id", Pagination: "none"},
		{ID: "audits", Capability: ar.Ref("mcp.audit.list"), KeyField: "id", Pagination: "none"},
	}, tabs)
}

func webSkillsContent() []ar.Node {
	prose := func(id, key string) ar.Node {
		return componentNode(id, "markdown", "", viewProps{"textKey": "webmcp:" + key})
	}
	code := func(id, path string) ar.Node {
		node := componentNode(id, "markdown", "$_code", nil)
		node.Layout.Data = []ar.DataSpec{{ID: "_code", Initial: "```\n" + path + "\n```"}}
		return node
	}
	card := func(id, title string, children ...ar.Node) ar.Node {
		node := panel(id, "admin-panel webmcp-section", children...)
		componentOf(node).Props["titleKey"] = "webmcp:" + title
		return node
	}
	skills := []ar.Node{}
	for _, skill := range []string{"navigate", "accounts", "diagnose"} {
		skills = append(skills, componentNode("skill-"+skill, "content-heading", "", viewProps{
			"titleKey":       "webmcp:skillNames." + skill,
			"descriptionKey": "webmcp:skillHints." + skill,
		}), code("path-"+skill, "webskills/baseline-web-"+skill))
	}
	skills = append(skills, prose("skills-install", "skillsInstall"))
	return []ar.Node{
		prose("skills-intro", "skillsIntro"),
		card("page-skills", "skills", skills...),
		card("business-skill", "businessSkill", prose("business-skill-hint", "businessSkillHint"),
			code("business-skill-path", "webskills/baseline-business")),
		card("skills-safety", "safetyTitle", prose("skills-safety-hint", "safety")),
	}
}

func legacyPersonalPreferencesApplication() ar.ApplicationV2 {
	return platformPage("personal-preferences", "/preferences", "governance:preferences", "governance:preferencesHint", nil,
		componentNode("preferences-editor", "settings-panel", "", viewProps{"personal": true}))
}

func systemSettingsApplication() ar.ApplicationV2 {
	const dictionaries = "_dictionaries"
	configPermission := permits("settings:read")
	fieldsPermission := permits("dictionaries:read")
	config := componentNode("workspace-settings", "settings-panel", "", nil)
	config.Layout.Visible = &configPermission
	name := platformColumn("displayName", "governance:dictionaries", "entity-link")
	name["subtitleField"] = "code"
	kind := platformColumn("kind", "governance:kind", "enumBadge")
	kind["prefixKey"] = "governance:"
	status := platformColumn("status", "governance:status", "enumBadge")
	status["prefixKey"] = "governance:"
	fields := componentNode("dictionary-workbench", "dictionary-panel", "$"+dictionaries, viewProps{
		"table": viewProps{"columns": []viewProps{
			name, platformColumn("category", "governance:category", ""), kind, status,
			platformColumn("revision", "governance:history", ""),
		}},
	}, ar.EventBinding{Event: "refresh", Action: "refresh", Target: dictionaries})
	fieldsActive := operation("and", fieldsPermission,
		operation("or", same("query.tab", "fields"), operation("not", configPermission)))
	fields.Layout.Visible = &fieldsPermission
	fields.Layout.Data = []ar.DataSpec{{ID: dictionaries, Provider: "dictionary-list", EnabledWhen: &fieldsActive, NonBlocking: true}}
	tabs := platformTabs("settings-tabs", viewProps{"param": "tab", "hideSingleTab": true, "tabs": []viewProps{
		{"id": "config", "labelKey": "governance:generalTab", "icon": "sliders", "permission": "settings:read"},
		{"id": "fields", "labelKey": "governance:fieldsTab", "icon": "tree", "permission": "dictionaries:read"},
	}}, platformTab("config", config), platformTab("fields", fields))
	return platformPage("system-settings", "/settings", "governance:settings", "governance:settingsHint",
		[]ar.Provider{{
			ID: "dictionary-list", Capability: ar.Ref("dictionary.list.read"), KeyField: "id", Pagination: "none",
			ResultPath: "definitions",
			Input:      []ar.Mapping{mapping("targetKind", constant("workspace")), mapping("targetId", constant(0))},
		}}, tabs)
}

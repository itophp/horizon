package app

import (
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/identity"
)

const analyticsLabel = "admin:nav.statistics"

func AnalyticsRuntimeApplication() ar.ApplicationV2 {
	return analyticsRuntimeApplication("1.0.1")
}

func analyticsRuntimeApplication(version string) ar.ApplicationV2 {
	updated := version != "1.0.0"
	const permission = ar.AuthenticatedPermission
	const id = "analytics"
	const pageID = "analytics.dashboard"
	const route = "/analytics"

	heading := componentNode("analytics-heading", "page-heading", "", viewProps{
		"titleKey": "admin:overview.title", "subtitleKey": "admin:overview.welcome",
		"subtitleAccountName": true, "showCurrentDate": true,
	})
	metrics := layoutNode("analytics-metrics", ar.LayoutProps{
		Gap: "16px", Columns: "minmax(0, 1fr)",
		Responsive: map[string]ar.LayoutProps{
			"md": {Columns: "repeat(2, minmax(0, 1fr))"},
			"lg": {Columns: "repeat(4, minmax(0, 1fr))"},
		},
	},
		metricCard("active", "activeAccounts", "users", "accounts.active", "totalAccounts", viewProps{"count": fieldPath("data.stats.accounts.total")}),
		metricCard("organizations", "orgCount", "building", "organizations.total", "orgDetail",
			viewProps{"enterprises": fieldPath("data.stats.organizations.enterprises"), "companies": fieldPath("data.stats.organizations.companies")}),
		metricCard("roles", "roleCount", "shield", "roles", "roleDetail", nil),
		metricCard("departments", "deptCount", "network", "departments", "deptDetail", nil),
	)

	tabs := componentNode("analytics-tabs", "layout-tabs", "", viewProps{
		"param": "tab", "tabs": []viewProps{
			{"id": "overview", "labelKey": "admin:overview.tabOverview"},
			{"id": "recent", "labelKey": "admin:overview.tabActivity"},
		},
	})
	componentOf(tabs).Content = layoutNode("analytics-tab-content", ar.LayoutProps{Gap: "16px"},
		analyticsTab("overview", analyticsOverview(updated)),
		analyticsTab("recent", analyticsActivityPanel(updated)),
	).Layout
	// The tabs sit above the shared indicators, and only their panels switch.
	componentOf(tabs).Content.Children = append([]ar.Node{metrics}, componentOf(tabs).Content.Children...)

	root := layoutNode("analytics-root", ar.LayoutProps{Gap: "16px", Width: "full"}, heading, tabs)
	root.Layout.Data = []ar.DataSpec{{
		ID: "stats", Provider: "analytics",
		Input: []ar.Mapping{mapping("timeZone", fieldPath("context.timeZone"))},
	}}
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", RuntimeCompatibility: "3.0", ID: id, Version: version,
		Source: identity.AnalyticsRuntimeSource, LabelKey: analyticsLabel, Permission: permission, Route: route,
		Providers: []ar.Provider{{ID: "analytics", Capability: ar.Ref("identity.analytics.read"), KeyField: "id", Pagination: "none"}},
		Pages: []ar.PageV3{{SchemaVersion: "3.0", ID: pageID, Route: route,
			TitleKey: "admin:overview.title", Permission: permission, Root: root}},
		Navigation: []ar.Navigation{{PageID: pageID, LabelKey: analyticsLabel, Icon: "dashboard"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", RuntimeCompatibility: "3.0", ID: id,
		Version: version, PluginVersion: version, MetadataVersion: "1", Source: protocol.Source,
		LabelKey: analyticsLabel, Permission: permission, Route: route,
	}
}

func analyticsTab(id string, content ar.Node) ar.Node {
	tab := componentNode("analytics-tab-"+id, "layout-tab-panel", "", viewProps{"tab": id})
	componentOf(tab).Content = content.Layout
	return tab
}

func metricCard(id, label, icon, path, detail string, args viewProps) ar.Node {
	value := fieldPath("data.stats." + path)
	metric := componentNode("metric-"+id, "metric-value", "", viewProps{
		"titleKey": "admin:overview." + label, "detailKey": "admin:overview." + detail,
		"icon": icon, "detailArgs": args,
	})
	componentOf(metric).Bindings = []ar.Mapping{mapping("value", value)}
	card := panel("metric-card-"+id, "admin-panel stat-card", metric)
	visible := operation("exists", value)
	card.Layout.Visible = &visible
	return card
}

func analyticsOverview(updated bool) ar.Node {
	trend := analyticsDataComponent("account-trend", "time-series", "trend", viewProps{
		"labelKey": "admin:overview.chartTitle",
	})
	distribution := analyticsDataComponent("department-bars", "horizontal-bars", "distribution", viewProps{
		"labelKey": "admin:overview.distribution",
	})
	componentOf(distribution).Bindings = append(componentOf(distribution).Bindings,
		mapping("maximum", fieldPath("data.stats.accounts.total")))
	accountTable := analyticsDataComponent("recent-accounts", "data-table", "recentAccounts", viewProps{
		"columns": []viewProps{
			{"name": "fullName", "labelKey": "admin:fields.fullName", "display": "entity"},
			{"name": "departments", "labelKey": "admin:fields.department", "display": "badges"},
			{"name": "status", "labelKey": "admin:fields.status", "display": "status"},
			{"name": "createdAt", "labelKey": "admin:fields.createdAt", "display": "date"},
		},
	})
	trendCard := analyticsCard("analytics-trend", "trend", "chart-panel",
		analyticsHeading("trend-heading", "admin:overview.chartTitle", "admin:overview.chartSubtitle", nil),
		trend, componentNode("trend-scope", "text", "", viewProps{"textKey": "admin:overview.scopeNote", "className": "panel-small-footer"}))
	distributionCard := analyticsCard("analytics-distribution", "distribution", "distribution-panel",
		analyticsHeading("distribution-heading", "admin:overview.distribution", "", analyticsLink("departments", "admin:departments.title", "/organizations?tab=departments", "organizations:read")),
		distribution)
	accountsCard := analyticsCard("analytics-accounts", "recentAccounts", "recent-accounts-panel",
		analyticsHeading("recent-heading", "admin:overview.recentAccounts", "admin:overview.scopeNote", analyticsLink("accounts", "admin:overview.viewAll", "/accounts", "accounts:read")),
		accountTable)
	shortcuts := panel("analytics-shortcuts", "admin-panel quick-actions",
		analyticsHeading("shortcuts-heading", "admin:overview.shortcuts", "", nil),
		componentNode("analytics-links", "item-list", "", viewProps{
			"items":    []viewProps{{"id": "structure"}, {"id": "access"}},
			"itemWhen": operation("if", same("record.id", "structure"), permits("organizations:read"), permits("roles:read")),
			"actions": []viewProps{
				analyticsShortcut("structure", "admin:overview.structure", "/organizations", "organizations:read"),
				analyticsShortcut("access", "admin:overview.access", "/permissions", "roles:read"),
			},
		}),
	)
	if updated {
		distributionCard = analyticsDistributionCard(distribution)
		content := componentOf(shortcuts).Content
		componentOf(content.Children[0]).Props["descriptionKey"] = "admin:overview.shortcutsDescription"
		componentOf(content.Children[1]).Props["className"] = "protocol-shortcut-grid"
		componentOf(content.Children[1]).Props["actions"] = []viewProps{
			analyticsShortcutDetails("structure", "admin:overview.structure", "/organizations", "organizations:read", "building"),
			analyticsShortcutDetails("access", "admin:overview.access", "/permissions", "roles:read", "shield"),
		}
		content.Children = append(content.Children, analyticsSecurityCard())
	}
	visible := operation("or", permits("organizations:read"), permits("roles:read"))
	shortcuts.Layout.Visible = &visible
	columns := ar.LayoutProps{Gap: "16px", Columns: "minmax(0, 1fr)", Align: "start",
		Responsive: map[string]ar.LayoutProps{"lg": {Columns: "minmax(0, 1.9fr) minmax(240px, 1fr)"}}}
	return layoutNode("analytics-overview", ar.LayoutProps{Gap: "16px"},
		layoutNode("analytics-charts", columns, trendCard, distributionCard),
		layoutNode("analytics-details", columns, accountsCard, shortcuts),
	)
}

func analyticsDataComponent(id, kind, path string, props viewProps) ar.Node {
	node := componentNode(id, kind, "$stats", props)
	componentOf(node).Bindings = []ar.Mapping{mapping("items", fieldPath("data.stats."+path))}
	return node
}

func analyticsCard(id, dataField, class string, children ...ar.Node) ar.Node {
	card := panel(id, "admin-panel "+class, children...)
	visible := operation("exists", fieldPath("data.stats."+dataField))
	card.Layout.Visible = &visible
	return card
}

func analyticsHeading(id, title, detail string, action viewProps) ar.Node {
	props := viewProps{"titleKey": title, "level": 2}
	if detail != "" {
		props["descriptionKey"] = detail
	}
	if action != nil {
		props["action"] = action
	}
	return componentNode(id, "content-heading", "", props)
}

func analyticsLink(id, key, href, permission string) viewProps {
	return viewProps{"id": id, "labelKey": key, "href": constant(href), "permission": permission, "className": "subtle-link"}
}

func analyticsShortcut(id, key, href, permission string) viewProps {
	link := analyticsLink(id, key, href, permission)
	link["visible"] = same("record.id", id)
	link["className"] = "panel-text-link"
	return link
}

func analyticsActivityPanel(updated bool) ar.Node {
	queryLink := func(prefix string) ar.Expression {
		return operation("concat", constant(prefix), operation("encode", fieldPath("record.code")))
	}
	activityHref := operation("if", same("record.kind", "account"), queryLink("/accounts?q="),
		operation("if", same("record.kind", "organization"), queryLink("/organizations?q="),
			operation("if", same("record.kind", "department"),
				operation("if", permits("organizations:read"), queryLink("/organizations?tab=departments&q="), constant("")),
				operation("concat", constant("/permissions?type=role&id="), fieldPath("record.id")))))
	list := analyticsDataComponent("analytics-activity-list", "item-list", "activity", viewProps{
		"keyField": "key", "emptyKey": "admin:overview.noRecentActivity", "itemClassName": "protocol-activity-item",
		"rowHref": activityHref,
		"fields": []viewProps{
			{"path": "name", "format": "strong"},
			{"path": "kind", "labelPrefix": "admin:overview.activityKind.", "format": "small"},
		},
		"tail": []viewProps{{"path": "createdAt", "format": "shortDate"}},
	})
	if updated {
		componentOf(list).Props["leadingIcon"] = "plus"
		componentOf(list).Props["linkArrow"] = true
	}
	return panel("analytics-activity", "admin-panel recent-panel",
		analyticsHeading("activity-heading", "admin:overview.tabActivity", "admin:overview.scopeNote", nil), list)
}

func analyticsDistributionCard(distribution ar.Node) ar.Node {
	heading := analyticsHeading("distribution-heading", "admin:overview.distribution", "admin:departments.total", nil)
	componentOf(heading).Bindings = []ar.Mapping{mapping("descriptionArgs", objectValue(
		mapping("count", fieldPath("data.stats.departments")),
	))}
	action := analyticsLink("departments", "admin:departments.title", "/organizations?tab=departments", "organizations:read")
	action["className"] = "panel-text-link"
	action["linkArrow"] = true
	footer := componentNode("analytics-departments-link", "button", "", viewProps{"action": action})
	pie := analyticsDataComponent("department-distribution-pie", "distribution-pie", "distribution", viewProps{
		"labelKey":       "admin:overview.departmentShare",
		"descriptionKey": "admin:overview.departmentShareHint",
	})
	return analyticsCard("analytics-distribution", "distribution", "distribution-panel", heading, distribution, pie, footer)
}

func analyticsShortcutDetails(id, key, href, permission, icon string) viewProps {
	link := analyticsShortcut(id, key, href, permission)
	link["className"] = "protocol-shortcut-link"
	link["icon"] = icon
	return link
}

func analyticsSecurityCard() ar.Node {
	link := analyticsLink("manage-access", "admin:actions.manageAccess", "/permissions", "roles:read")
	link["className"] = "protocol-callout-link"
	link["linkArrow"] = true
	heading := componentNode("analytics-security-heading", "content-heading", "", viewProps{
		"className": "protocol-callout-heading", "level": 3,
		"eyebrowKey": "admin:overview.security", "icon": "shield",
		"titleKey": "admin:overview.securityTitle", "descriptionKey": "admin:overview.securityHint",
	})
	card := panel("analytics-security", "access-callout", heading,
		componentNode("analytics-security-link", "button", "", viewProps{"action": link}))
	visible := permits("roles:read")
	card.Layout.Visible = &visible
	return card
}

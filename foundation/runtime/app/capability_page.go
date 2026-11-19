package app

import (
	"encoding/json"

	ar "horizon/foundation/kernel/applicationruntime"
)

type viewProps = map[string]any

func constant(value any) ar.Expression {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return ar.Expression{Kind: "literal", Literal: string(raw)}
}

func fieldPath(path string) ar.Expression {
	return ar.Expression{Kind: "path", Path: path}
}

func operation(kind string, items ...ar.Expression) ar.Expression {
	return ar.Expression{Kind: kind, Items: items}
}

func objectValue(fields ...ar.Mapping) ar.Expression {
	return ar.Expression{Kind: "object", Fields: fields}
}

func mapping(target string, value ar.Expression) ar.Mapping {
	return ar.Mapping{Target: target, Value: value}
}

func permits(permission string) ar.Expression {
	return operation("in", constant(permission), fieldPath("context.permissions"))
}

func layoutNode(id string, props ar.LayoutProps, children ...ar.Node) ar.Node {
	return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id, Kind: "grid", Props: props, Children: children}}
}

func componentNode(id, kind, data string, props viewProps, events ...ar.EventBinding) ar.Node {
	return layoutNode(id+"-layout", ar.LayoutProps{}, ar.Node{Type: "component", Component: &ar.ComponentSpec{
		ID: id, Kind: kind, Version: "1.0", Data: data, Props: props, Events: events,
	}})
}

func componentOf(node ar.Node) *ar.ComponentSpec {
	return node.Layout.Children[0].Component
}

func panel(id, className string, children ...ar.Node) ar.Node {
	node := componentNode(id, "card", "", viewProps{"className": className})
	componentOf(node).Content = layoutNode(id+"-content", ar.LayoutProps{}, children...).Layout
	return node
}

func same(path string, value any) ar.Expression {
	return operation("eq", fieldPath(path), constant(value))
}

func labeled(path, prefix string) viewProps {
	return viewProps{"path": path, "labelPrefix": prefix}
}

func plain(path string) viewProps { return viewProps{"path": path} }

func column(key string, fields ...viewProps) viewProps {
	return viewProps{"labelKey": "capability:" + key, "fields": fields}
}

func action(id, key, style, icon string) viewProps {
	return viewProps{"id": id, "labelKey": "capability:" + key, "className": style, "icon": icon, "iconOnly": icon == "refresh"}
}

func heading(id, title, description string, control viewProps, events ...ar.EventBinding) ar.Node {
	props := viewProps{"titleKey": "capability:" + title, "className": "ai-chat-heading"}
	if description != "" {
		props["descriptionKey"] = "capability:" + description
	}
	if control != nil {
		props["action"] = control
	}
	return componentNode(id, "content-heading", "", props, events...)
}

func captions(id, data string, fields ...viewProps) ar.Node {
	return componentNode(id, "content-fields", data, viewProps{"fields": fields})
}

func capabilityName(path string) viewProps {
	return viewProps{
		"value":       operation("replace", fieldPath("record."+path), constant("."), constant("_")),
		"labelPrefix": "capability:definitions.", "labelSuffix": ".title",
		"fallbackValue": fieldPath("record." + path), "format": "strong",
	}
}

func versionedName(path string) viewProps {
	return viewProps{"value": operation("concat", fieldPath("record."+path), constant("@"), fieldPath("record.version")), "format": "small"}
}

func capabilityTitle() ar.Node {
	return componentNode("heading", "page-heading", "", viewProps{
		"titleKey": "capability:title", "subtitleKey": "capability:subtitle",
		"badges": []viewProps{
			{"labelKey": "capability:summary.definitions", "className": "ai-badge", "args": viewProps{"total": operation("count", fieldPath("data.definitions"))}},
			{"labelKey": "capability:summary.events", "className": "ai-badge", "args": viewProps{"total": operation("count", fieldPath("data._events"))}},
			{"labelKey": "capability:summary.pending", "className": "ai-badge ai-status-awaiting_approval", "when": operation("not", operation("eq", operation("count", fieldPath("data._approvals")), constant(0))), "args": viewProps{"total": operation("count", fieldPath("data._approvals"))}},
		},
	})
}

func capabilityCatalog() ar.Node {
	label := capabilityName("name")
	description := viewProps{
		"value": label["value"], "labelPrefix": "capability:definitions.",
		"labelSuffix": ".description", "fallbackValue": fieldPath("record.description"), "format": "small",
	}
	name := column("catalog.columns.capability", label, description, viewProps{
		"value": operation("concat", fieldPath("record.name"), constant("@"), fieldPath("record.version")), "format": "code",
	})
	name["className"] = "ai-cell-stack"
	name["actions"] = []viewProps{{
		"id": "execution-plan", "labelKey": "capability:plans.entry", "permission": "executionPlans:read",
		"href": operation("concat", constant("/capabilities/"), operation("encode", fieldPath("record.name")),
			constant("?version="), operation("encode", fieldPath("record.version"))),
	}}
	risk := labeled("risk", "capability:risk.")
	risk["className"], risk["classPrefix"] = "ai-badge ai-risk", "is-"
	domain := column("catalog.columns.domain", plain("domain"), viewProps{"path": "entity", "labelPrefix": "entity:", "labelSuffix": ".label", "format": "small"})
	domain["className"] = "ai-cell-stack"
	return panel("catalog-card", "entity-card capability-catalog", componentNode("catalog", "content-table", "$definitions", viewProps{
		"keyFields":    []string{"name", "version"},
		"searchFields": []string{"name", "domain", "permission"}, "searchKey": "capability:catalog.search",
		"filterField": "risk", "filterLabelKey": "capability:catalog.title", "countKey": "capability:catalog.count",
		"filters":  []viewProps{{"value": "", "labelKey": "capability:catalog.title"}, {"value": "low", "labelKey": "capability:risk.low"}, {"value": "medium", "labelKey": "capability:risk.medium"}, {"value": "high", "labelKey": "capability:risk.high"}, {"value": "critical", "labelKey": "capability:risk.critical"}},
		"emptyKey": "capability:catalog.empty",
		"columns": []viewProps{name, domain, column("catalog.columns.risk", risk),
			column("catalog.columns.approval", labeled("approval", "capability:approval.")),
			column("catalog.columns.permission", viewProps{"path": "permission", "format": "code", "fallbackKey": "capability:catalog.none"}),
			column("catalog.columns.events", viewProps{"path": "successEvents", "format": "list", "fallbackKey": "capability:catalog.none"}),
		},
	}))
}

func approvalList(id, source string, title bool) ar.Node {
	props := viewProps{
		"className": "ai-pending", "itemClassName": "capability-approval", "actionsClassName": "capability-approval-actions",
		"fields": []viewProps{capabilityName("capability"), {"labelKey": "capability:approvals.expires", "format": "small",
			"args": viewProps{"time": viewProps{"expression": fieldPath("record.expiresAt"), "format": "dateTime"}}}},
		"actions": []viewProps{action("approve", "approvals.approve", "ai-button", ""), action("reject", "approvals.reject", "ai-button is-ghost", "")},
	}
	if title {
		props["titleKey"], props["emptyKey"] = "capability:approvals.title", "capability:approvals.empty"
	} else {
		actions := props["actions"].([]viewProps)
		actions[0]["labelKey"], actions[1]["labelKey"] = "ai:approval.approve", "ai:approval.reject"
		props["itemClassName"] = "flex flex-col gap-3 rounded-xl border border-border bg-muted/30 p-4"
		props["fields"] = append([]viewProps{{"labelKey": "ai:approval.title", "format": "strong"}}, props["fields"].([]viewProps)...)
	}
	events := []ar.EventBinding{}
	for _, decision := range []string{"approve", "reject"} {
		when := same("event.action", decision)
		binding := ar.EventBinding{
			Event: "action", Action: "system", Target: "capability.approval.decide", When: &when,
			Payload: objectValue(mapping("id", fieldPath("event.item.id")), mapping("approve", constant(decision == "approve"))),
		}
		if id == "run-approvals" {
			binding.ResultTarget = "_runSelection"
			result := objectValue(
				mapping("flow", fieldPath("data._runSelection.flow")),
				mapping("traceId", fieldPath("data._runSelection.traceId")),
				mapping("runId", operation("coalesce", fieldPath("event.data.runId"), fieldPath("data._runSelection.runId"))),
				mapping("invocationId", fieldPath("event.invocationId")),
				mapping("status", fieldPath("event.status")),
			)
			binding.Result = &result
		}
		events = append(events, binding)
	}
	return componentNode(id, "item-list", source, props, events...)
}

func capabilityEvents() ar.Node {
	return componentNode("events", "item-list", "$_events", viewProps{
		"titleKey": "capability:events.title", "emptyKey": "capability:events.empty",
		"className": "capability-events", "itemClassName": "capability-event", "keyField": "eventId",
		"fields": []viewProps{
			{"path": "type", "format": "strong"},
			{"labelKey": "capability:events.aggregate", "format": "small", "args": viewProps{"type": fieldPath("record.aggregateType"), "id": fieldPath("record.aggregateId")}},
			{"labelKey": "capability:events.meta", "format": "small", "args": viewProps{
				"time":   viewProps{"expression": fieldPath("record.createdAt"), "format": "dateTime"},
				"depth":  viewProps{"expression": fieldPath("record.depth"), "labelKey": "capability:events.depth"},
				"policy": viewProps{"expression": fieldPath("record.aiPolicy"), "labelKey": "capability:events.policySuffix", "skipEmpty": true},
			}},
		},
		"tail": []viewProps{{"path": "aiStatus", "labelPrefix": "capability:events.aiStatus.", "className": "ai-badge", "classPrefix": "ai-status-"}},
	})
}

func capabilityInvocations() ar.Node {
	name := column("invocations.columns.capability", capabilityName("capability"), versionedName("capability"))
	name["className"] = "ai-cell-stack"
	state := column("invocations.columns.status", viewProps{"path": "status", "labelPrefix": "capability:invocations.status.", "className": "ai-badge", "classPrefix": "ai-status-"}, viewProps{"path": "errorCode", "format": "small"})
	state["className"] = "ai-cell-stack"
	return panel("invocations-card", "entity-card capability-table-card",
		heading("invocations-heading", "invocations.title", "", nil),
		componentNode("invocations", "content-table", "$_invocations", viewProps{
			"keyField": "invocationId", "emptyKey": "capability:invocations.empty",
			"columns": []viewProps{name, column("invocations.columns.origin", labeled("origin", "capability:invocations.origin.")), state,
				column("invocations.columns.duration", viewProps{"path": "durationMs", "append": "ms"}),
				column("invocations.columns.time", viewProps{"path": "createdAt", "format": "dateTime"})},
		}),
	)
}

func CapabilityRuntimeApplication() ar.ApplicationV2 {
	const source = "builtin:capabilities"
	const version = "1.1.0"
	const permission = ar.AuthenticatedPermission
	root := layoutNode("capabilities", ar.LayoutProps{Width: "full"},
		capabilityTitle(),
		layoutNode("capability-columns", ar.LayoutProps{Columns: "minmax(0, 1fr)", Align: "start", Gap: "16px",
			Responsive: map[string]ar.LayoutProps{"xl": {Columns: "minmax(0, 1fr) 320px"}}},
			panel("activity", "entity-card ai-side", approvalList("approvals", "$_approvals", true), capabilityEvents())),
		capabilityInvocations(), capabilityCatalog(),
	)
	root.Layout.Data = []ar.DataSpec{
		{ID: "definitions", Provider: "definitions"},
		{ID: "_events", Provider: "events", PollMs: 5000, NonBlocking: true},
		{ID: "_approvals", Provider: "approvals", PollMs: 5000, NonBlocking: true},
		{ID: "_invocations", Provider: "invocations", NonBlocking: true},
	}
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "capabilities", Version: version, Source: source, RuntimeCompatibility: "3.0",
		LabelKey: "capability:title", Permission: permission, Route: "/capabilities",
		Pages: []ar.PageV3{{SchemaVersion: "3.0", ID: "capabilities.console", Route: "/capabilities",
			TitleKey: "capability:title", SubtitleKey: "capability:subtitle", Permission: permission, Root: root}},
		Providers: []ar.Provider{
			{ID: "definitions", Adapter: "system.capability-definitions"},
			{ID: "events", Adapter: "system.capability-events"},
			{ID: "approvals", Adapter: "system.capability-approvals"},
			{ID: "invocations", Adapter: "system.capability-invocations"},
		},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: version, PluginVersion: version,
		Source: source, RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: permission, Route: protocol.Route,
	}
}

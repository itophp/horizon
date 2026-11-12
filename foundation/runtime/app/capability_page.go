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

func recordCreatedNotice() ar.Node {
	notice := componentNode("record-notice", "content-notice", "", viewProps{
		"labelKey": "capability:records.created",
		"args":     viewProps{"id": fieldPath("data._createdRecord")},
	}, ar.EventBinding{Event: "dismiss", Action: "setData", Target: "_createdRecord",
		Payload: constant(nil)})
	created := operation("exists", fieldPath("data._createdRecord"))
	componentOf(notice).Visible = &created
	return notice
}

func exampleRecords() ar.Node {
	create := componentNode("record-create", "inline-input", "", viewProps{
		"inputId": "record-title", "labelKey": "capability:records.titleLabel", "placeholderKey": "capability:records.titlePlaceholder",
		"submitKey": "capability:records.create", "pendingKey": "capability:records.creating", "className": "capability-create", "buttonClassName": "ai-button",
	}, ar.EventBinding{Event: "submit", Action: "invoke", Target: "example.record.create@1.0.0",
		Payload: objectValue(mapping("title", fieldPath("event.value"))), ResultTarget: "_createdRecord", Result: &ar.Expression{Kind: "path", Path: "event.data.id"}})
	createAllowed := permits("example:create")
	componentOf(create).Visible = &createAllowed

	archive := action("archive", "records.archive", "ai-button is-ghost", "archive")
	archive["permission"], archive["pendingKey"] = "example:archive", "capability:records.archiving"
	archive["visible"] = operation("not", same("record.status", "archived"))
	key := operation("concat", constant("archive:"), fieldPath("event.item.id"))
	table := componentNode("record-table", "content-table", "$_records", viewProps{
		"emptyKey": "capability:records.empty",
		"columns": []viewProps{
			column("records.columns.id", plain("id")), column("records.columns.title", plain("title")),
			column("records.columns.status", viewProps{"path": "status", "labelPrefix": "capability:status.", "className": "ai-badge", "classPrefix": "record-"}),
			{"labelKey": "capability:records.columns.actions", "actions": []viewProps{archive}},
		},
	}, ar.EventBinding{Event: "action", Action: "invoke", Target: "example.record.archive@1.0.0",
		Payload: objectValue(mapping("id", fieldPath("event.item.id"))), IdempotencyKey: &key,
		ResultTarget: "_createdRecord", Result: &ar.Expression{Kind: "literal", Literal: "null"}})
	read := permits("example:read")
	componentOf(table).Visible = &read
	forbidden := captions("records-forbidden", "", viewProps{"labelKey": "capability:forbidden", "className": "gov-empty"})
	noRead := operation("not", read)
	componentOf(forbidden).Visible = &noRead
	return panel("records-card", "entity-card capability-records",
		heading("records-heading", "records.title", "records.hint", action("refresh", "invocations.title", "ai-button is-ghost", "refresh"),
			ar.EventBinding{Event: "action", Action: "refresh", Target: "_records"}),
		create, table, forbidden,
	)
}

func workflowExample() ar.Node {
	canStart := operation("and", permits("orchestration:run"), permits("orchestration:read"),
		permits("example:read"), permits("example:create"), permits("example:archive"))
	modelReady := operation("and", permits("assistant:use"), fieldPath("data._model.enabled"), fieldPath("data._model.configured"))
	running := operation("or",
		operation("in", fieldPath("data._run.status"), constant([]string{"running", "awaiting_approval"})),
		operation("and", operation("exists", fieldPath("data._runSelection.runId")), operation("not", operation("exists", fieldPath("data._run.status")))),
		operation("and", same("data._runSelection.status", "awaiting_approval"), operation("not", operation("exists", fieldPath("data._runSelection.runId")))),
	)
	controls := []ar.Node{}
	for _, option := range []struct{ flow, key string }{{"example-create-archive", "demo.basic"}, {"example-lifecycle", "demo.full"}} {
		control := componentNode(option.flow, "button", "", viewProps{"action": action(option.flow, option.key, "ai-button", "play")},
			ar.EventBinding{Event: "action", Action: "invoke", Target: "orchestration.workflow.run@1.0.0",
				Payload: objectValue(mapping("flow", constant(option.flow)), mapping("version", constant("1.0.0")),
					mapping("input", objectValue(mapping("title", fieldPath("event.label"))))),
				IdempotencyKey: &ar.Expression{Kind: "path", Path: "event.operationKey"},
				ResultTarget:   "_runSelection",
				Result: &ar.Expression{Kind: "object", Fields: []ar.Mapping{
					mapping("flow", constant(option.flow)), mapping("traceId", fieldPath("event.traceId")),
					mapping("runId", fieldPath("event.data.runId")), mapping("invocationId", fieldPath("event.invocationId")),
					mapping("status", fieldPath("event.status")),
				}},
			})
		componentOf(control).Props["action"].(viewProps)["payloadLabelKey"] = "capability:demo.recordTitle"
		componentOf(control).Visible = &canStart
		disabled := running
		if option.flow == "example-lifecycle" {
			disabled = operation("or", running, operation("not", modelReady))
			componentOf(control).Props["action"].(viewProps)["className"] = "ai-button is-ghost"
		}
		componentOf(control).Bindings = []ar.Mapping{mapping("disabled", disabled)}
		controls = append(controls, control)
	}
	cancel := componentNode("run-cancel", "button", "", viewProps{"action": action("cancel", "demo.cancel", "ai-button is-ghost", "stop")},
		ar.EventBinding{Event: "action", Action: "invoke", Target: "orchestration.workflow.cancel@1.0.0",
			Payload:        objectValue(mapping("runId", fieldPath("data._runSelection.runId"))),
			IdempotencyKey: &ar.Expression{Kind: "concat", Items: []ar.Expression{constant("workflow-demo:cancel:"), fieldPath("data._runSelection.runId")}},
		})
	canCancel := operation("and", permits("orchestration:cancel"), running, operation("exists", fieldPath("data._runSelection.runId")))
	componentOf(cancel).Visible = &canCancel
	controls = append(controls, cancel)
	toolbar := layoutNode("run-actions", ar.LayoutProps{Columns: "repeat(auto-fit, minmax(min(100%, 145px), max-content))", Gap: "10px"}, controls...)
	hints := captions("run-hints", "",
		viewProps{"labelKey": "capability:demo.forbidden", "className": "gov-empty", "when": operation("not", canStart)},
		viewProps{"labelKey": "capability:demo.fullReady", "className": "workflow-example-hint", "when": operation("and", canStart, modelReady)},
		viewProps{"labelKey": "capability:demo.modelRequired", "className": "workflow-example-hint", "when": operation("and", canStart, operation("not", modelReady))},
	)
	result := captions("run-status", "$_run",
		viewProps{"labelKey": "capability:demo.run", "format": "strong", "when": operation("exists", fieldPath("data._runSelection.runId")), "args": viewProps{"id": fieldPath("data._runSelection.runId")}},
		viewProps{"labelKey": "capability:demo.pendingStart", "when": same("data._runSelection.status", "awaiting_approval")},
		viewProps{"value": fieldPath("data._run.status"), "labelPrefix": "ai:runStatus.", "className": "ai-badge", "classPrefix": "ai-status-"},
	)
	matchedTrace := operation("find", fieldPath("data._trace"), operation("and",
		operation("eq", fieldPath("record.correlationId"), fieldPath("data._runSelection.traceId")),
		operation("eq", fieldPath("record.capability"), fieldPath("event.capability")),
		operation("or", operation("not", operation("exists", fieldPath("event.origin"))),
			operation("eq", fieldPath("record.origin"), fieldPath("event.origin"))),
	))
	stages := componentNode("run-stages", "item-list", "$_trace", viewProps{
		"ordered": true, "keyField": "label", "className": "workflow-example-stages",
		"fields": []viewProps{{"path": "label", "labelPrefix": "capability:demo.stages."}},
		"tail": []viewProps{{
			"value": operation("get", matchedTrace, constant("status")), "labelPrefix": "capability:invocations.status.",
			"fallbackKey": "capability:demo.waiting", "className": "ai-badge",
		}},
	})
	basic := []viewProps{{"label": "create", "capability": "example.record.create"}, {"label": "archive", "capability": "example.record.archive"}}
	full := []viewProps{basic[0], {"label": "ready", "capability": "example.record.get", "origin": "workflow"}, {"label": "review", "capability": "example.record.get", "origin": "agent"}, basic[1]}
	componentOf(stages).Bindings = []ar.Mapping{mapping("items", operation("if",
		same("data._runSelection.flow", "example-lifecycle"), constant(full), constant(basic),
	))}
	approvals := approvalList("run-approvals", "$_approvals", false)
	componentOf(approvals).Bindings = []ar.Mapping{mapping("items", operation("filter", fieldPath("data._approvals"),
		operation("or", operation("eq", fieldPath("record.invocationId"), fieldPath("data._runSelection.invocationId")),
			operation("exists", operation("find", fieldPath("data._trace"), operation("and",
				operation("eq", fieldPath("record.invocationId"), fieldPath("event.invocationId")),
				operation("eq", fieldPath("record.correlationId"), fieldPath("data._runSelection.traceId")),
			))),
		),
	))}
	outcomes := captions("run-outcome", "",
		viewProps{"labelKey": "capability:demo.approvalHint", "when": same("data._run.status", "awaiting_approval")},
		viewProps{"labelKey": "capability:demo.completed", "when": same("data._run.status", "completed"), "className": "workflow-example-success"},
		viewProps{"labelKey": "capability:demo.failed", "when": same("data._run.status", "failed"), "args": viewProps{"code": fieldPath("data._run.errorCode")}},
		viewProps{"labelKey": "capability:demo.cancelled", "when": same("data._run.status", "cancelled")},
	)
	run := layoutNode("run-details", ar.LayoutProps{Gap: "12px"}, result, stages, outcomes, approvals)
	hasRun := operation("and", operation("exists", fieldPath("data._runSelection")), operation("not", same("data._runSelection.status", "cancelled")))
	run.Layout.Visible = &hasRun
	body := layoutNode("run-body", ar.LayoutProps{Padding: "16px 20px", Gap: "10px"}, toolbar, hints, run)
	return panel("workflow-example", "entity-card workflow-example",
		heading("workflow-heading", "demo.title", "demo.hint", action("refresh", "demo.refresh", "ai-button is-ghost", "refresh"),
			ar.EventBinding{Event: "action", Action: "refresh", Target: "_trace"}),
		body,
	)
}

// CapabilityRuntimeApplication contributes the business-capability console.
func CapabilityRuntimeApplication() ar.ApplicationV2 {
	const source = "builtin:capabilities"
	const version = "1.0.1"
	const permission = ar.AuthenticatedPermission
	root := layoutNode("capabilities", ar.LayoutProps{Width: "full"},
		capabilityTitle(), recordCreatedNotice(), workflowExample(),
		layoutNode("capability-columns", ar.LayoutProps{Columns: "minmax(0, 1fr)", Align: "start", Gap: "16px",
			Responsive: map[string]ar.LayoutProps{"xl": {Columns: "minmax(0, 1fr) 320px"}}},
			exampleRecords(), panel("activity", "entity-card ai-side", approvalList("approvals", "$_approvals", true), capabilityEvents())),
		capabilityInvocations(), capabilityCatalog(),
	)
	runAvailable := operation("exists", fieldPath("data._runSelection.runId"))
	traceAvailable := operation("exists", fieldPath("data._runSelection.traceId"))
	tracePolling := operation("or", operation("not", operation("exists", fieldPath("data._run.status"))),
		operation("in", fieldPath("data._run.status"), constant([]string{"running", "awaiting_approval"})))
	activeRun := operation("or", operation("not", operation("exists", fieldPath("record.status"))),
		operation("in", fieldPath("record.status"), constant([]string{"running", "awaiting_approval"})))
	root.Layout.Data = []ar.DataSpec{
		{ID: "definitions", Provider: "definitions"},
		{ID: "_records", Provider: "records", NonBlocking: true},
		{ID: "_events", Provider: "events", PollMs: 5000, NonBlocking: true},
		{ID: "_approvals", Provider: "approvals", PollMs: 5000, NonBlocking: true},
		{ID: "_invocations", Provider: "invocations", NonBlocking: true},
		{ID: "_model", Provider: "model", NonBlocking: true},
		{ID: "_runSelection"}, {ID: "_createdRecord"},
		{ID: "_run", Provider: "run", NonBlocking: true, EnabledWhen: &runAvailable, PollWhen: &activeRun, PollMs: 2000,
			Input: []ar.Mapping{mapping("runId", fieldPath("data._runSelection.runId"))}},
		{ID: "_trace", Provider: "invocations", NonBlocking: true, EnabledWhen: &traceAvailable, PollWhen: &tracePolling, PollMs: 2000,
			Input: []ar.Mapping{mapping("limit", constant(100))}},
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
			{ID: "model", Adapter: "system.ai-status"},
			{ID: "records", Capability: ar.Ref("example.record.list"), KeyField: "id", ResultPath: "records",
				Input: []ar.Mapping{mapping("limit", constant(30))}},
			{ID: "run", Capability: ar.Ref("orchestration.workflow.get"), KeyField: "runId",
				Input: []ar.Mapping{mapping("runId", fieldPath("data._runSelection.runId"))}},
		},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: version, PluginVersion: version,
		Source: source, RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: permission, Route: protocol.Route,
	}
}

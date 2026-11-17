package automation

import ar "horizon/foundation/kernel/applicationruntime"

// applicationVersion is the built-in Runtime deployment version. A published
// Runtime definition is immutable, so changing the task form (the AI form
// assistant now fills the name and schedule too) requires a new deployment
// version; legacy installations move over through migration and the Runtime
// installation upgrade flow.
const applicationVersion = "1.0.1"

// taskRuntimeForm supplies AI suggestions to the editable Markdown draft and
// to the schedule. The user describes the job in one line (or Markdown); the
// assistant fills the name, schedule kind, wall-clock component and time zone
// as well, because a task the user has to re-schedule by hand is not really
// created from the description. Every suggested value is still a plain string
// validated by the suggesting capability and revalidated by the create/update
// capability on save.
func taskRuntimeForm() ar.FormV2 {
	visible := func(kind string) ar.Expression {
		return ar.Condition("eq", ar.Path("form.scheduleKind"), ar.Literal(`"`+kind+`"`))
	}
	visibleIn := func(kinds string) ar.Expression {
		return ar.Condition("in", ar.Path("form.scheduleKind"), ar.Literal(kinds))
	}
	fields := []ar.FieldV2{
		{Name: "brief", LabelKey: "automation:brief", Type: "string", Component: "textarea", ComponentVersion: "2.0", MaxLength: 2000},
		{Name: "name", LabelKey: "automation:name", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: maxNameLength},
		{Name: "scheduleKind", LabelKey: "automation:schedule", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: ScheduleKinds, ValueLabelPrefix: "automation:scheduleKinds.", Default: ar.Literal(`"once"`)},
		{Name: "description", LabelKey: "automation:description", Type: "string", Component: "markdown", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: maxDescriptionLength},
		{Name: "runAt", LabelKey: "automation:runAt", Type: "datetime", Component: "datetime", ComponentVersion: "2.0", Required: true, VisibleWhen: visible(ScheduleOnce)},
		{Name: "time", LabelKey: "automation:time", Type: "time", Component: "time", ComponentVersion: "2.0", Required: true, Default: ar.Literal(`"09:00"`), VisibleWhen: visibleIn(`["daily","weekly","monthly"]`)},
		{Name: "minute", LabelKey: "automation:minute", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: numberRange(0, 59), Default: ar.Literal(`"0"`), VisibleWhen: visible(ScheduleHourly)},
		{Name: "second", LabelKey: "automation:second", Type: "enum", Component: "enum", ComponentVersion: "2.0", Enum: numberRange(0, 59), Default: ar.Literal(`"0"`), VisibleWhen: visible(ScheduleHourly)},
		{Name: "weekday", LabelKey: "automation:weekday", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: weekdays(), ValueLabelPrefix: "automation:weekdays.", Default: ar.Literal(`"1"`), VisibleWhen: visible(ScheduleWeekly)},
		{Name: "dayOfMonth", LabelKey: "automation:dayOfMonth", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: numberRange(1, 31), Default: ar.Literal(`"1"`), VisibleWhen: visible(ScheduleMonthly)},
		{Name: "timezone", LabelKey: "automation:timezone", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: Timezones, Default: ar.Literal(`"Asia/Shanghai"`)},
	}
	form := ar.FormV2{
		ID: "automation.task.form", LabelKey: "automation:heading", Tags: []string{"automation"},
		Create: ar.Command{ActionType: "capability", Capability: ar.Ref("automation.task.create"), Fields: fields},
		Edit:   ar.Command{ActionType: "capability", Capability: ar.Ref("automation.task.update"), Fields: fields},
		AIAssistant: &ar.AIFormAssistant{
			LabelKey: "automation:ai.fillTask", Action: "suggest-description",
			InputFields: []string{"brief"},
			TargetFields: []string{
				"name", "scheduleKind", "description", "timezone",
				"runAt", "time", "weekday", "dayOfMonth", "minute", "second",
			},
			Prompt: "用户会用一句话或 Markdown 描述要自动完成的任务，可能包含执行时间或周期。请按 suggestions 补全自动化任务表单，每个字段一条建议：\n" +
				"- name：任务名称，用用户的语言概括对象与动作，不超过 60 个字符。\n" +
				"- scheduleKind：只能是 once、hourly、daily、weekly、monthly。用户说「每天」用 daily；「每周X」用 weekly；「每月X号」用 monthly；「每小时」用 hourly；只执行一次或给出单个日期时间用 once。用户没有提供任何时间信息时用 once，且不要编造执行时间。\n" +
				"- timezone：IANA 时区，只能取目标字段 enum 中的值；用户未说明时用 Asia/Shanghai。\n" +
				"- once 时返回 runAt：格式 YYYY-MM-DDTHH:MM，按 timezone 解释，不带偏移；用户说「今天/明天/后天/下周X」时按 currentTime 推算日期，runAt 必须晚于 currentTime。\n" +
				"- daily、weekly、monthly 时返回 time：格式 HH:MM（24 小时制）；weekly 还要返回 weekday（1=周一，7=周日）；monthly 还要返回 dayOfMonth（1-31）。\n" +
				"- hourly 时返回 minute（0-59）；只有用户给出秒时才返回 second（0-59）。\n" +
				"- description：可直接无人值守执行的结构化 Markdown 指令，必须包含任务目标、使用的数据或对象范围、输出形式与交付位置、成功标准、禁止事项与边界。只使用已提供的信息，不得编造事实、数字、权限或执行承诺；信息不足处写「未指定」，不要自行扩大范围；不超过 3000 字符。\n" +
				"只返回当前 scheduleKind 需要的字段；无法确定的值直接省略，不要猜测。保持用户的语言。",
			CreateScope: "$_aiTask", EditScope: "$_aiTask",
		},
	}
	for _, field := range fields {
		if field.Name == "brief" {
			continue
		}
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	return ar.Canonical(form)
}

func weekdays() []string {
	return []string{"1", "2", "3", "4", "5", "6", "7"}
}

func runForm() ar.FormV2 {
	fields := []ar.FieldV2{
		{Name: "trigger", LabelKey: "automation:trigger", Type: "enum", Component: "enum", ComponentVersion: "2.0", Enum: []string{"schedule", "manual"}, ValueLabelPrefix: "automation:triggers.", ReadOnly: true},
		{Name: "status", LabelKey: "automation:status", Type: "enum", Component: "enum", ComponentVersion: "2.0", Enum: runStatuses(), ValueLabelPrefix: "automation:statuses.", ReadOnly: true},
		{Name: "scheduledFor", LabelKey: "automation:scheduledFor", Type: "datetime", Component: "datetime", ComponentVersion: "2.0", ReadOnly: true},
		{Name: "startedAt", LabelKey: "automation:startedAt", Type: "datetime", Component: "datetime", ComponentVersion: "2.0", ReadOnly: true},
		{Name: "finishedAt", LabelKey: "automation:finishedAt", Type: "datetime", Component: "datetime", ComponentVersion: "2.0", ReadOnly: true},
		{Name: "errorCode", LabelKey: "automation:errorCode", Type: "string", Component: "string", ComponentVersion: "2.0", ReadOnly: true},
	}
	form := ar.FormV2{ID: "automation.run.form", LabelKey: "automation:history", Tags: []string{"automation"}}
	for _, field := range fields {
		form.Create.Fields = append(form.Create.Fields, field)
		form.Edit.Fields = append(form.Edit.Fields, field)
	}
	return ar.Canonical(form)
}

func runStatuses() []string {
	return []string{"queued", "running", "awaiting_approval", "completed", "failed", "skipped", "cancelled"}
}

// RuntimeApplication is the built-in automation workspace. It is a Runtime V3
// contribution so the page cannot bypass the capability Gateway and the AI form
// assistant reuses the shared suggest capability instead of a private endpoint.
func RuntimeApplication() ar.ApplicationV2 {
	list := ar.Provider{
		ID: "automation.task.list", Capability: ar.Ref("automation.task.list"), KeyField: "id",
		Pagination: "server", PageSize: 20, Tags: []string{"automation"},
		Input: []ar.Mapping{
			ar.Bind("page", "query.page"), ar.Bind("pageSize", "query.pageSize"),
			ar.Bind("keyword", "query.keyword"),
		},
	}
	task := ar.Provider{
		ID: "automation.task.get", Capability: ar.Ref("automation.task.get"), KeyField: "id",
		Tags: []string{"automation"}, Input: []ar.Mapping{ar.Bind("id", "route.id")},
	}
	runs := ar.Provider{
		ID: "automation.run.list", Capability: ar.Ref("automation.run.list"), KeyField: "id",
		Pagination: "server", PageSize: 20, Tags: []string{"automation"},
		Input: []ar.Mapping{
			ar.Bind("taskId", "route.id"), ar.Bind("page", "query.page"),
			ar.Bind("pageSize", "query.pageSize"),
		},
	}
	run := ar.Provider{
		ID: "automation.run.get", Capability: ar.Ref("automation.run.get"), KeyField: "id",
		Tags: []string{"automation"}, Input: []ar.Mapping{ar.Bind("id", "route.runId")},
	}
	aiDisabled := ar.Literal("false")
	aiScope := ar.DataSpec{ID: "_aiTask", Provider: task.ID, EnabledWhen: &aiDisabled}
	form := taskRuntimeForm()
	columns := []map[string]any{
		{"name": "name", "labelKey": "automation:name"},
		{"name": "scheduleKind", "labelKey": "automation:schedule", "display": "enumBadge", "valueLabelPrefix": "automation:scheduleKinds."},
		{"name": "enabled", "labelKey": "automation:enabled", "display": "yesno"},
		{"name": "nextRunAt", "labelKey": "automation:nextRunAt", "display": "date", "emptyKey": "automation:never"},
		{"name": "lastRunAt", "labelKey": "automation:lastRunAt", "display": "date", "emptyKey": "automation:never"},
		{"name": "failureCount", "labelKey": "automation:failureCount"},
	}
	runColumns := []map[string]any{
		{"name": "scheduledFor", "labelKey": "automation:scheduledFor", "display": "date"},
		{"name": "trigger", "labelKey": "automation:trigger", "display": "enumBadge", "valueLabelPrefix": "automation:triggers."},
		{"name": "status", "labelKey": "automation:status", "display": "enumBadge", "valueLabelPrefix": "automation:statuses."},
		{"name": "errorCode", "labelKey": "automation:errorCode"},
		{"name": "finishedAt", "labelKey": "automation:finishedAt", "display": "date", "emptyKey": "automation:never"},
	}
	taskPanel := map[string]any{
		"rowsPath": "items", "keyField": "id", "totalPath": "total", "serverPage": true,
		"serverSearch": true, "searchParam": "keyword", "searchFields": []string{"name"},
		"columns": columns, "totalKey": "automation:total",
		"createKey": "automation:create", "createPermission": WritePermission, "createIcon": "plus",
		"forms": map[string]any{"create": form, "edit": form, "view": form},
		"dialogTitles": map[string]any{
			"create": map[string]any{"titleKey": "automation:create", "descriptionKey": "automation:dialogHint", "wide": true},
			"edit":   map[string]any{"titleKey": "automation:edit", "descriptionKey": "automation:dialogHint", "wide": true},
			"view":   map[string]any{"titleKey": "automation:details", "descriptionKey": "automation:dialogHint", "wide": true},
		},
		"rowActions": []map[string]any{
			{"labelKey": "automation:open", "icon": "eye", "mode": "event", "event": "open"},
			{"labelKey": "automation:edit", "icon": "pencil", "mode": "edit", "permission": WritePermission},
			{"labelKey": "automation:runNow", "icon": "play", "mode": "event", "event": "run", "permission": WritePermission},
			{"labelKey": "automation:enable", "icon": "play", "mode": "event", "event": "enable", "permission": WritePermission,
				"when": ar.Condition("eq", ar.Path("record.enabled"), ar.Literal("false"))},
			{"labelKey": "automation:disable", "icon": "pause", "mode": "event", "event": "disable", "permission": WritePermission,
				"when": ar.Condition("eq", ar.Path("record.enabled"), ar.Literal("true"))},
			{"labelKey": "automation:delete", "icon": "trash", "mode": "event", "event": "remove", "permission": WritePermission,
				"confirmKey": "runtime:confirm"},
		},
	}
	runPanel := map[string]any{
		"rowsPath": "items", "keyField": "id", "totalPath": "total", "serverPage": true,
		"columns": runColumns, "totalKey": "automation:runTotal",
		"rowActions": []map[string]any{{"labelKey": "automation:openRun", "icon": "eye", "mode": "event", "event": "open"}},
	}
	detailPanel := map[string]any{
		"titleField": "name", "subtitleField": "timezone", "bodyField": "description", "bodyTitleKey": "automation:description",
		"sections": []map[string]any{{"titleKey": "automation:details", "fields": []map[string]any{
			{"name": "scheduleKind", "labelKey": "automation:schedule", "display": "enumBadge", "valueLabelPrefix": "automation:scheduleKinds."},
			{"name": "timezone", "labelKey": "automation:timezone"},
			{"name": "nextRunAt", "labelKey": "automation:nextRunAt", "display": "date", "emptyKey": "automation:never"},
			{"name": "lastRunAt", "labelKey": "automation:lastRunAt", "display": "date", "emptyKey": "automation:never"},
			{"name": "failureCount", "labelKey": "automation:failureCount"},
		}}},
	}
	runDetailPanel := map[string]any{
		"titleField": "scheduledFor", "subtitleField": "trigger", "statusField": "status",
		"statusValuePrefix": "automation:statuses.", "bodyField": "answer", "bodyTitleKey": "automation:answer",
		"sections": []map[string]any{{"titleKey": "automation:runDetails", "fields": []map[string]any{
			{"name": "trigger", "labelKey": "automation:trigger", "display": "enumBadge", "valueLabelPrefix": "automation:triggers."},
			{"name": "status", "labelKey": "automation:status", "display": "status"},
			{"name": "errorCode", "labelKey": "automation:errorCode", "emptyKey": "automation:noError"},
			{"name": "startedAt", "labelKey": "automation:startedAt", "display": "date", "emptyKey": "automation:never"},
			{"name": "finishedAt", "labelKey": "automation:finishedAt", "display": "date", "emptyKey": "automation:never"},
			{"name": "toolCalls", "labelKey": "automation:toolCalls"},
		}}},
	}

	page := func(id, route, titleKey, subtitleKey, permission string, data []ar.DataSpec, children []ar.Node) ar.PageV3 {
		return ar.PageV3{SchemaVersion: "3.0", ID: id, Route: route, TitleKey: titleKey, SubtitleKey: subtitleKey, Permission: permission, Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-root", Kind: "stack", Props: ar.LayoutProps{Gap: "20px", Width: "full"}, Data: data, Children: children}}}
	}
	heading := func(id, titleKey, subtitleKey string) ar.Node {
		return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-heading", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{ID: id + "-heading", Kind: "page-heading", Version: "1.0", Props: map[string]any{"titleKey": titleKey, "subtitleKey": subtitleKey}}}}}}
	}
	panel := func(id string, component *ar.ComponentSpec) ar.Node {
		return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-panel", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{{Type: "component", Component: component}}}}
	}

	listComponent := &ar.ComponentSpec{
		ID: "automation-task-list", Kind: "table-panel", Version: "1.0", Data: "$tasks", Props: taskPanel,
		Events: []ar.EventBinding{
			{Event: "open", Action: "navigate", Target: "automation.detail", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "run", Action: "invoke", Target: "automation.run.now@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "enable", Action: "invoke", Target: "automation.task.enable@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "disable", Action: "invoke", Target: "automation.task.disable@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "remove", Action: "invoke", Target: "automation.task.delete@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "suggest-description", Action: "invoke", Target: ar.SuggestCapability + "@1.0.0"},
		},
	}
	detailComponent := &ar.ComponentSpec{
		ID: "automation-task-detail", Kind: "detail-panel", Version: "1.0", Data: "$task", Props: detailPanel,
	}
	runComponent := &ar.ComponentSpec{
		ID: "automation-runs", Kind: "table-panel", Version: "1.0", Data: "$runs", Props: runPanel,
		Events: []ar.EventBinding{
			{Event: "open", Action: "navigate", Target: "automation.run.detail", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("runId", "event.id")}}},
		},
	}
	runDetailComponent := &ar.ComponentSpec{
		ID: "automation-run-detail", Kind: "detail-panel", Version: "1.0", Data: "$run", Props: runDetailPanel,
	}

	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "automation", Version: applicationVersion, Source: "builtin:automation",
		RuntimeCompatibility: "3.0", LabelKey: "automation:heading", Permission: ReadPermission, Route: "/automation",
		Pages: []ar.PageV3{
			page("automation.list", "/automation", "automation:heading", "automation:subtitle", ReadPermission, []ar.DataSpec{{ID: "tasks", Provider: list.ID}, aiScope}, []ar.Node{
				heading("automation-list", "automation:heading", "automation:subtitle"),
				panel("automation-list", listComponent),
			}),
			page("automation.detail", "/automation/:id", "automation:details", "automation:subtitle", ReadPermission, []ar.DataSpec{{ID: "task", Provider: task.ID}, {ID: "runs", Provider: runs.ID}}, []ar.Node{
				heading("automation-detail", "automation:details", "automation:subtitle"),
				panel("automation-detail", detailComponent),
				panel("automation-runs", runComponent),
			}),
			page("automation.run.detail", "/automation/runs/:runId", "automation:runDetails", "automation:subtitle", ReadPermission, []ar.DataSpec{{ID: "run", Provider: run.ID}}, []ar.Node{
				heading("automation-run-detail", "automation:runDetails", "automation:subtitle"),
				panel("automation-run-detail", runDetailComponent),
			}),
		},
		Providers:  []ar.Provider{list, task, runs, run},
		Navigation: []ar.Navigation{{PageID: "automation.list", LabelKey: "automation:heading", Icon: "clock"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: applicationVersion, MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}

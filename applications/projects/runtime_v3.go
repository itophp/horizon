package projects

import ar "horizon/foundation/kernel/applicationruntime"

func projectRuntimeForm() ar.FormV2 {
	createFields := []ar.FieldV2{
		{Name: "code", LabelKey: "projects:code", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 40},
		{Name: "name", LabelKey: "projects:name", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 120},
		{Name: "priority", LabelKey: "projects:priority", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: priorities, ValueLabelPrefix: "projects:priorities.", Default: ar.Literal(`"normal"`)},
		{Name: "dueDate", LabelKey: "projects:dueDate", Type: "date", Component: "date", ComponentVersion: "2.0", Default: ar.Literal(`""`)},
	}
	editFields := createFields[1:]
	form := ar.FormV2{
		ID: "project.form", LabelKey: "projects:heading", Tags: []string{"projects"},
		Create: ar.Command{ActionType: "capability", Capability: ar.Ref("project.create"), Fields: createFields},
		Edit:   ar.Command{ActionType: "capability", Capability: ar.Ref("project.update"), Fields: editFields},
	}
	for _, field := range createFields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	for _, field := range editFields {
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	return ar.Canonical(form)
}

func taskRuntimeForms(plans, members string) (createEdit ar.FormV2, assign ar.FormV2) {
	fields := []ar.FieldV2{
		{Name: "title", LabelKey: "projects:tasks.title", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 200},
		{Name: "description", LabelKey: "projects:tasks.descriptionField", Type: "string", Component: "markdown", ComponentVersion: "2.0", Default: ar.Literal(`""`)},
		{Name: "priority", LabelKey: "projects:priority", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: priorities, ValueLabelPrefix: "projects:priorities.", Default: ar.Literal(`"normal"`)},
		{Name: "dueDate", LabelKey: "projects:dueDate", Type: "date", Component: "date", ComponentVersion: "2.0", Default: ar.Literal(`""`)},
		{Name: "planId", LabelKey: "projects:tasks.plan", Type: "reference", Component: "reference", ComponentVersion: "2.0", Reference: plans, Default: ar.Literal("0")},
		{Name: "assigneeId", LabelKey: "projects:tasks.assignee", Type: "reference", Component: "reference", ComponentVersion: "2.0", Reference: members, Default: ar.Literal("0")},
	}
	createEdit = ar.FormV2{
		ID: "task.form", LabelKey: "projects:tasks.heading", Tags: []string{"tasks", "projects"},
		Create: ar.Command{ActionType: "capability", Capability: ar.Ref("project.task.create"), Fields: fields},
		Edit:   ar.Command{ActionType: "capability", Capability: ar.Ref("project.task.update"), Fields: fields},
	}
	for _, field := range fields {
		createEdit.Create.Input = append(createEdit.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		createEdit.Edit.Input = append(createEdit.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	createEdit.Create.Input = append(createEdit.Create.Input, ar.Bind("projectId", "route.id"))
	createEdit.Edit.Input = append(createEdit.Edit.Input, ar.Bind("id", "record.id"))
	createEdit.ReferencePaths = map[string]string{plans: "_plans", members: "_members"}
	assignField := fields[len(fields)-1]
	assign = ar.FormV2{
		ID: "task.assign.form", LabelKey: "projects:tasks.assign", Tags: []string{"tasks", "projects"},
		Edit: ar.Command{
			ActionType: "capability", Capability: ar.Ref("project.task.assign"),
			Fields: []ar.FieldV2{assignField}, Input: []ar.Mapping{ar.Bind("id", "record.id"), ar.Bind("assigneeId", "form.assigneeId")},
		},
		ReferencePaths: map[string]string{members: "_members"},
	}
	return ar.Canonical(createEdit), ar.Canonical(assign)
}

func planRuntimeForm(members string) ar.FormV2 {
	fields := []ar.FieldV2{
		{Name: "planType", LabelKey: "projects:plans.type", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: planTypes, ValueLabelPrefix: "projects:plans.types.", Default: ar.Literal(`"phase"`)},
		{Name: "title", LabelKey: "projects:plans.title", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 200},
		{Name: "content", LabelKey: "projects:plans.content", Type: "string", Component: "markdown", ComponentVersion: "2.0", Default: ar.Literal(`""`)},
		{Name: "priority", LabelKey: "projects:priority", Type: "enum", Component: "enum", ComponentVersion: "2.0", Required: true, Enum: priorities, ValueLabelPrefix: "projects:priorities.", Default: ar.Literal(`"normal"`)},
		{Name: "dueDate", LabelKey: "projects:dueDate", Type: "date", Component: "date", ComponentVersion: "2.0", Default: ar.Literal(`""`)},
		{Name: "ownerId", LabelKey: "projects:owner", Type: "reference", Component: "reference", ComponentVersion: "2.0", Reference: members, Required: true},
	}
	form := ar.FormV2{
		ID: "plan.form", LabelKey: "projects:plans.heading", Tags: []string{"plans", "projects"},
		Create: ar.Command{ActionType: "capability", Capability: ar.Ref("project.plan.create"), Fields: fields},
		Edit:   ar.Command{ActionType: "capability", Capability: ar.Ref("project.plan.update"), Fields: fields},
	}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Create.Input = append(form.Create.Input, ar.Bind("projectId", "route.id"))
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	form.ReferencePaths = map[string]string{members: "_members"}
	return ar.Canonical(form)
}

func componentLayout(id string, component *ar.ComponentSpec) ar.Node {
	return ar.Node{
		Type: "layout",
		Layout: &ar.LayoutSpec{
			ID: id, Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"},
			Children: []ar.Node{{Type: "component", Component: component}},
		},
	}
}

func protocolMergePanel(metadata, panel map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range metadata {
		out[key] = value
	}
	for key, value := range panel {
		out[key] = value
	}
	return out
}

func RuntimeApplication() ar.ApplicationV2 {
	return runtimeApplication("2.0.2")
}

func runtimeApplication(version string) ar.ApplicationV2 {
	ai := version != "2.0.0"
	updatedDetail := version == "2.0.2"
	projectList := ar.Provider{
		ID: "projects", Capability: ar.Ref("project.list"), KeyField: "id",
		Pagination: "server", PageSize: 20, Tags: []string{"projects"},
		Input: []ar.Mapping{
			ar.Bind("page", "query.page"), ar.Bind("pageSize", "query.pageSize"),
			ar.Bind("keyword", "query.keyword"), ar.Bind("status", "query.status"),
			ar.Bind("priority", "query.priority"), ar.Bind("sort", "query.sort"),
		},
	}
	project := ar.Provider{ID: "project", Capability: ar.Ref("project.get"), KeyField: "id", Tags: []string{"projects"}, Input: []ar.Mapping{ar.Bind("id", "route.id")}}
	tasks := ar.Provider{
		ID: "tasks", Capability: ar.Ref("project.task.list"), KeyField: "id",
		Pagination: "server", PageSize: 20, Tags: []string{"tasks", "projects"},
		Input: []ar.Mapping{
			ar.Bind("projectId", "route.id"), ar.Bind("page", "query.page"),
			ar.Bind("pageSize", "query.pageSize"), ar.Bind("keyword", "query.keyword"),
			ar.Bind("status", "query.status"),
		},
	}
	task := ar.Provider{ID: "task", Capability: ar.Ref("project.task.get"), KeyField: "id", Tags: []string{"tasks", "projects"}, Input: []ar.Mapping{ar.Bind("id", "route.id")}, PollInterval: 5000}
	members := ar.Provider{ID: "members", Capability: ar.Ref("project.member.list"), KeyField: "accountId", LabelField: "name", ResultPath: "items", Tags: []string{"members", "projects"}, Input: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("keyword", "query.referenceKeyword")}}
	memberSearch := ar.Provider{
		ID: "member.search", Capability: ar.Ref("project.member.search"), KeyField: "accountId", LabelField: "name",
		ResultPath: "items", Tags: []string{"members", "projects"},
		Input: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("keyword", "event.keyword")},
	}
	plans := ar.Provider{ID: "plans", Capability: ar.Ref("project.plan.list"), KeyField: "id", LabelField: "title", ResultPath: "items", Tags: []string{"plans", "projects"}, Input: []ar.Mapping{ar.Bind("projectId", "route.id")}}
	projectForm := projectRuntimeForm()
	planForm := planRuntimeForm(members.ID)
	taskForm, assignForm := taskRuntimeForms(plans.ID, members.ID)
	aiEvent := func(event, capability string) ar.EventBinding {
		return ar.EventBinding{Event: event, Action: "invoke", Target: capability + "@1.0.0"}
	}
	if ai {
		taskForm.AIAssistant = &ar.AIFormAssistant{
			LabelKey: "projects:ai.draftTaskDescription", Action: "suggest-task-description",
			InputFields: []string{"title", "priority", "dueDate"}, TargetFields: []string{"description"},
			Prompt:      "根据任务标题、优先级和截止日期起草简短的任务描述。仅使用已提供的信息，信息不足时返回空建议。",
			CreateScope: "$_aiProject", EditScope: "$_aiTask",
		}
	}

	page := func(id, route, titleKey, subtitleKey, permission string, data []ar.DataSpec, children []ar.Node) ar.PageV3 {
		return ar.PageV3{SchemaVersion: "3.0", ID: id, Route: route, TitleKey: titleKey, SubtitleKey: subtitleKey, Permission: ReadPermission, Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-root", Kind: "stack", Props: ar.LayoutProps{Gap: "20px", Width: "full"}, Data: data, Children: children}}}
	}
	heading := func(id, titleKey, subtitleKey string, actions []map[string]any) ar.Node {
		props := map[string]any{"titleKey": titleKey, "subtitleKey": subtitleKey}
		if actions != nil {
			props["actions"] = actions
		}
		return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-heading", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{ID: id + "-heading", Kind: "page-heading", Version: "1.0", Props: props}}}}}
	}
	panel := func(id string, dataRef string, spec map[string]any, events []ar.EventBinding) ar.Node {
		return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-panel", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{ID: id, Kind: "table-panel", Version: "1.0", Data: dataRef, Props: spec, Events: events}}}}}
	}
	openTaskEvent := ar.EventBinding{Event: "open", Action: "navigate", Target: "task.detail", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("projectId", "event.projectId"), ar.Bind("id", "event.id")}}}
	taskStatusEvent := func(event, status string) ar.EventBinding {
		return ar.EventBinding{Event: event, Action: "invoke", Target: "project.task.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"`+status+`"`)}}}
	}

	projectColumns := []map[string]any{
		{"name": "code", "labelKey": "projects:code", "display": "mono"},
		{"name": "name", "labelKey": "projects:name"},
		{"name": "status", "labelKey": "projects:status", "display": "enumBadge", "valueLabelPrefix": "projects:states."},
		{"name": "priority", "labelKey": "projects:priority", "display": "enumBadge", "valueLabelPrefix": "projects:priorities."},
		{"name": "ownerName", "labelKey": "projects:owner"},
		{"name": "taskCount", "labelKey": "projects:progress", "display": "progress", "field": "taskCount", "templateKey": "projects:ratio"},
	}
	listEvents := []ar.EventBinding{{Event: "open", Action: "navigate", Target: "project.detail", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}}}
	listData := []ar.DataSpec{{ID: "projects", Provider: projectList.ID}}
	if ai {
		projectColumns = append(projectColumns, map[string]any{
			"name": "aiRisk", "labelKey": "projects:ai.risk", "display": "ai",
			"analysis": ar.AIAnalysisSpec{Action: "analyze-risk-column",
				Input:  []ar.AIDataInput{{Source: "$_aiProject", Fields: []string{"status", "dueDate", "taskOverdueCount"}}},
				Prompt: "根据项目状态、截止日期和逾期任务数判断风险。label 必须是 high、medium、low 或 insufficient。用一句话说明主要依据，信息不足时说明缺少哪些数据。",
			},
		})
		listEvents = append(listEvents, aiEvent("analyze-risk-column", ar.AnalyzeCapability))
		listData = append(listData, ar.DataSpec{ID: "_aiProject", Provider: project.ID, EnabledWhen: aiDisabledBinding()})
	}
	projectListPanel := map[string]any{
		"rowsPath": "items", "keyField": "id", "totalPath": "total", "serverPage": true,
		"serverSearch": true, "searchParam": "keyword", "searchFields": []string{"code", "name"},
		"tabs": map[string]any{"param": "status", "field": "status", "options": append([]map[string]any{{"value": "all", "labelKey": "projects:all"}}, func() []map[string]any {
			out := []map[string]any{}
			for _, status := range projectStates {
				out = append(out, map[string]any{"value": status, "labelKey": "projects:states." + status})
			}
			return out
		}()...)},
		"select": map[string]any{"param": "priority", "field": "priority", "options": append([]map[string]any{{"value": "all", "labelKey": "projects:all"}}, func() []map[string]any {
			out := []map[string]any{}
			for _, priority := range priorities {
				out = append(out, map[string]any{"value": priority, "labelKey": "projects:priorities." + priority})
			}
			return out
		}()...)},
		"columns": projectColumns, "totalKey": "projects:total", "createKey": "projects:create",
		"createPermission": WritePermission, "createIcon": "plus", "forms": map[string]any{"create": projectForm, "edit": projectForm, "view": projectForm},
		"dialogTitles": map[string]any{
			"create": map[string]any{"titleKey": "projects:create", "descriptionKey": "projects:dialog.projectHint"},
			"edit":   map[string]any{"titleKey": "projects:edit", "descriptionKey": "projects:dialog.projectEditHint"},
			"view":   map[string]any{"titleKey": "projects:heading", "descriptionKey": "projects:description"},
		},
		"rowActions": []map[string]any{{"labelKey": "projects:open", "icon": "eye", "mode": "event", "event": "open"}},
	}
	projectDetailPanel := map[string]any{
		"titleField": "name", "subtitleField": "code", "statusField": "status",
		"statusValuePrefix": "projects:states.",
		"sections": []map[string]any{
			{"titleKey": "projects:tabs.overview", "fields": []map[string]any{
				{"name": "code", "labelKey": "projects:code", "display": "mono"},
				{"name": "priority", "labelKey": "projects:priority", "display": "enumBadge", "valueLabelPrefix": "projects:priorities."},
				{"name": "ownerName", "labelKey": "projects:owner"},
				{"name": "dueDate", "labelKey": "projects:dueDate", "display": "date", "emptyKey": "projects:dash"},
				{"name": "createdAt", "labelKey": "admin:fields.createdAt", "display": "date"},
			}},
			{"titleKey": "projects:progress", "fields": []map[string]any{
				{"name": "taskCount", "labelKey": "projects:tabs.tasks"},
				{"name": "taskDoneCount", "labelKey": "projects:progress"},
				{"name": "taskOverdueCount", "labelKey": "projects:overdue"},
				{"name": "planCount", "labelKey": "projects:plans.heading"},
			}},
		},
		"actions": []map[string]any{
			{"labelKey": "projects:edit", "icon": "pencil", "permission": WritePermission, "mode": "dialog", "dialog": "edit"},
			{"labelKey": "projects:statusActions.active", "icon": "play", "permission": WritePermission, "mode": "event", "event": "active", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"paused"`))},
			{"labelKey": "projects:statusActions.paused", "icon": "refresh", "permission": WritePermission, "mode": "event", "event": "paused", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"active"`))},
			{"labelKey": "projects:statusActions.completed", "icon": "check", "permission": WritePermission, "mode": "event", "event": "completed", "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["active","paused"]`))},
			{"labelKey": "projects:statusActions.archived", "icon": "reject", "permission": WritePermission, "mode": "event", "event": "archived", "confirmKey": "runtime:confirm", "when": ar.Condition("and", ar.Condition("in", ar.Path("record.status"), ar.Literal(`["active","paused","completed"]`)), ar.Condition("eq", ar.Path("record.ownerId"), ar.Path("context.accountId")))},
		},
		"forms": map[string]any{"edit": projectForm}, "dialogTitles": map[string]any{"edit": map[string]any{"titleKey": "projects:edit", "descriptionKey": "projects:dialog.projectEditHint"}},
	}
	taskColumns := []map[string]any{
		{"name": "title", "labelKey": "projects:tasks.title"},
		{"name": "status", "labelKey": "projects:status", "display": "enumBadge", "valueLabelPrefix": "projects:tasks.states."},
		{"name": "priority", "labelKey": "projects:priority", "display": "enumBadge", "valueLabelPrefix": "projects:priorities."},
		{"name": "assigneeName", "labelKey": "projects:tasks.assignee", "emptyKey": "projects:tasks.unassigned"},
		{"name": "planTitle", "labelKey": "projects:tasks.plan", "emptyKey": "projects:tasks.noPlan"},
		{"name": "dueDate", "labelKey": "projects:dueDate", "display": "date", "emptyKey": "projects:dash"},
	}
	taskPanelSpec := func(totalKey string) map[string]any {
		return map[string]any{
			"rowsPath": "items", "keyField": "id", "totalPath": "total", "serverPage": true,
			"serverSearch": true, "searchParam": "keyword", "searchFields": []string{"title"},
			"tabs": map[string]any{"param": "status", "field": "status", "options": []map[string]any{
				{"value": "all", "labelKey": "projects:all"}, {"value": "todo", "labelKey": "projects:tasks.states.todo"},
				{"value": "in_progress", "labelKey": "projects:tasks.states.in_progress"},
				{"value": "done", "labelKey": "projects:tasks.states.done"},
				{"value": "cancelled", "labelKey": "projects:tasks.states.cancelled"},
			}},
			"columns": taskColumns, "totalKey": totalKey, "createKey": "projects:tasks.new",
			"createPermission": WritePermission, "createIcon": "plus",
			"references": map[string]string{"plans": "_plans", "members": "_members"},
			"forms":      map[string]any{"create": taskForm, "edit": taskForm, "view": taskForm, "access": assignForm},
			"dialogTitles": map[string]any{
				"create": map[string]any{"titleKey": "projects:tasks.new", "descriptionKey": "projects:dialog.taskHint"},
				"edit":   map[string]any{"titleKey": "projects:edit", "descriptionKey": "projects:dialog.taskHint", "wide": true},
				"view":   map[string]any{"titleKey": "projects:tasks.detail", "descriptionKey": "projects:dialog.taskDetailHint", "wide": true},
				"access": map[string]any{"titleKey": "projects:tasks.assign", "descriptionKey": "projects:dialog.taskHint"},
			},
			"rowActions": []map[string]any{
				{"labelKey": "projects:tasks.detail", "icon": "eye", "mode": "event", "event": "open"},
				{"labelKey": "projects:edit", "icon": "pencil", "mode": "edit", "permission": WritePermission},
				{"labelKey": "projects:tasks.assign", "icon": "users", "mode": "access", "permission": WritePermission},
				{"labelKey": "projects:tasks.states.todo", "mode": "event", "event": "todo", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"done"`))},
				{"labelKey": "projects:tasks.states.in_progress", "mode": "event", "event": "in_progress", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"todo"`))},
				{"labelKey": "projects:tasks.states.done", "mode": "event", "event": "done", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"in_progress"`))},
				{"labelKey": "projects:tasks.states.cancelled", "mode": "event", "event": "cancelled", "permission": WritePermission, "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["todo","in_progress"]`))},
			},
		}
	}
	taskEvents := []ar.EventBinding{openTaskEvent, taskStatusEvent("todo", "todo"), taskStatusEvent("in_progress", "in_progress"), taskStatusEvent("done", "done"), taskStatusEvent("cancelled", "cancelled")}
	if ai {
		taskEvents = append(taskEvents, aiEvent("suggest-task-description", ar.SuggestCapability))
	}
	taskDetailPanel := map[string]any{
		"titleField": "title", "subtitleField": "planTitle", "statusField": "status",
		"statusValuePrefix": "projects:tasks.states.",
		"bodyField":         "description", "bodyTitleKey": "projects:tasks.descriptionField",
		"sections": []map[string]any{
			{"titleKey": "projects:tasks.heading", "fields": []map[string]any{
				{"name": "priority", "labelKey": "projects:priority", "display": "enumBadge", "valueLabelPrefix": "projects:priorities."},
				{"name": "assigneeName", "labelKey": "projects:tasks.assignee", "emptyKey": "projects:tasks.unassigned"},
				{"name": "planTitle", "labelKey": "projects:tasks.plan", "emptyKey": "projects:tasks.noPlan"},
				{"name": "dueDate", "labelKey": "projects:dueDate", "display": "date", "emptyKey": "projects:dash"},
				{"name": "completedAt", "labelKey": "projects:tasks.completedAt", "display": "date", "emptyKey": "projects:dash"},
			}},
			{"titleKey": "projects:tasks.brief", "fields": []map[string]any{
				{"name": "brief", "labelKey": "projects:tasks.brief", "emptyKey": "projects:tasks.briefEmpty"},
				{"name": "briefModel", "labelKey": "projects:tasks.briefHint", "emptyKey": "projects:dash"},
			}},
		},
		"actions": []map[string]any{
			{"labelKey": "projects:edit", "icon": "pencil", "permission": WritePermission, "mode": "dialog", "dialog": "edit"},
			{"labelKey": "projects:tasks.assign", "icon": "users", "permission": WritePermission, "mode": "dialog", "dialog": "access"},
			{"labelKey": "projects:tasks.states.todo", "permission": WritePermission, "mode": "event", "event": "todo", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"done"`))},
			{"labelKey": "projects:tasks.states.in_progress", "permission": WritePermission, "mode": "event", "event": "in_progress", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"todo"`))},
			{"labelKey": "projects:tasks.states.done", "permission": WritePermission, "mode": "event", "event": "done", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"in_progress"`))},
			{"labelKey": "projects:tasks.states.cancelled", "permission": WritePermission, "mode": "event", "event": "cancelled", "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["todo","in_progress"]`))},
		},
		"forms": map[string]any{"edit": taskForm, "access": assignForm},
		"dialogTitles": map[string]any{
			"edit":   map[string]any{"titleKey": "projects:edit", "descriptionKey": "projects:dialog.taskHint", "wide": true},
			"access": map[string]any{"titleKey": "projects:tasks.assign", "descriptionKey": "projects:dialog.taskHint"},
		},
	}
	planPanel := map[string]any{
		"rowsPath": "_plans", "keyField": "id", "searchFields": []string{"title", "ownerName"},
		"columns": []map[string]any{
			{"name": "title", "labelKey": "projects:plans.title"},
			{"name": "planType", "labelKey": "projects:plans.type", "display": "enumBadge", "valueLabelPrefix": "projects:plans.types."},
			{"name": "ownerName", "labelKey": "projects:owner"},
			{"name": "dueDate", "labelKey": "projects:dueDate", "display": "date", "emptyKey": "projects:dash"},
			{"name": "status", "labelKey": "projects:status", "display": "enumBadge", "valueLabelPrefix": "projects:plans.states."},
		},
		"totalKey": "projects:plansCount", "createKey": "projects:plans.new", "createPermission": WritePermission,
		"references": map[string]string{"members": "_members"},
		"forms":      map[string]any{"create": planForm, "edit": planForm, "view": planForm},
		"dialogTitles": map[string]any{
			"create": map[string]any{"titleKey": "projects:plans.new", "descriptionKey": "projects:dialog.planHint", "wide": true},
			"edit":   map[string]any{"titleKey": "projects:edit", "descriptionKey": "projects:dialog.planHint", "wide": true},
			"view":   map[string]any{"titleKey": "projects:plans.heading", "descriptionKey": "projects:dialog.planHint", "wide": true},
		},
		"detailCapability": "project.plan.get", "detailVersion": "1.0.0",
		"detailInput": []ar.Mapping{ar.Bind("id", "record.id")},
		"rowActions": []map[string]any{
			{"labelKey": "projects:edit", "icon": "pencil", "mode": "edit", "permission": WritePermission},
			{"labelKey": "projects:plans.actions.in_progress", "mode": "event", "event": "planInProgress", "permission": WritePermission, "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["pending","done"]`))},
			{"labelKey": "projects:plans.actions.done", "mode": "event", "event": "planDone", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"in_progress"`))},
			{"labelKey": "projects:plans.actions.cancelled", "mode": "event", "event": "planCancelled", "permission": WritePermission, "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["pending","in_progress"]`))},
		},
	}
	memberTable := map[string]any{
		"rowsPath": "_members", "keyField": "accountId", "searchFields": []string{"name", "email"},
		"columns": []map[string]any{
			{"name": "name", "labelKey": "admin:fields.fullName"},
			{"name": "email", "labelKey": "projects:members.email"},
			{"name": "isOwner", "labelKey": "projects:members.role", "display": "yesno"},
		},
		"totalKey": "projects:members.count", "search": false,
		"rowActions": []map[string]any{
			{"labelKey": "projects:members.makeOwner", "icon": "key", "mode": "event", "event": "transferOwner", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.isOwner"), ar.Literal(`false`))},
			{"labelKey": "projects:members.remove", "icon": "trash", "mode": "event", "event": "removeMember", "permission": WritePermission, "when": ar.Condition("eq", ar.Path("record.isOwner"), ar.Literal(`false`))},
		},
	}
	memberPanel := map[string]any{
		"provider": memberSearch.ID, "searchInput": []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("keyword", "event.keyword")},
		"existingPath": "_members", "existingKeyField": "accountId", "resultKeyField": "accountId",
		"resultTitleField": "name", "resultSubtitleField": "email", "descriptionKey": "projects:members.description",
		"searchLabelKey": "projects:members.keyword", "searchButtonKey": "projects:members.search",
		"emptyKey": "projects:members.noResult", "alreadyLabelKey": "projects:members.memberBadge",
		"addLabelKey": "projects:members.add", "addPermission": WritePermission, "addIcon": "plus",
		"addEvent": "addMember", "table": memberTable,
	}
	taskTabSpec := taskPanelSpec("projects:tabs.tasks")
	taskTabSpec["rowsPath"] = "_tasks.items"
	detailComponent := componentLayout("project-detail-tabs", &ar.ComponentSpec{
		ID: "project-detail", Kind: "page-tabs", Version: "1.0", Data: "$project",
		Props: map[string]any{
			"param": "tab",
			"panels": []map[string]any{
				protocolMergePanel(map[string]any{"id": "overview", "labelKey": "projects:tabs.overview", "icon": "grid", "type": "detail"}, projectDetailPanel),
				protocolMergePanel(map[string]any{"id": "plans", "labelKey": "projects:tabs.plans", "icon": "list", "type": "table"}, planPanel),
				protocolMergePanel(map[string]any{"id": "tasks", "labelKey": "projects:tabs.tasks", "icon": "users", "type": "table"}, taskTabSpec),
				protocolMergePanel(map[string]any{"id": "members", "labelKey": "projects:tabs.members", "icon": "users", "type": "search-picker"}, memberPanel),
			},
		},
		Events: []ar.EventBinding{
			{Event: "active", Action: "invoke", Target: "project.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"active"`)}}},
			{Event: "paused", Action: "invoke", Target: "project.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"paused"`)}}},
			{Event: "completed", Action: "invoke", Target: "project.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"completed"`)}}},
			{Event: "archived", Action: "invoke", Target: "project.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"archived"`)}}},
			{Event: "planInProgress", Action: "invoke", Target: "project.plan.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"in_progress"`)}}},
			{Event: "planDone", Action: "invoke", Target: "project.plan.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"done"`)}}},
			{Event: "planCancelled", Action: "invoke", Target: "project.plan.status.change@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("from", "event.status"), ar.Const("status", `"cancelled"`)}}},
			{Event: "addMember", Action: "invoke", Target: "project.member.add@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("accountId", "event.accountId")}}},
			{Event: "removeMember", Action: "invoke", Target: "project.member.remove@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("accountId", "event.accountId")}}},
			{Event: "transferOwner", Action: "invoke", Target: "project.owner.transfer@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("accountId", "event.accountId")}}},
			openTaskEvent,
			taskStatusEvent("todo", "todo"),
			taskStatusEvent("in_progress", "in_progress"),
			taskStatusEvent("done", "done"),
			taskStatusEvent("cancelled", "cancelled"),
		},
	})
	taskDetailComponent := componentLayout("task-detail-panel", &ar.ComponentSpec{
		ID: "task-detail", Kind: "detail-panel", Version: "1.0", Data: "$task", Props: taskDetailPanel, Events: taskEvents,
	})

	detailData := []ar.DataSpec{{ID: "project", Provider: project.ID}, {ID: "_tasks", Provider: tasks.ID}, {ID: "_members", Provider: members.ID}, {ID: "_plans", Provider: plans.ID}}
	taskListData := []ar.DataSpec{{ID: "project", Provider: project.ID}, {ID: "tasks", Provider: tasks.ID}, {ID: "_members", Provider: members.ID}, {ID: "_plans", Provider: plans.ID}}
	taskDetailData := []ar.DataSpec{
		{ID: "task", Provider: task.ID},
		{ID: "_members", Provider: members.ID, Input: []ar.Mapping{ar.Bind("projectId", "route.projectId")}},
		{ID: "_plans", Provider: plans.ID, Input: []ar.Mapping{ar.Bind("projectId", "route.projectId")}},
	}
	detailNodes := []ar.Node{detailComponent, panel("project-tasks", "$_tasks", taskPanelSpec("projects:tabs.tasks"), taskEvents)}
	if ai {
		aiScope := []ar.DataSpec{{ID: "_aiProject", Provider: project.ID, EnabledWhen: aiDisabledBinding()}, {ID: "_aiTask", Provider: task.ID, EnabledWhen: aiDisabledBinding()}}
		detailData = append(detailData, aiScope...)
		taskListData = append(taskListData, aiScope...)
		taskDetailData = append(taskDetailData, aiScope...)
		detailNodes = append(detailNodes, componentLayout("project-ai-analysis", &ar.ComponentSpec{
			ID: "project-ai-analysis", Kind: "ai-analysis-panel", Version: "1.0", Data: "$project",
			Props: map[string]any{"scenarios": []ar.AIScenario{
				{ID: "schedule-risk", LabelKey: "projects:ai.scheduleRisk", Analysis: ar.AIAnalysisSpec{
					Action: "analyze-schedule", Input: []ar.AIDataInput{{Source: "$project", Fields: []string{"status", "dueDate", "taskOverdueCount"}}},
					Prompt: "说明项目是否存在延期风险，列出支持结论的字段。信息不足时指出缺少哪些数据。",
				}},
				{ID: "task-blockers", LabelKey: "projects:ai.taskBlockers", Analysis: ar.AIAnalysisSpec{
					Action: "analyze-blockers", Input: []ar.AIDataInput{{Source: "$project", Fields: []string{"status"}}, {Source: "$_tasks", Fields: []string{"title", "status", "dueDate"}}},
					Prompt: "从提供的任务中找出最多三项可能阻塞项目推进的任务，引用任务标题和状态说明理由。",
				}},
			}},
			Events: []ar.EventBinding{aiEvent("analyze-schedule", ar.AnalyzeCapability), aiEvent("analyze-blockers", ar.AnalyzeCapability)},
		}))
	}
	if updatedDetail {
		legacyEvents := detailComponent.Layout.Children[0].Component.Events
		overviewProps := protocolMergePanel(nil, projectDetailPanel)
		overviewProps["sections"] = []map[string]any{
			{"titleKey": "projects:detail.info", "fields": []map[string]any{
				{"name": "ownerName", "labelKey": "projects:owner"},
				{"name": "priority", "labelKey": "projects:priority", "display": "enumBadge", "valueLabelPrefix": "projects:priorities."},
				{"name": "dueDate", "labelKey": "projects:dueDate", "display": "date", "emptyKey": "projects:dash"},
			}},
			{"titleKey": "projects:progress", "fields": []map[string]any{
				{"name": "taskCount", "labelKey": "projects:detail.taskTotal"},
				{"name": "taskDoneCount", "labelKey": "projects:detail.taskDone"},
				{"name": "taskOverdueCount", "labelKey": "projects:overdue"},
			}},
		}
		overview := componentLayout("project-overview", &ar.ComponentSpec{
			ID: "project-overview", Kind: "detail-panel", Version: "1.0", Data: "$project",
			Props: overviewProps, Events: projectDetailEvents(legacyEvents, "active", "paused", "completed", "archived"),
		})
		overviewNodes := append([]ar.Node{overview}, detailNodes[2:]...)
		planProps := protocolMergePanel(nil, planPanel)
		delete(planProps, "rowsPath") // The plans Provider already selects items.
		plansPanel := panel("project-plans", "$_plans", planProps, projectDetailEvents(legacyEvents, "planInProgress", "planDone", "planCancelled"))
		tasksPanel := panel("project-tasks", "$_tasks", taskPanelSpec("projects:tabs.tasks"), taskEvents)
		membersPanel := componentLayout("project-members", &ar.ComponentSpec{
			ID: "project-members", Kind: "search-picker-panel", Version: "1.0",
			Props: memberPanel, Events: projectDetailEvents(legacyEvents, "addMember", "removeMember", "transferOwner"),
		})
		detailNodes = []ar.Node{projectDetailTabs(overviewNodes, plansPanel, tasksPanel, membersPanel)}

		tab := func(id string) ar.Expression {
			return ar.Condition("eq", ar.Path("query.tab"), ar.Literal(`"`+id+`"`))
		}
		taskTab := tab("tasks")
		plansNeeded := ar.Condition("or", tab("plans"), taskTab)
		membersNeeded := ar.Condition("or", tab("members"), plansNeeded)
		for index := range detailData {
			switch detailData[index].ID {
			case "_tasks":
				detailData[index].EnabledWhen = &taskTab
			case "_plans":
				detailData[index].EnabledWhen = &plansNeeded
			case "_members":
				detailData[index].EnabledWhen = &membersNeeded
			}
		}
	}
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "projects", Version: version, Source: "com.horizon.projects",
		RuntimeCompatibility: "3.0", LabelKey: "projects:heading", Permission: ReadPermission, Route: "/projects",
		Pages: []ar.PageV3{
			page("project.list", "/projects", "projects:heading", "projects:description", ReadPermission, listData, []ar.Node{
				heading("project-list", "projects:heading", "projects:description", []map[string]any{{"labelKey": "projects:create", "icon": "plus", "permission": WritePermission, "dialog": "create"}}),
				panel("project-list", "$projects", projectListPanel, listEvents),
			}),
			page("project.detail", "/projects/:id", "projects:title", "projects:description", ReadPermission, detailData, detailNodes),
			page("project.tasks", "/projects/:id/tasks", "projects:tasks.heading", "projects:tasks.description", ReadPermission, taskListData, []ar.Node{
				heading("project-tasks", "projects:tasks.heading", "projects:tasks.description", nil),
				panel("project-task-list", "$tasks", taskPanelSpec("projects:tabs.tasks"), taskEvents),
			}),
			page("task.detail", "/projects/:projectId/tasks/:id", "projects:tasks.detail", "projects:dialog.taskDetailHint", ReadPermission, taskDetailData, []ar.Node{taskDetailComponent}),
		},
		Providers:  []ar.Provider{projectList, project, tasks, task, members, memberSearch, plans},
		Navigation: []ar.Navigation{{PageID: "project.list", LabelKey: "projects:heading", Icon: "grid"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: version, MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}

func aiDisabledBinding() *ar.Expression {
	value := ar.Literal("false")
	return &value
}

func projectDetailEvents(events []ar.EventBinding, names ...string) []ar.EventBinding {
	var selected []ar.EventBinding
	for _, event := range events {
		for _, name := range names {
			if event.Event == name {
				selected = append(selected, event)
				break
			}
		}
	}
	return selected
}

func projectDetailTabs(overview []ar.Node, plans, tasks, members ar.Node) ar.Node {
	tabPanel := func(id string, content ...ar.Node) ar.Node {
		return componentLayout("project-"+id+"-tab", &ar.ComponentSpec{
			ID: "project-" + id + "-tab", Kind: "layout-tab-panel", Version: "1.0",
			Props: map[string]any{"tab": id},
			Content: &ar.LayoutSpec{ID: "project-" + id + "-content", Kind: "stack",
				Props: ar.LayoutProps{Gap: "16px"}, Children: content},
		})
	}
	return componentLayout("project-detail-tabs", &ar.ComponentSpec{
		ID: "project-detail", Kind: "layout-tabs", Version: "1.0",
		Props: map[string]any{"param": "tab", "labelKey": "projects:heading", "tabs": []map[string]any{
			{"id": "overview", "labelKey": "projects:tabs.overview"},
			{"id": "plans", "labelKey": "projects:tabs.plans"},
			{"id": "tasks", "labelKey": "projects:tabs.tasks"},
			{"id": "members", "labelKey": "projects:tabs.members"},
		}},
		Content: &ar.LayoutSpec{ID: "project-detail-content", Kind: "stack", Props: ar.LayoutProps{Gap: "16px"},
			Children: []ar.Node{tabPanel("overview", overview...), tabPanel("plans", plans), tabPanel("tasks", tasks), tabPanel("members", members)}},
	})
}

package projects

import ar "baseline/services/api/kernel/applicationruntime"

func RuntimeApplication() ar.ApplicationV2 {
	app := ar.ApplicationV2{SchemaVersion: "2.0", ID: "projects", Version: "1.0.0", Source: "com.horizon.projects", RuntimeCompatibility: "2.0", LabelKey: "projects:heading", Permission: ReadPermission, Route: "/projects"}
	app.Providers = []ar.Provider{
		{ID: "projects", Capability: ar.Ref("project.list"), KeyField: "id", ResultPath: "items", Pagination: "server", PageSize: 20, Tags: []string{"projects"}, Input: []ar.Mapping{ar.Bind("page", "query.page"), ar.Bind("pageSize", "query.pageSize"), ar.Bind("keyword", "query.keyword"), ar.Bind("status", "query.status"), ar.Bind("priority", "query.priority"), ar.Bind("sort", "query.sort")}},
		{ID: "project", Capability: ar.Ref("project.get"), KeyField: "id", Tags: []string{"projects"}, Input: []ar.Mapping{ar.Bind("id", "route.id")}},
		{ID: "tasks", Capability: ar.Ref("project.task.list"), KeyField: "id", ResultPath: "items", Pagination: "server", PageSize: 20, Tags: []string{"tasks", "projects"}, Input: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("page", "query.page"), ar.Bind("pageSize", "query.pageSize"), ar.Bind("keyword", "query.keyword")}},
		{ID: "task", PollInterval: 5000, Capability: ar.Ref("project.task.get"), KeyField: "id", Tags: []string{"tasks"}, Input: []ar.Mapping{ar.Bind("id", "record.id")}},
		{ID: "members", Select: []string{"accountId", "name"}, Capability: ar.Ref("project.member.list"), KeyField: "accountId", LabelField: "name", ResultPath: "items", Tags: []string{"members"}, Input: []ar.Mapping{ar.Bind("projectId", "route.id"), ar.Bind("keyword", "query.referenceKeyword")}},
		{ID: "plans", Capability: ar.Ref("project.plan.list"), KeyField: "id", LabelField: "title", ResultPath: "items", Tags: []string{"plans"}, Input: []ar.Mapping{ar.Bind("projectId", "route.id")}},
	}
	fields := []ar.FieldV2{ar.TextField("code", "projects:code"), ar.TextField("name", "projects:name"), {Name: "priority", LabelKey: "projects:priority", ValueLabelPrefix: "projects:priorities.", Default: ar.Literal(`"normal"`)}, {Name: "dueDate", LabelKey: "projects:dueDate", Type: "date", Default: ar.Literal(`""`)}}
	form := ar.FormV2{ID: "project.form", LabelKey: "projects:heading", Tags: []string{"projects"}, Create: ar.Command{Capability: ar.Ref("project.create"), Fields: fields}, Edit: ar.Command{Capability: ar.Ref("project.update"), Fields: fields[1:], When: ar.Condition("not", ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"archived"`)))}}
	for _, f := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(f.Name, "form."+f.Name))
	}
	for _, f := range fields[1:] {
		form.Edit.Input = append(form.Edit.Input, ar.Bind(f.Name, "form."+f.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	columns := []ar.FieldV2{ar.TextField("code", "projects:code"), ar.TextField("name", "projects:name"), {Name: "status", LabelKey: "projects:status", ValueLabelPrefix: "projects:states."}, {Name: "priority", LabelKey: "projects:priority", ValueLabelPrefix: "projects:priorities."}, {Name: "ownerName", LabelKey: "projects:owner", Derived: true}, {Name: "taskCount", LabelKey: "projects:tabs.tasks", Derived: true}}
	columns[1].LinkPage = "project.detail"
	list := ar.PageV2{SchemaVersion: "2.0", ID: "project.list", Type: "list", Mode: "metadata", Route: "/projects", TitleKey: "projects:heading", Permission: ReadPermission, EntityRef: "project", Provider: "projects", Layout: ar.LayoutV2{Type: "standard"}, Forms: []ar.FormV2{form}, Table: ar.TableV2{Columns: columns, DefaultSort: "id:desc", Filters: []ar.Filter{{Name: "keyword", LabelKey: "runtime:search", Type: "string"}, {Name: "status", LabelKey: "projects:status", Type: "enum", Enum: projectStates, ValueLabelPrefix: "projects:states."}, {Name: "priority", LabelKey: "projects:priority", Type: "enum", Enum: priorities, ValueLabelPrefix: "projects:priorities."}}, Sorts: []ar.Sort{{Value: "id:desc", LabelKey: "runtime:newest"}, {Value: "name:asc", LabelKey: "projects:name"}}}, Actions: []ar.Action{ar.DialogAction("project.new", "projects:create", "project.form", "create", "toolbar")}}
	detail := ar.PageV2{SchemaVersion: "2.0", ID: "project.detail", Type: "detail", Mode: "hybrid", Route: "/projects/:id", TitleKey: "projects:heading", Permission: ReadPermission, EntityRef: "project", Provider: "project", Fields: columns, Layout: ar.LayoutV2{Type: "grid", Columns: 2, Sections: []ar.Section{{ID: "related", LabelKey: "projects:heading", Slot: "project.related"}}}, Forms: []ar.FormV2{form}, Actions: []ar.Action{ar.DialogAction("project.edit", "projects:edit", "project.form", "edit", "detail")}}
	for _, next := range []string{"active", "paused", "completed", "archived"} {
		a := ar.ActionCall("project.status."+next, "projects:statusActions."+next, "project.status.change", ar.Bind("id", "record.id"), ar.Bind("from", "record.status"), ar.Const("status", `"`+next+`"`))
		from := []string{}
		for _, tr := range projectEntity().Transitions {
			if tr.To == next && tr.Capability == "project.status.change" {
				from = append(from, tr.From...)
			}
		}
		raw := `["` + join(from, `","`) + `"]`
		a.When = ar.Condition("in", ar.Path("record.status"), ar.Literal(raw))
		a.Tags = []string{"projects"}
		if next == "archived" {
			a.ConfirmKey = "runtime:confirm"
			a.When = ar.Condition("and", a.When, ar.Condition("eq", ar.Path("record.ownerId"), ar.Path("context.accountId")))
		}
		detail.Actions = append(detail.Actions, a)
	}
	taskFields := []ar.FieldV2{ar.TextField("title", "projects:tasks.title"), {Name: "description", LabelKey: "projects:tasks.descriptionField", Component: "textarea", Default: ar.Literal(`""`)}, {Name: "priority", LabelKey: "projects:priority", ValueLabelPrefix: "projects:priorities.", Default: ar.Literal(`"normal"`)}, {Name: "dueDate", LabelKey: "projects:dueDate", Type: "date", Default: ar.Literal(`""`)}, {Name: "planId", LabelKey: "projects:tasks.plan", Type: "reference", Component: "reference", Reference: "plans", Default: ar.Literal(`0`)}, {Name: "assigneeId", LabelKey: "projects:tasks.assignee", Type: "reference", Component: "reference", Reference: "members", SearchParam: "referenceKeyword", Default: ar.Literal(`0`)}}
	tf := ar.FormV2{ID: "task.form", Provider: "task", LabelKey: "projects:tabs.tasks", Tags: []string{"tasks", "projects"}, Create: ar.Command{Capability: ar.Ref("project.task.create"), Fields: taskFields}, Edit: ar.Command{Capability: ar.Ref("project.task.update"), Fields: taskFields}}
	for _, f := range taskFields {
		tf.Create.Input = append(tf.Create.Input, ar.Bind(f.Name, "form."+f.Name))
		tf.Edit.Input = append(tf.Edit.Input, ar.Bind(f.Name, "form."+f.Name))
	}
	tf.Create.Input = append(tf.Create.Input, ar.Bind("projectId", "route.id"))
	tf.Edit.Input = append(tf.Edit.Input, ar.Bind("id", "record.id"))
	assign := ar.FormV2{ID: "task.assign", LabelKey: "projects:tasks.assignee", Tags: []string{"tasks", "projects"}, Create: ar.Command{Capability: ar.Ref("project.task.assign.batch"), ActionType: "batch", Fields: []ar.FieldV2{{Name: "assigneeId", LabelKey: "projects:tasks.assignee", Type: "reference", Component: "reference", Reference: "members", SearchParam: "referenceKeyword", Default: ar.Literal(`0`)}}, Input: []ar.Mapping{ar.Bind("ids", "selection.ids"), ar.Bind("operationId", "context.operationId"), ar.Bind("assigneeId", "form.assigneeId")}}}
	tasks := ar.PageV2{SchemaVersion: "2.0", ID: "project.tasks", Type: "list", Mode: "metadata", TitleKey: "projects:tabs.tasks", Permission: ReadPermission, EntityRef: "project_task", Provider: "tasks", Layout: ar.LayoutV2{Type: "standard"}, Forms: []ar.FormV2{tf, assign}, Table: ar.TableV2{Selectable: true, Columns: []ar.FieldV2{ar.TextField("title", "projects:tasks.title"), {Name: "status", LabelKey: "projects:status", ValueLabelPrefix: "projects:tasks.states."}, {Name: "assigneeName", LabelKey: "projects:tasks.assignee", Derived: true}}, Filters: []ar.Filter{{Name: "keyword", LabelKey: "runtime:search", Type: "string"}}}, Actions: []ar.Action{ar.DialogAction("task.new", "projects:tasks.new", "task.form", "create", "toolbar"), ar.DialogAction("task.edit", "runtime:edit", "task.form", "edit", "row"), ar.DialogAction("task.batch", "runtime:assignSelected", "task.assign", "create", "selection")}}
	for _, next := range []string{"todo", "in_progress", "done", "cancelled"} {
		from := []string{}
		for _, tr := range projectTaskEntity().Transitions {
			if tr.Capability == "project.task.status.change" && tr.To == next {
				from = append(from, tr.From...)
			}
		}
		if len(from) == 0 {
			continue
		}
		a := ar.ActionCall("task.status."+next, "projects:tasks.states."+next, "project.task.status.change", ar.Bind("id", "record.id"), ar.Bind("from", "record.status"), ar.Const("status", `"`+next+`"`))
		a.Placement = "row"
		a.WebMCP = true
		a.When = ar.Condition("in", ar.Path("record.status"), ar.Literal(`["`+join(from, `","`)+`"]`))
		a.Tags = []string{"tasks", "projects"}
		tasks.Actions = append(tasks.Actions, a)
	}
	single := ar.FormV2{ID: "task.assign.single", Provider: "task", LabelKey: "projects:tasks.assign", Tags: []string{"tasks", "projects"}, Edit: ar.Command{Capability: ar.Ref("project.task.assign"), Fields: []ar.FieldV2{{Name: "assigneeId", LabelKey: "projects:tasks.assignee", Type: "reference", Component: "reference", Reference: "members", SearchParam: "referenceKeyword", Default: ar.Literal(`0`)}}, Input: []ar.Mapping{ar.Bind("id", "record.id"), ar.Bind("assigneeId", "form.assigneeId")}}}
	taskDetail := ar.PageV2{SchemaVersion: "2.0", ID: "task.detail", Type: "detail", Mode: "metadata", TitleKey: "projects:tasks.detail", Permission: ReadPermission, EntityRef: "project_task", Provider: "task", Layout: ar.LayoutV2{Type: "section"}, Fields: []ar.FieldV2{ar.TextField("title", "projects:tasks.title"), ar.TextField("description", "projects:tasks.descriptionField"), {Name: "status", LabelKey: "projects:status", ValueLabelPrefix: "projects:tasks.states."}, {Name: "assigneeName", LabelKey: "projects:tasks.assignee", Derived: true}, {Name: "planTitle", LabelKey: "projects:tasks.plan", Derived: true}, {Name: "dueDate", LabelKey: "projects:dueDate", Type: "date"}, {Name: "brief", LabelKey: "projects:tasks.brief", Derived: true}, {Name: "briefModel", LabelKey: "projects:tasks.briefHint", Derived: true}}, Forms: []ar.FormV2{tf, single}, Actions: []ar.Action{ar.DialogAction("task.detail.edit", "runtime:edit", "task.form", "edit", "detail"), ar.DialogAction("task.detail.assign", "projects:tasks.assign", "task.assign.single", "edit", "detail")}}
	tasks.Actions = append(tasks.Actions, ar.DialogAction("task.open", "projects:tasks.detail", "task.detail", "detail", "row"))
	detail.RelationBindings = []ar.RelatedBinding{{EntityRef: "project_task", ProviderID: "tasks", MaxItems: 10}}
	app.Pages = []ar.PageV2{list, detail, tasks, taskDetail}
	app.Extensions = []ar.ExtensionV2{{ID: "project.related", Source: app.Source, Version: "1.0.0", Type: "component", Entry: "project.related", RuntimeCompatibility: "2.0", Permissions: []string{ReadPermission}}}
	for i := range app.Pages {
		for j := range app.Pages[i].Actions {
			if app.Pages[i].Actions[j].Capability.Name == "project.status.change" {
				app.Pages[i].Actions[j].WebMCP = true
			}
		}
		if app.Pages[i].ID == "project.detail" {
			action := ar.ActionCall("project.analyze", "runtime:analyze", "orchestration.workflow.run", ar.Const("flow", `"project-analysis"`), ar.Const("version", `"1.0.0"`), ar.Mapping{Target: "input", Value: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "record.id"), ar.Const("question", `"Analyze the current record and explain its status, using only available evidence."`)}}})
			action.Type = "agent"
			action.When = ar.Condition("eq", ar.Path("context.canUseAssistant"), ar.Literal(`true`))
			action.Tags = []string{"projects"}
			app.Pages[i].Actions = append(app.Pages[i].Actions, action)
		}
	}
	for i := range app.Pages {
		safe := func(fields []ar.FieldV2) {
			for j := range fields {
				switch fields[j].Name {
				case "id", "title", "code", "name", "status", "priority", "dueDate", "revision", "reviewCode", "taskCount", "taskDoneCount", "taskOverdueCount":
					fields[j].Semantic = true
				}
			}
		}
		safe(app.Pages[i].Fields)
		safe(app.Pages[i].Table.Columns)
	}
	app.AgentTools = []ar.CapabilityRef{ar.Ref("project.list"), ar.Ref("project.get"), ar.Ref("project.task.list")}
	return ar.Canonical(app)
}
func join(values []string, sep string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}

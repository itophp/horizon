package articles

import ar "baseline/services/api/kernel/applicationruntime"

func RuntimeApplication() ar.ApplicationV2 {
	app, err := ar.Upgrade(Application())
	if err != nil {
		panic(err)
	}
	app.Permission = "articles:read"
	for i := range app.Providers {
		p := &app.Providers[i]
		p.Tags = []string{"articles"}
		p.PollInterval = 5000
		if p.Pagination == "server" {
			p.Input = append(p.Input, ar.Bind("keyword", "query.keyword"), ar.Bind("sort", "query.sort"), ar.Bind("status", "query.status"))
			p.PollInterval = 5000
		}
	}
	for i := range app.Pages {
		p := &app.Pages[i]
		p.Permission = "articles:read"
		for j := range p.Table.Columns {
			if p.Table.Columns[j].LinkPage != "" {
				p.Table.Columns[j].LinkPage = "article.detail"
			}
		}
		if p.Type == "list" {
			p.Table.Filters = []ar.Filter{{Name: "keyword", Type: "string", LabelKey: "runtime:search"}, {Name: "status", Type: "enum", LabelKey: "articles:status", Enum: articleEntity().StateNames(), ValueLabelPrefix: "articles:states."}}
			p.Table.Sorts = []ar.Sort{{Value: "id:desc", LabelKey: "runtime:newest"}, {Value: "id:asc", LabelKey: "runtime:oldest"}, {Value: "title:asc", LabelKey: "articles:title"}}
			p.Table.DefaultSort = "id:desc"
			p.Actions = []ar.Action{ar.DialogAction("article.new", "articles:new", "article.form", "create", "toolbar")}
		} else {
			p.Actions = []ar.Action{ar.DialogAction("article.edit", "articles:article.edit", "article.form", "edit", "detail")}
			publish := ar.ActionCall("article.submit", "articles:submit", "orchestration.workflow.run", ar.Const("flow", `"article-publish"`), ar.Const("version", `"1.0.0"`), ar.Mapping{Target: "input", Value: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "record.id"), ar.Bind("revision", "record.revision")}}})
			publish.Type = "workflow"
			publish.When = ar.Condition("in", ar.Path("record.status"), ar.Literal(`["draft","pending_ai"]`))
			publish.Tags = []string{"articles"}
			p.Actions = append(p.Actions, publish)
			for _, approve := range []bool{true, false} {
				id, label, value := "article.approve", "articles:approve", "true"
				if !approve {
					id, label, value = "article.reject-human", "articles:reject", "false"
				}
				a := ar.ActionCall(id, label, "article.decide", ar.Bind("id", "record.id"), ar.Bind("revision", "record.revision"), ar.Const("approve", value))
				a.When = ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"pending_review"`))
				a.Tags = []string{"articles"}
				p.Actions = append(p.Actions, a)
			}
		}
	}
	for i := range app.Pages {
		if app.Pages[i].ID == "article.detail" {
			action := ar.ActionCall("article.analyze", "runtime:analyze", "orchestration.workflow.run", ar.Const("flow", `"article-analysis"`), ar.Const("version", `"1.0.0"`), ar.Mapping{Target: "input", Value: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "record.id"), ar.Const("question", `"Analyze the current record and explain its status, using only available evidence."`)}}})
			action.Type = "agent"
			action.When = ar.Condition("eq", ar.Path("context.canUseAssistant"), ar.Literal(`true`))
			action.Tags = []string{"articles"}
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
	app.AgentTools = []ar.CapabilityRef{ar.Ref("article.list")}
	return ar.Canonical(app)
}

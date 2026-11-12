package articles

import ar "horizon/foundation/kernel/applicationruntime"

func articleRuntimeForm() ar.FormV2 {
	fields := []ar.FieldV2{
		{Name: "title", LabelKey: "articles:title", Type: "string", Component: "string", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 200},
		{Name: "body", LabelKey: "articles:body", Type: "string", Component: "markdown", ComponentVersion: "2.0", Required: true, MinLength: 1, MaxLength: 20000},
	}
	form := ar.FormV2{
		ID: "article.form", LabelKey: "articles:heading", Tags: []string{"articles"},
		Create: ar.Command{ActionType: "capability", Capability: ar.Ref("article.create"), Fields: fields},
		Edit:   ar.Command{ActionType: "capability", Capability: ar.Ref("article.update"), Fields: fields},
	}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	form.Edit.Input = append(form.Edit.Input, ar.Bind("revision", "record.revision"))
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

func RuntimeApplication() ar.ApplicationV2 {
	list := ar.Provider{
		ID: "article.list", Capability: ar.Ref("article.list"), KeyField: "id",
		Pagination: "server", PageSize: 20, Tags: []string{"articles"},
		Input: []ar.Mapping{
			ar.Bind("page", "query.page"), ar.Bind("pageSize", "query.pageSize"),
			ar.Bind("keyword", "query.keyword"), ar.Bind("sort", "query.sort"),
			ar.Bind("status", "query.status"),
		},
	}
	detail := ar.Provider{
		ID: "article.detail", Capability: ar.Ref("article.get"), KeyField: "id",
		Tags: []string{"articles"}, Input: []ar.Mapping{ar.Bind("id", "route.id")},
	}
	form := articleRuntimeForm()
	listColumns := []map[string]any{
		{"name": "title", "labelKey": "articles:title"},
		{"name": "status", "labelKey": "articles:status", "display": "enumBadge", "valueLabelPrefix": "articles:states."},
		{"name": "revision", "labelKey": "articles:article.revision"},
		{"name": "reviewCode", "labelKey": "articles:reason", "display": "enumBadge", "valueLabelPrefix": "articles:codes.", "emptyKey": "articles:article.noReview"},
	}
	listPanel := map[string]any{
		"rowsPath": "items", "keyField": "id", "totalPath": "total", "serverPage": true,
		"serverSearch": true, "searchParam": "keyword", "searchFields": []string{"title", "reviewCode"},
		"tabs": map[string]any{"param": "status", "field": "status", "options": []map[string]any{
			{"value": "all", "labelKey": "articles:runtime.all"},
			{"value": "draft", "labelKey": "articles:states.draft"},
			{"value": "pending_ai", "labelKey": "articles:states.pending_ai"},
			{"value": "pending_review", "labelKey": "articles:states.pending_review"},
			{"value": "published", "labelKey": "articles:states.published"},
			{"value": "rejected", "labelKey": "articles:states.rejected"},
		}},
		"columns": listColumns, "totalKey": "articles:article.total",
		"createKey": "articles:new", "createPermission": "articles:write", "createIcon": "plus",
		"forms": map[string]any{"create": form, "edit": form, "view": form},
		"dialogTitles": map[string]any{
			"create": map[string]any{"titleKey": "articles:new", "descriptionKey": "articles:article.dialogHint"},
			"edit":   map[string]any{"titleKey": "articles:article.edit", "descriptionKey": "articles:article.dialogHint", "wide": true},
			"view":   map[string]any{"titleKey": "articles:article.details", "descriptionKey": "articles:description", "wide": true},
		},
		"rowActions": []map[string]any{
			{"labelKey": "articles:article.details", "icon": "eye", "mode": "event", "event": "open"},
			{"labelKey": "articles:article.edit", "icon": "pencil", "mode": "edit", "permission": "articles:write"},
		},
	}
	detailFields := []map[string]any{
		{"name": "status", "labelKey": "articles:status", "display": "enumBadge", "valueLabelPrefix": "articles:states."},
		{"name": "revision", "labelKey": "articles:article.revision"},
		{"name": "reviewCode", "labelKey": "articles:reason", "display": "enumBadge", "valueLabelPrefix": "articles:codes.", "emptyKey": "articles:article.noReview"},
	}
	detailPanel := map[string]any{
		"titleField": "title", "statusField": "status", "statusValuePrefix": "articles:states.", "bodyField": "body", "bodyTitleKey": "articles:body",
		"sections": []map[string]any{{"titleKey": "articles:article.details", "fields": detailFields}},
		"actions": []map[string]any{
			{"labelKey": "articles:article.edit", "icon": "pencil", "permission": "articles:write", "event": "edit", "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["draft","rejected"]`))},
			{"labelKey": "articles:submit", "icon": "play", "permission": "orchestration:run", "event": "submit", "when": ar.Condition("in", ar.Path("record.status"), ar.Literal(`["draft","pending_ai"]`))},
			{"labelKey": "articles:approve", "icon": "check", "permission": "articles:review", "event": "approve", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"pending_review"`))},
			{"labelKey": "articles:reject", "icon": "reject", "permission": "articles:review", "event": "reject", "when": ar.Condition("eq", ar.Path("record.status"), ar.Literal(`"pending_review"`))},
		},
	}
	page := func(id, route, titleKey, subtitleKey, permission string, data []ar.DataSpec, children []ar.Node) ar.PageV3 {
		return ar.PageV3{SchemaVersion: "3.0", ID: id, Route: route, TitleKey: titleKey, SubtitleKey: subtitleKey, Permission: permission, Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: id + "-root", Kind: "stack", Props: ar.LayoutProps{Gap: "20px", Width: "full"}, Data: data, Children: children}}}
	}
	heading := func(id, titleKey, subtitleKey string, actions []map[string]any) ar.Node {
		props := map[string]any{"titleKey": titleKey, "subtitleKey": subtitleKey}
		if actions != nil {
			props["actions"] = actions
		}
		return componentLayout(id+"-heading", &ar.ComponentSpec{ID: id + "-heading", Kind: "page-heading", Version: "1.0", Props: props})
	}
	listComponent := componentLayout("article-list-panel", &ar.ComponentSpec{
		ID: "article-list", Kind: "table-panel", Version: "1.0", Data: "$articles", Props: listPanel,
		Events: []ar.EventBinding{{Event: "open", Action: "navigate", Target: "article.detail", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}}},
	})
	detailComponent := componentLayout("article-detail-panel", &ar.ComponentSpec{
		ID: "article-detail", Kind: "detail-panel", Version: "1.0", Data: "$article", Props: detailPanel,
		Events: []ar.EventBinding{
			{Event: "edit", Action: "navigate", Target: "article.edit-page", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id")}}},
			{Event: "submit", Action: "invoke", Target: "orchestration.workflow.run@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{
				ar.Const("flow", `"article-publish"`), ar.Const("version", `"1.0.0"`),
				{Target: "input", Value: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("revision", "event.revision")}}},
			}}},
			{Event: "approve", Action: "invoke", Target: "article.decide@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("revision", "event.revision"), ar.Const("approve", "true")}}},
			{Event: "reject", Action: "invoke", Target: "article.decide@1.0.0", Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("id", "event.id"), ar.Bind("revision", "event.revision"), ar.Const("approve", "false")}}},
		},
	})
	createForm := componentLayout("article-create-form", &ar.ComponentSpec{
		ID: "article-create", Kind: "record-form-panel", Version: "1.0",
		Props: map[string]any{
			"titleKey": "articles:new", "descriptionKey": "articles:article.markdownHint", "form": form, "mode": "create", "successTarget": "/articles", "cancelTarget": "/articles",
		},
	})
	editForm := componentLayout("article-edit-form", &ar.ComponentSpec{
		ID: "article-edit", Kind: "record-form-panel", Version: "1.0", Data: "$article",
		Props: map[string]any{
			"titleKey": "articles:article.edit", "descriptionKey": "articles:article.markdownHint", "form": form, "mode": "edit", "successTarget": "/articles/:id", "cancelTarget": "/articles/:id",
		},
	})
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "articles", Version: "2.0.0", Source: "builtin:articles",
		RuntimeCompatibility: "3.0", LabelKey: "articles:heading", Permission: "articles:read", Route: "/articles",
		Pages: []ar.PageV3{
			page("article.list", "/articles", "articles:heading", "articles:description", "articles:read", []ar.DataSpec{{ID: "articles", Provider: list.ID}}, []ar.Node{
				heading("article-list", "articles:heading", "articles:description", []map[string]any{{"labelKey": "articles:new", "icon": "plus", "permission": "articles:write", "dialog": "create"}}),
				listComponent,
			}),
			page("article.detail", "/articles/:id", "articles:heading", "articles:description", "articles:read", []ar.DataSpec{{ID: "article", Provider: detail.ID}}, []ar.Node{detailComponent}),
			page("article.create-page", "/articles/new", "articles:new", "articles:description", "articles:write", nil, []ar.Node{heading("article-create", "articles:new", "articles:description", nil), createForm}),
			page("article.edit-page", "/articles/:id/edit", "articles:article.edit", "articles:description", "articles:write", []ar.DataSpec{{ID: "article", Provider: detail.ID}}, []ar.Node{heading("article-edit", "articles:article.edit", "articles:description", nil), editForm}),
		},
		Providers:  []ar.Provider{list, detail},
		Navigation: []ar.Navigation{{PageID: "article.list", LabelKey: "articles:heading", Icon: "file"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: "2.0.0", MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}

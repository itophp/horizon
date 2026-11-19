package applicationruntime

import (
	"testing"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
)

// The kernel tests build a neutral widget protocol instead of borrowing a
// generated business application: kernel files and tests must not carry
// business names, and the projection and AI behavior under test is protocol
// structure, not business content.

const (
	widgetRead  = "applications:read"
	widgetWrite = "applications:manage"
)

func widgetForm() ar.FormV2 {
	inputSchema := `{"type":"object","properties":{"id":{"type":"integer","minimum":1},"name":{"type":"string","minLength":1,"maxLength":120},"schedule":{"type":"string","enum":["once","daily"]}},"required":["name"],"additionalProperties":false}`
	form := ar.FormV2{
		ID: "widget.form", LabelKey: "runtime:widget.form", Provider: "widget-list",
		Create: ar.Command{
			ActionType: "capability", Capability: ar.Ref("widget.task.create"), Permission: widgetWrite,
			InputSchema: inputSchema,
			Fields: []ar.FieldV2{
				{Name: "name", LabelKey: "runtime:widget.name", Type: "string", Required: true, MinLength: 1, MaxLength: 120},
				{Name: "schedule", LabelKey: "runtime:widget.schedule", Type: "enum", Required: true, Enum: []string{"once", "daily"}},
				{Name: "brief", LabelKey: "runtime:widget.brief", Type: "string", MaxLength: 2000},
			},
			Input: []ar.Mapping{ar.Bind("name", "form.name"), ar.Bind("schedule", "form.schedule")},
		},
		Edit: ar.Command{
			ActionType: "capability", Capability: ar.Ref("widget.task.update"), Permission: widgetWrite,
			InputSchema: inputSchema,
			Fields: []ar.FieldV2{
				{Name: "name", LabelKey: "runtime:widget.name", Type: "string", Required: true, MinLength: 1, MaxLength: 120},
				{Name: "schedule", LabelKey: "runtime:widget.schedule", Type: "enum", Required: true, Enum: []string{"once", "daily"}},
			},
			Input: []ar.Mapping{ar.Bind("name", "form.name"), ar.Bind("schedule", "form.schedule"), ar.Bind("id", "record.id")},
		},
	}
	form.AIAssistant = &ar.AIFormAssistant{
		LabelKey: "runtime:widget.ai", Action: "suggest-summary",
		InputFields: []string{"brief"}, TargetFields: []string{"name", "schedule"},
		CreateScope: "$_aiTask", EditScope: "$_aiTask",
		Prompt: "按 suggestions 补全自动化任务表单，每个字段一条建议。",
	}
	return form
}

func widgetApplication() ar.ApplicationV2 {
	table := ar.Node{Type: "component", Component: &ar.ComponentSpec{
		ID: "widget-task-list", Kind: "table-panel", Version: "1.0", Data: "$tasks",
		Props: map[string]any{
			"forms": map[string]ar.FormV2{"create": widgetForm(), "edit": widgetForm()},
		},
		Events: []ar.EventBinding{
			{Event: "open", Action: "navigate", Target: "widget.detail", Payload: ar.Path("event.id")},
			{Event: "suggest-summary", Action: "invoke", Target: ar.SuggestCapability + "@1.0.0"},
			{Event: "remove", Action: "invoke", Target: "widget.update@1.0.0"},
		},
	}}
	list := ar.PageV3{
		SchemaVersion: "3.0", ID: "widget.list", Route: "/widgets", TitleKey: "runtime:widget.list",
		Permission: widgetRead,
		Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "widget-list-root", Kind: "stack",
			Data:     []ar.DataSpec{{ID: "tasks", Provider: "widget-list"}, {ID: "_aiTask", Provider: "widget-get"}},
			Children: []ar.Node{table},
		}},
	}
	detail := ar.PageV3{
		SchemaVersion: "3.0", ID: "widget.detail", Route: "/widgets/:id", TitleKey: "runtime:widget.detail",
		Permission: widgetRead,
		Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "widget-detail-root", Kind: "stack",
			Data: []ar.DataSpec{{ID: "widget", Provider: "widget-get"}, {ID: "widgetTasks", Provider: "widget-list"}},
			Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{
				ID: "widget-ai-analysis", Kind: "ai-analysis-panel", Version: "1.0", Data: "$widget",
				Props: map[string]any{
					"scenarios": []ar.AIScenario{{
						ID: "widget-blockers", LabelKey: "runtime:widget.blockers",
						Analysis: ar.AIAnalysisSpec{
							Action: "analyze-blockers", Prompt: "找出最多三项阻塞项",
							Input: []ar.AIDataInput{
								{Source: "$widget", Fields: []string{"status"}},
								{Source: "$widgetTasks", Fields: []string{"title", "status"}},
							},
						},
					}},
				},
				Events: []ar.EventBinding{
					{Event: "analyze-blockers", Action: "invoke", Target: ar.AnalyzeCapability + "@1.0.0"},
					{Event: "edit", Action: "invoke", Target: "widget.update@1.0.0"},
				},
			}}},
		}},
	}
	admin := ar.PageV3{
		SchemaVersion: "3.0", ID: "widget.admin", Route: "/widgets/admin", TitleKey: "runtime:widget.admin",
		Permission: widgetWrite,
		Root:       ar.Node{Type: "layout", Layout: &ar.LayoutSpec{ID: "widget-admin-root", Kind: "stack", Children: []ar.Node{}}},
	}
	imp := ar.PageV3{
		SchemaVersion: "3.0", ID: "widget.import", Route: "/widgets/import", TitleKey: "runtime:widget.import",
		Permission: widgetWrite,
		Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "widget-import-root", Kind: "stack",
			Data: []ar.DataSpec{{ID: "widget", Provider: "widget-get"}},
			Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{
				ID: "widget-batch-import", Kind: "batch-import", Version: "1.0", Data: "$widget",
				Props:  map[string]any{"templatePath": "/templates/widgets-import.xlsx"},
				Events: []ar.EventBinding{{Event: "import", Action: "invoke", Target: "widget.update@1.0.0"}},
			}}},
		}},
	}
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "widgets", Version: "1.0.0", Source: "system.identity",
		RuntimeCompatibility: "3.0",
		Permission:           widgetRead, LabelKey: "runtime:widget.app", Route: "/widgets",
		Providers: []ar.Provider{
			{ID: "widget-list", Adapter: "gateway", Permission: widgetRead,
				Capability: ar.Ref("widget.task.list"), KeyField: "id", PageSize: 20,
				InputSchema: `{"type":"object","properties":{"projectId":{"type":"integer","minimum":1},"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}},"required":["projectId"]}`,
				Input:       []ar.Mapping{ar.Bind("projectId", "route.id")}},
			{ID: "widget-get", Adapter: "gateway", Permission: widgetRead,
				Capability: ar.Ref("widget.get"), KeyField: "id",
				InputSchema: `{"type":"object","properties":{"id":{"type":"integer","minimum":1}},"required":["id"]}`,
				Input:       []ar.Mapping{ar.Bind("id", "route.id")}},
		},
		Pages: []ar.PageV3{list, detail, admin, imp},
	}
	return ar.ApplicationV2{
		SchemaVersion: "3.0", ID: "widgets", Version: "1.0.0", MetadataVersion: "1",
		Source: "system.identity", PluginVersion: "1.0.0",
		Permission: widgetRead, LabelKey: "runtime:widget.app", Route: "/widgets",
		Protocol:  protocol,
		Providers: protocol.Providers,
	}
}

func widgetDefinitions() map[string]contracts.Definition {
	return map[string]contracts.Definition{
		"widget.update@1.0.0": {Name: "widget.update", Version: "1.0.0", Permission: widgetWrite},
	}
}

func widgetAIBindings(t *testing.T) (*ar.ComponentSpec, map[string]ar.DataSpec) {
	t.Helper()
	app := widgetApplication()
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[0].Root, "widget-task-list")
	if component == nil {
		t.Fatal("widget AI component missing")
	}
	return component, bindings
}

func widgetAnalysisComponent(t *testing.T) (*ar.ComponentSpec, map[string]ar.DataSpec) {
	t.Helper()
	app := widgetApplication()
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[1].Root, "widget-ai-analysis")
	if component == nil {
		t.Fatal("widget analysis component missing")
	}
	return component, bindings
}

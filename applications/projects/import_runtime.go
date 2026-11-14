package projects

import (
	"horizon/foundation/ai/workbook"
	ar "horizon/foundation/kernel/applicationruntime"
)

func projectImportNode(fields []ar.FieldV2) ar.Node {
	return ar.Node{Type: "component", Component: &ar.ComponentSpec{
		ID: "project-batch-import", Kind: "batch-import", Version: "1.0",
		Props: map[string]any{
			"fields": fields, "uniqueField": "code",
			"templatePath": "/templates/projects-import.xlsx",
			"maxRows":      maxImportRows, "maxFileBytes": workbook.MaxFile,
			"commitHintKey": "projects:import.atomic", "conflictKey": "projects:import.conflict",
		},
		Events: []ar.EventBinding{
			{Event: "parse", Action: "invoke", Target: "document.excel.parse@1.0.0",
				Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("content", "event.content"), ar.Bind("sheet", "event.sheet")}}},
			{Event: "extract", Action: "invoke", Target: "project.import.extract@1.0.0",
				Payload: ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("content", "event.content"), ar.Bind("sheet", "event.sheet")}}},
			{Event: "submit", Action: "invoke", Target: "project.import.commit@1.0.0",
				Payload:        ar.Expression{Kind: "object", Fields: []ar.Mapping{ar.Bind("rows", "event.rows")}},
				IdempotencyKey: importKey()},
			{Event: "success", Action: "navigate", Target: "project.list"},
			{Event: "cancel", Action: "navigate", Target: "project.list"},
		},
	}}
}

func importKey() *ar.Expression {
	key := ar.Path("event.operationKey")
	return &key
}

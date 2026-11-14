package harness

import (
	"context"
	"encoding/base64"

	"horizon/foundation/ai/workbook"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
)

func ExcelParseEntry() registry.Entry {
	cell := contracts.Object(map[string]*contracts.Schema{
		"address": contracts.Str(2, 12), "value": contracts.Str(0, workbook.MaxCell),
		"type": contracts.Enum("text", "number", "boolean", "date", "formula", "error"),
	}, "address", "value", "type")
	return registry.Entry{
		Definition: contracts.Definition{
			Name: workbook.ParseCapability, Version: "1.0.0", Title: "Parse Excel workbook",
			Description: "Extract values and coordinates from one sheet of an uploaded workbook.",
			Domain:      "documents", Layer: contracts.LayerAtomic, ReadOnly: true, TransientResult: true,
			Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Executor: "go:" + workbook.ParseCapability,
			AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI, contracts.OriginCapability},
			Tags:           []string{"documents", "import"},
			Input: contracts.Object(map[string]*contracts.Schema{
				"content": contracts.Str(1, base64.StdEncoding.EncodedLen(workbook.MaxFile)),
				"sheet":   contracts.Str(1, 31),
			}, "content"),
			Output: contracts.Object(map[string]*contracts.Schema{
				"sheets": {Type: "array", Items: contracts.Str(1, 31)},
				"sheet":  contracts.Str(1, 31),
				"rows": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
					"number": contracts.Int(1, workbook.MaxRows),
					"cells":  {Type: "array", Items: cell},
				}, "number", "cells")},
			}, "sheets", "sheet", "rows"),
		},
		Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
			content, err := base64.StdEncoding.Strict().DecodeString(in["content"].(string))
			if err != nil {
				return contracts.Output{}, contracts.Problem("excel_invalid")
			}
			sheet, _ := in["sheet"].(string)
			result, err := workbook.Parse(ctx, content, sheet)
			if err != nil {
				return contracts.Output{}, err
			}
			return contracts.Output{Data: result, Summary: "Excel worksheet parsed"}, nil
		}},
	}
}

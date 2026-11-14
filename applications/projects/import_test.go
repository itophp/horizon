package projects

import (
	"context"
	"strings"
	"testing"

	"horizon/foundation/ai/workbook"
	"horizon/foundation/kernel/capability/contracts"
)

func TestProjectExtractionUsesSelectedRowsAndValidatesModelResult(t *testing.T) {
	selected := workbook.Result{
		Sheets: []string{"Secret sheet name", "Selected"}, Sheet: "Selected",
		Rows: []workbook.Row{{Number: 3, Cells: []workbook.Cell{{Address: "B3", Value: "Project W-1", Type: "text"}}}},
	}
	for _, test := range []struct {
		name, reply, code string
	}{
		{"unknown fields can be omitted", `{"rows":[{"sourceRow":3,"values":{"code":"W-1"}}]}`, ""},
		{"fenced reply with blank fields", "```json\n" + `{"rows":[{"sourceRow":3,"values":{"code":"00123","name":"","dueDate":null}}]}` + "\n```", ""},
		{"no projects found", `{"rows":[]}`, "excel_empty"},
		{"invalid source", `{"rows":[{"sourceRow":2,"values":{"code":"W-1"}}]}`, "excel_ai_invalid"},
		{"invented field", `{"rows":[{"sourceRow":3,"values":{"ownerId":10}}]}`, "excel_ai_invalid"},
		{"malformed reply", `not JSON`, "excel_ai_invalid"},
		{"provider failure", "", "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			result, err := extractProjectDraft(context.Background(), func(_ context.Context, prompt, data string) (string, error) {
				calls++
				if !strings.Contains(data, "B3") || !strings.Contains(data, "Project W-1") || strings.Contains(data, "Secret sheet") || !strings.Contains(prompt, "sourceRow") {
					t.Fatalf("unexpected model input: %s", data)
				}
				if test.reply == "" {
					return "", contracts.Problem("unavailable")
				}
				return test.reply, nil
			}, selected)
			wantCalls := 1
			if test.code == "excel_ai_invalid" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatalf("expected %d model calls, got %d", wantCalls, calls)
			}
			if test.name == "fenced reply with blank fields" && err == nil {
				values := result.Data.(map[string]any)["rows"].([]any)[0].(map[string]any)["values"].(map[string]any)
				if len(values) != 1 || values["code"] != "00123" {
					t.Fatalf("unexpected draft values: %v", values)
				}
			}
			if test.code == "" && (err != nil || result.Data == nil) || test.code != "" && contracts.Code(err) != test.code {
				t.Fatalf("result: %+v, %v", result, err)
			}
		})
	}
}

func TestProjectExtractionRecoversFromInvalidModelReply(t *testing.T) {
	calls := 0
	sheet := workbook.Result{Rows: []workbook.Row{{Number: 3}}}
	result, err := extractProjectDraft(context.Background(), func(_ context.Context, prompt, _ string) (string, error) {
		calls++
		if calls == 1 {
			return "I found a project.", nil
		}
		if !strings.Contains(prompt, "previous response failed validation") {
			t.Fatal("retry did not instruct the model to correct the response")
		}
		return `{"rows":[{"sourceRow":3,"values":{"code":"W-1"}}]}`, nil
	}, sheet)
	if err != nil || result.Data == nil || calls != 2 {
		t.Fatalf("expected valid draft after two calls: %+v, %v, calls=%d", result, err, calls)
	}
}

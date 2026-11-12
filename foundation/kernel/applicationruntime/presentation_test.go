package applicationruntime

import "testing"

func TestProtocolLocalMessagesRequireCompleteResources(t *testing.T) {
	app := ApplicationV2{
		ID: "app_test", LabelKey: "$app:title",
		Messages: []LocaleMessage{{Key: "$app:title", En: "Title", ZhCN: "标题"}},
		Protocol: &ApplicationV3{
			SchemaVersion: "3.0", RuntimeCompatibility: "3.0",
			Pages: []PageV3{{SchemaVersion: "3.0", TitleKey: "$app:title"}},
		},
	}
	bound, err := BindApplicationMessages(app)
	if err != nil || validatePresentation(bound) != nil {
		t.Fatal(bound, err)
	}
	bound.Protocol.Pages[0].TitleKey = "app_test_local:missing"
	if validatePresentation(bound) == nil {
		t.Fatal("undefined local label accepted")
	}
	bound.Protocol.Pages[0].TitleKey = "app_test_local:title"
	bound.Protocol.Pages[0].Root = Node{
		Type: "layout",
		Layout: &LayoutSpec{
			ID: "root", Kind: "grid",
			Children: []Node{{
				Type: "component",
				Component: &ComponentSpec{
					ID: "table", Kind: "table-panel", Version: "1.0",
					Props: map[string]any{
						"columns": []map[string]any{{
							"name": "status", "valueLabelPrefix": "app_test_local:state.",
							"enum": []string{"open", "done"},
						}},
					},
				},
			}},
		},
	}
	bound.Messages = append(bound.Messages, LocaleMessage{Key: "app_test_local:state.open", En: "Open", ZhCN: "开放"})
	if validatePresentation(bound) == nil {
		t.Fatal("incomplete enum messages accepted")
	}
	bound.Messages = append(bound.Messages, LocaleMessage{Key: "app_test_local:state.done", En: "Done", ZhCN: "完成"})
	if err := validatePresentation(bound); err != nil {
		t.Fatal(err)
	}
}

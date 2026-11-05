package applicationruntime

import "testing"

func TestLocalMessagesRequireCompleteResources(t *testing.T) {
	app := ApplicationV2{ID: "app_test", LabelKey: "$app:title", Messages: []LocaleMessage{{Key: "$app:title", En: "Title", ZhCN: "标题"}}}
	bound, err := BindApplicationMessages(app)
	if err != nil || validatePresentation(bound) != nil {
		t.Fatal(bound, err)
	}
	bound.Pages = []PageV2{{TitleKey: "app_test_local:missing"}}
	if validatePresentation(bound) == nil {
		t.Fatal("undefined local label accepted")
	}
	bound.Pages = []PageV2{{Fields: []FieldV2{{ValueLabelPrefix: "app_test_local:state.", Enum: []string{"open", "done"}}}}}
	bound.Messages = append(bound.Messages, LocaleMessage{Key: "app_test_local:state.open", En: "Open", ZhCN: "开放"})
	if validatePresentation(bound) == nil {
		t.Fatal("incomplete enum messages accepted")
	}
	bound.Messages = append(bound.Messages, LocaleMessage{Key: "app_test_local:state.done", En: "Done", ZhCN: "完成"})
	if err := validatePresentation(bound); err != nil {
		t.Fatal(err)
	}
}

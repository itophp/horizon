package applicationruntime

import "testing"

func TestDefinitionImpactsIgnoreVersionAndIdentifyActions(t *testing.T) {
	base := ApplicationV2{ID: "test", ApplicationVersion: "1.0.0.0", Pages: []PageV2{{ID: "detail", Actions: []Action{{ID: "save", LabelKey: "save"}}}}}
	next := Canonical(base)
	next.ApplicationVersion = "1.0.0.1"
	if changes := DefinitionImpacts(base, next); len(changes) != 0 {
		t.Fatal(changes)
	}
	next.Pages[0].Actions[0].LabelKey = "update"
	changes := DefinitionImpacts(base, next)
	if len(changes) != 2 || changes[0].Section != "actions" || changes[0].ID != "detail/save" || changes[0].Change != "updated" || changes[1].Section != "pages" {
		t.Fatal(changes)
	}
}

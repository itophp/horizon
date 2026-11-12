package applicationruntime

import "testing"

func TestDefinitionImpactsIgnoreRuntimeVersionAndUseProtocolIDs(t *testing.T) {
	base := ApplicationV2{
		ID: "test", ApplicationVersion: "1.0.0.0",
		Protocol: &ApplicationV3{Pages: []PageV3{{ID: "detail", TitleKey: "detail"}}},
	}
	next := Canonical(base)
	next.ApplicationVersion = "1.0.0.1"
	if changes := DefinitionImpacts(base, next); len(changes) != 0 {
		t.Fatal(changes)
	}
	next.Protocol.Pages[0].TitleKey = "update"
	changes := DefinitionImpacts(base, next)
	if len(changes) != 1 || changes[0].Section != "pages" || changes[0].ID != "detail" || changes[0].Change != "updated" {
		t.Fatal(changes)
	}
}

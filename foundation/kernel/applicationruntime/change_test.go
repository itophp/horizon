package applicationruntime

import "testing"

func TestApplyDefinitionChangesTargetsProtocolDeclarations(t *testing.T) {
	app := ApplicationV2{
		ID: "app", Source: "code", Protocol: &ApplicationV3{
			Pages:      []PageV3{{ID: "list", TitleKey: "old"}},
			Providers:  []Provider{{ID: "items"}},
			Navigation: []Navigation{{PageID: "list"}, {PageID: "detail"}},
		},
	}
	changed, err := ApplyDefinitionChanges(app, []DefinitionChange{{
		Section: "pages", ID: "list", Operation: "upsert",
		Value: `{"schemaVersion":"3.0","id":"list","route":"/items","titleKey":"new","permission":"items:read","root":{"type":"layout","layout":{"id":"root","kind":"stack"}}}`,
	}})
	if err != nil || changed.Protocol.Pages[0].TitleKey != "new" {
		t.Fatal(changed, err)
	}
	if _, err = ApplyDefinitionChanges(ApplicationV2{ID: "old"}, []DefinitionChange{{Section: "pages", ID: "x", Operation: "remove"}}); err == nil {
		t.Fatal("legacy envelope accepted")
	}
}

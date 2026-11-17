package applicationruntime

import (
	"encoding/json"
	"os"
	"testing"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
)

func compiledFixture(t *testing.T, id string) ar.ApplicationV2 {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/generated/runtime/applications.json")
	if err != nil {
		t.Fatal(err)
	}
	var apps []ar.ApplicationV2
	if err := json.Unmarshal(raw, &apps); err != nil {
		t.Fatal(err)
	}
	for _, app := range apps {
		if app.ID == id {
			return app
		}
	}
	t.Fatalf("compiled fixture %s not found", id)
	return ar.ApplicationV2{}
}

func collectFixtureEvents(node ar.Node, definitions map[string]contracts.Definition) {
	if node.Layout != nil {
		for _, child := range node.Layout.Children {
			collectFixtureEvents(child, definitions)
		}
	}
	if node.Component == nil {
		return
	}
	for _, binding := range node.Component.Events {
		if binding.Action == "invoke" {
			definitions[binding.Target] = contracts.Definition{Permission: "articles:write"}
		}
	}
}

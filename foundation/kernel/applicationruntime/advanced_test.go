package applicationruntime_test

import (
	"encoding/json"
	ar "horizon/foundation/kernel/applicationruntime"
	"strings"
	"testing"
)

func TestDeploymentRetainsVersionsWithoutReplacingContracts(t *testing.T) {
	sources, entities, defs := fixture()
	old := ar.Canonical(sources[1])
	old.Version = "0.9.0"
	old.PluginVersion = "0.9.0"
	old.Protocol.Version = "0.9.0"
	current, versions, err := ar.AssembleDeployment(append(sources, old), entities, defs)
	if err != nil || len(current) != 2 || len(versions) != 3 {
		t.Fatal(len(current), len(versions), err)
	}
	for _, app := range current {
		if app.ID == old.ID && app.Version == old.Version {
			t.Fatal("old candidate selected as current")
		}
		if app.SchemaVersion != "3.0" || app.Protocol == nil {
			t.Fatal(app.SchemaVersion)
		}
	}
	if _, _, err = ar.AssembleDeployment(append(sources, sources[0]), entities, defs); err == nil {
		t.Fatal("duplicate version accepted")
	}
}
func TestFilterCompilerRejectsUndeclaredAndAmbiguousMappings(t *testing.T) {
	provider := ar.Provider{QueryBindings: []ar.QueryBinding{{Field: "title", Operator: "contains", Target: "keyword"}, {Field: "status", Operator: "eq", Target: "status"}}}
	filter := ar.FilterExpression{Kind: "and", Children: []ar.FilterExpression{{Kind: "condition", Field: "title", Operator: "contains", Value: `"hello"`}, {Kind: "condition", Field: "status", Operator: "eq", Value: `"draft"`}}}
	result, err := ar.CompileFilter(filter, provider)
	if err != nil || result["keyword"] != "hello" || result["status"] != "draft" {
		t.Fatal(result, err)
	}
	invalid := []ar.FilterExpression{{Kind: "condition", Field: "body", Operator: "contains", Value: `"secret"`}, {Kind: "or", Children: filter.Children}, {Kind: "and", Children: append(filter.Children, filter.Children[0])}}
	for _, expr := range invalid {
		if _, err = ar.CompileFilter(expr, provider); err == nil {
			t.Fatal("unsafe filter accepted", expr)
		}
	}
}
func TestMappingInterpreterRejectsMissingPathsAndPrototypeAccess(t *testing.T) {
	for _, path := range []string{"record.missing", "record.__proto__", "record.constructor"} {
		if _, err := ar.Evaluate(ar.Path(path), map[string]any{"record": map[string]any{"id": 1}}); err == nil {
			t.Fatal(path)
		}
	}
	result, err := ar.Map([]ar.Mapping{ar.Bind("id", "$context.record.id"), ar.Const("approved", "false")}, map[string]any{"record": map[string]any{"id": 7}})
	if err != nil || result["id"] != 7 || result["approved"] != false {
		t.Fatal(result, err)
	}
}

func TestDynamicCloneValidation(t *testing.T) {
	sources, entities, defs := fixture()
	compiled, e := ar.Assemble(sources, entities, defs)
	if e != nil {
		t.Fatal(e)
	}
	app := ar.Canonical(compiled[1])
	app.ID = "app_test"
	app.Source = "builtin:runtime-host"
	app.Version = "0.1.0"
	app.Protocol.ID = app.ID
	app.Protocol.Source = app.Source
	app.Protocol.Version = app.Version
	app.ApplicationVersion = "0.1.0.0"
	app.PluginVersion = "1.0.0"
	app.Components = nil
	app.AgentTools = nil
	app.PluginDependencies = nil
	raw, _ := json.Marshal(app)
	raw = []byte(strings.ReplaceAll(string(raw), "/projects", "/apps/operations"))
	if err := json.Unmarshal(raw, &app); err != nil {
		t.Fatal(err)
	}
	if _, err := ar.Assemble(append(sources, app), entities, defs); err != nil {
		t.Fatal(err)
	}
}

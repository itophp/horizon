package applicationruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"horizon/applications/projects"
	"horizon/foundation/ai/harness"
	gen "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

func aiFixture(t *testing.T) (*Module, ar.ApplicationV2) {
	t.Helper()
	m := &Module{Definitions: map[string]contracts.Definition{}}
	entries := append(projects.New(nil).Entries(), m.AIEntries()...)
	entries = append(entries, harness.ExcelParseEntry())
	var definitions []contracts.Definition
	for _, entry := range entries {
		m.Definitions[entry.Definition.Key()] = entry.Definition
		definitions = append(definitions, entry.Definition)
	}
	apps, err := ar.Assemble([]ar.ApplicationV2{projects.RuntimeApplication()}, nil, definitions)
	if err != nil {
		t.Fatal(err)
	}
	app := apps[0]
	app.Providers = app.Protocol.Providers
	return m, app
}

func TestRuntimeAIAnalysisProjectsOnlyConfiguredFields(t *testing.T) {
	m, app := aiFixture(t)
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[1].Root, "project-ai-analysis")
	calls := 0
	m.Invoke = func(_ context.Context, name, version string, in contracts.Input, _ string) (*contracts.Result, error) {
		if version != "1.0.0" {
			t.Fatal(version)
		}
		switch name {
		case "project.get":
			if in["id"] != 7 {
				t.Fatal(in)
			}
			return &contracts.Result{Status: contracts.StatusSucceeded, Data: map[string]any{"status": "active", "ownerName": "private"}}, nil
		case "project.task.list":
			if in["projectId"] != 7 || in["page"] != 1 || in["pageSize"] != 20 {
				t.Fatal(in)
			}
			return &contracts.Result{Status: contracts.StatusSucceeded, Data: map[string]any{
				"items": []map[string]any{{"title": "Prepare release", "status": "todo", "description": "private"}}, "total": 27,
			}}, nil
		}
		t.Fatalf("unexpected provider %s", name)
		return nil, nil
	}
	m.GenerateAI = func(_ context.Context, prompt, data string) (string, error) {
		calls++
		if !strings.Contains(prompt, "找出最多三项") || !strings.Contains(data, "Prepare release") || strings.Contains(data, "private") {
			t.Fatalf("unexpected prompt or projection: %s %s", prompt, data)
		}
		return `{"summary":"Release preparation is outstanding","evidence":["Prepare release is todo"]}`, nil
	}
	in := contracts.Input{"sceneId": "task-blockers", "recordId": 7}
	result, err := m.analyzeAI(t.Context(), app, component, bindings, in)
	if err != nil || calls != 1 {
		t.Fatal(result, err, calls)
	}
	raw, _ := json.Marshal(result.Data)
	if !strings.Contains(string(raw), `"coverage":{"analyzed":1,"total":27}`) {
		t.Fatal(string(raw))
	}
	in["sceneId"] = "forged"
	if _, err := m.analyzeAI(t.Context(), app, component, bindings, in); err == nil || calls != 1 {
		t.Fatal("forged scene reached the model")
	}
	in["sceneId"] = "task-blockers"
	m.Invoke = func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error) {
		return nil, contracts.Problem("not_found")
	}
	if _, err := m.analyzeAI(t.Context(), app, component, bindings, in); err == nil || calls != 1 {
		t.Fatal("unreadable record reached the model")
	}
}

func TestRuntimeAIFormRejectsUnlistedDraftAndModelFields(t *testing.T) {
	m, app := aiFixture(t)
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[2].Root, "project-task-list")
	p, err := identity.NewPrincipal(&gen.AccountRecord{ID: 1, IsAdmin: true}, "test-tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Invoke = func(_ context.Context, name, version string, in contracts.Input, _ string) (*contracts.Result, error) {
		if name != "project.get" || version != "1.0.0" || in["id"] != 7 {
			t.Fatal(name, version, in)
		}
		return &contracts.Result{Status: contracts.StatusSucceeded, Data: map[string]any{"id": 7}}, nil
	}
	calls := 0
	response := `{"suggestions":[{"field":"description","value":"Run the release checklist."}]}`
	m.GenerateAI = func(_ context.Context, _, data string) (string, error) {
		calls++
		if strings.Contains(data, "private") {
			t.Fatal("private draft value sent to model")
		}
		return response, nil
	}
	in := contracts.Input{"sceneId": "task.form", "mode": "create", "recordId": 7, "draft": map[string]any{"title": "Release", "description": "private"}}
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err == nil || calls != 0 {
		t.Fatal("unlisted draft field reached the model")
	}
	in["draft"] = map[string]any{"title": "Release"}
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	response = `{"suggestions":[{"field":"title","value":"Forbidden overwrite"}]}`
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err == nil {
		t.Fatal("unlisted output field accepted")
	}
	response = `{"suggestions":[{"field":"description","value":"A"},{"field":"description","value":"B"}]}`
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err == nil {
		t.Fatal("duplicate suggestions accepted")
	}
}

func TestRuntimeAIRequiresGatewayRuntimeContext(t *testing.T) {
	m, _ := aiFixture(t)
	for _, entry := range m.AIEntries() {
		if entry.Definition.AllowsOrigin(contracts.OriginAgent) || entry.Definition.AllowsOrigin(contracts.OriginAPI) ||
			!entry.Definition.TransientResult || strings.Contains(strings.Join(entry.Definition.Loggable, ","), "draft") {
			t.Fatal(entry.Definition)
		}
		if _, err := entry.Handler.Execute(t.Context(), nil); err == nil {
			t.Fatal("execution without a Gateway Runtime reference was allowed")
		}
	}
}

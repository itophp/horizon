package applicationruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	gen "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

func aiFixture(t *testing.T) (*Module, ar.ApplicationV2) {
	t.Helper()
	app := compiledFixture(t, "projects")
	m := &Module{Definitions: map[string]contracts.Definition{}}
	for _, provider := range app.Protocol.Providers {
		if provider.Capability.Name == "" || provider.InputSchema == "" {
			continue
		}
		var input contracts.Schema
		if err := json.Unmarshal([]byte(provider.InputSchema), &input); err != nil {
			t.Fatal(err)
		}
		d := contracts.Definition{Name: provider.Capability.Name, Version: provider.Capability.Version, ReadOnly: true, Input: &input}
		m.Definitions[d.Key()] = d
	}
	component, _ := ar.FindAIComponent(app.Protocol.Pages[2].Root, "project-task-list")
	for _, form := range ar.AIForms(component) {
		for _, command := range []ar.Command{form.Create, form.Edit} {
			if command.Capability.Name == "" || command.InputSchema == "" {
				continue
			}
			var input contracts.Schema
			if err := json.Unmarshal([]byte(command.InputSchema), &input); err != nil {
				t.Fatal(err)
			}
			d := contracts.Definition{Name: command.Capability.Name, Version: command.Capability.Version, Permission: "projects:write", Input: &input}
			m.Definitions[d.Key()] = d
		}
	}
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
	// Models often add explanation keys; only the consumed fields are validated.
	response = `{"suggestions":[{"field":"description","value":"Run the release checklist.","reason":"clear"}],"note":"extra"}`
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err != nil {
		t.Fatal("extra model keys rejected", err)
	}
	response = `{"suggestions":[{"field":"description","value":"Run the release checklist."}]}`
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

// A schedule form picks its wall-clock component from the schedule kind, so a
// suggestion target may be conditionally visible. The model also receives the
// target types and the device clock, and enum/date/time/datetime values are
// validated before the browser can apply them.
func TestRuntimeAIFormFillsTypedConditionalTargets(t *testing.T) {
	fields := []ar.FieldV2{
		{Name: "brief", Type: "string", Component: "textarea"},
		{Name: "name", Type: "string", Component: "string", Required: true, MinLength: 1, MaxLength: 20},
		{Name: "kind", Type: "enum", Component: "enum", Enum: []string{"daily", "once"}},
		{Name: "minute", Type: "enum", Component: "enum", Enum: []string{"0", "5", "59"}},
		{Name: "time", Type: "time", Component: "time",
			VisibleWhen: ar.Condition("eq", ar.Path("form.kind"), ar.Literal(`"daily"`))},
		{Name: "runAt", Type: "datetime", Component: "datetime",
			VisibleWhen: ar.Condition("eq", ar.Path("form.kind"), ar.Literal(`"once"`))},
	}
	command := ar.Command{
		ActionType: "capability", Capability: ar.Ref("test.schedule.create"), Fields: fields,
		Input: []ar.Mapping{
			ar.Bind("name", "form.name"), ar.Bind("kind", "form.kind"),
			ar.Bind("minute", "form.minute"),
			ar.Bind("time", "form.time"), ar.Bind("runAt", "form.runAt"),
		},
	}
	form := ar.FormV2{
		ID: "schedule.form", Create: command, Edit: command,
		AIAssistant: &ar.AIFormAssistant{
			LabelKey: "test:ai", Action: "suggest", InputFields: []string{"brief"},
			TargetFields: []string{"name", "kind", "minute", "time", "runAt"}, Prompt: "Fill the schedule form.",
		},
	}
	component := &ar.ComponentSpec{
		ID: "schedule-panel", Kind: "table-panel", Version: "1.0", Data: "$item",
		Props:  map[string]any{"forms": map[string]any{"create": form, "edit": form}},
		Events: []ar.EventBinding{{Event: "suggest", Action: "invoke", Target: ar.SuggestCapability + "@1.0.0"}},
	}
	definition := contracts.Definition{
		Name: "test.schedule.create", Version: "1.0.0", Permission: "automation:write",
		AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI},
		Input: contracts.Object(map[string]*contracts.Schema{
			"name": contracts.Str(1, 20), "kind": contracts.Enum("daily", "once"),
			"minute": contracts.Enum("0", "5", "59"),
			"time":   contracts.Str(0, 5), "runAt": contracts.Str(0, 40),
		}, "name", "kind"),
	}
	m := &Module{Definitions: map[string]contracts.Definition{definition.Key(): definition}}
	p, err := identity.NewPrincipal(&gen.AccountRecord{ID: 1, IsAdmin: true}, "test-tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	response := `{"suggestions":[{"field":"name","value":"Daily report"},{"field":"kind","value":"daily"},{"field":"time","value":"09:00"}]}`
	modelData := ""
	m.GenerateAI = func(_ context.Context, _, data string) (string, error) {
		modelData = data
		return response, nil
	}
	in := contracts.Input{
		"sceneId": "schedule.form", "mode": "create", "recordId": 0,
		"now": "2026-11-17T10:00:00+08:00", "draft": map[string]any{"brief": "每天 9 点生成报表"},
	}
	if _, err := m.suggestAI(t.Context(), p, ar.ApplicationV2{}, component, nil, in); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(modelData), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["currentTime"] != "2026-11-17T10:00:00+08:00" {
		t.Fatalf("device clock missing from model input: %v", payload)
	}
	targets, _ := payload["targetFields"].([]any)
	found := map[string]map[string]any{}
	for _, raw := range targets {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("target metadata is not an object: %v", raw)
		}
		found[entry["name"].(string)] = entry
	}
	if found["time"] == nil || found["time"]["type"] != "time" {
		t.Fatalf("time target type missing: %v", found)
	}
	if enum, _ := found["kind"]["enum"].([]any); len(enum) != 2 {
		t.Fatalf("enum target values missing: %v", found["kind"])
	}
	if found["name"]["maxLength"] != float64(20) {
		t.Fatalf("target length bound missing: %v", found["name"])
	}

	for _, invalid := range []string{
		`{"suggestions":[{"field":"time","value":"25:00"}]}`,
		`{"suggestions":[{"field":"runAt","value":"tomorrow"}]}`,
		`{"suggestions":[{"field":"kind","value":"sometimes"}]}`,
		`{"suggestions":[{"field":"name","value":"This name is longer than twenty characters"}]}`,
	} {
		response = invalid
		if _, err := m.suggestAI(t.Context(), p, ar.ApplicationV2{}, component, nil, in); err == nil {
			t.Fatalf("invalid suggestion accepted: %s", invalid)
		}
	}
	// Deterministic representations are normalized instead of failing the
	// whole request: a zero padded select value and a single digit hour.
	response = `{"suggestions":[{"field":"minute","value":"05"},{"field":"time","value":"9:30"},{"field":"runAt","value":"2026-11-18T9:30"}]}`
	out, err := m.suggestAI(t.Context(), p, ar.ApplicationV2{}, component, nil, in)
	if err != nil {
		t.Fatal(err)
	}
	normalized, _ := json.Marshal(out.Data)
	for _, want := range []string{`"value":"5"`, `"value":"09:30"`, `"value":"2026-11-18T09:30"`} {
		if !strings.Contains(string(normalized), want) {
			t.Fatalf("suggestion %s was not normalized: %s", want, normalized)
		}
	}
	// A malformed device clock must not block the advisory suggestion.
	in["now"] = "tomorrow"
	response = `{"suggestions":[{"field":"kind","value":"once"},{"field":"runAt","value":"2026-11-18T09:00"}]}`
	if _, err := m.suggestAI(t.Context(), p, ar.ApplicationV2{}, component, nil, in); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(modelData, "tomorrow") {
		t.Fatal("malformed device clock reached the model")
	}
}

// A create form suggests from the draft before any record exists: no business
// record is read, and an edit form still refuses a missing record.
func TestRuntimeAIFormSuggestsFromDraftWithoutRecord(t *testing.T) {
	m, app := aiFixture(t)
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[2].Root, "project-task-list")
	p, err := identity.NewPrincipal(&gen.AccountRecord{ID: 1, IsAdmin: true}, "test-tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Invoke = func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error) {
		t.Fatal("draft-only suggestion must not read a business record")
		return nil, nil
	}
	calls := 0
	m.GenerateAI = func(_ context.Context, _, data string) (string, error) {
		calls++
		if !strings.Contains(data, "Release checklist") {
			t.Fatal(data)
		}
		return `{"suggestions":[{"field":"description","value":"Run the release checklist."}]}`, nil
	}
	in := contracts.Input{"sceneId": "task.form", "mode": "create", "recordId": 0, "draft": map[string]any{"title": "Release checklist"}}
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	in["mode"] = "edit"
	if _, err := m.suggestAI(t.Context(), p, app, component, bindings, in); err == nil || calls != 1 {
		t.Fatal("edit without a record reached the model")
	}
}

func TestDecodeAIObjectStripsCodeFence(t *testing.T) {
	schema := contracts.Object(map[string]*contracts.Schema{
		"suggestions": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"field": contracts.Str(1, 100), "value": contracts.Str(1, 4000),
		}, "field", "value")},
	}, "suggestions")
	out, err := decodeAIObject("```json\n{\"suggestions\":[{\"field\":\"description\",\"value\":\"brief\"}]}\n```", schema)
	if err != nil || len(out["suggestions"].([]any)) != 1 {
		t.Fatal(out, err)
	}
	if _, err = decodeAIObject("not json", schema); err == nil {
		t.Fatal("prose accepted as structured output")
	}
	if _, err = decodeAIObject(`{"suggestions":[{"field":"description","value":"ok"}],"extra":true}`, schema); err == nil {
		t.Fatal("unknown field accepted")
	}
}

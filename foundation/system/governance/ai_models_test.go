package governance

import (
	"strconv"
	"testing"
)

func TestResolveAIModelUsesBusinessOverrideAndWorkspaceDefault(t *testing.T) {
	core := New(nil, nil, Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	values := map[string]Value{}
	for _, d := range core.Definitions {
		values[d.Key] = Value{Raw: d.Default, Source: "default"}
	}
	set := func(key, raw string) { values[key] = Value{Raw: raw, Source: "workspace:0"} }
	setSecret := func(key, token string) {
		sealed, err := core.Seal("tenant", "workspace:0", key, strconv.Quote(token))
		if err != nil {
			t.Fatal(err)
		}
		set(key, strconv.Quote(sealed))
	}
	set("agent.enabled", "true")
	set("assistant.enabled", "true")
	setSecret("assistant.token", "glm-key")

	model, err := core.resolveAIModel("tenant", values, ModelWorkspace)
	if err != nil || model.Profile != "deepseek" || model.Name != "deepseek-flash" || model.Configured() {
		t.Fatal("missing DeepSeek credentials must not silently use the GLM model", err)
	}

	setSecret("aiModels.deepseek.token", "deepseek-key")
	set("aiModels.route.projectImport", `"glm"`)
	set("aiModels.route.assistant", `"glm"`)
	for _, use := range modelUses {
		name, token, protocol := "deepseek-flash", "deepseek-key", ChatCompletions
		switch use {
		case ModelProjectImport:
			name, token = "glm-5.3-flash", "glm-key"
		case ModelAssistant:
			name, token, protocol = "glm-5.3-flash", "glm-key", Responses
		}
		model, err = core.resolveAIModel("tenant", values, use)
		if err != nil || !model.Enabled || !model.Configured() || model.Name != name || model.Token != token || model.Protocol != protocol {
			t.Fatalf("wrong model for %s: %v", use, err)
		}
	}

	set("aiModels.default", `"glm"`)
	model, err = core.resolveAIModel("tenant", values, ModelWorkspace)
	if err != nil || model.Name != "glm-5.3-flash" || model.Protocol != ChatCompletions || model.BaseURL != "https://open.bigmodel.cn/api/coding/paas/v4" {
		t.Fatal("workspace should follow a change to the default model", err)
	}
	if _, err = core.resolveAIModel("another-tenant", values, ModelWorkspace); err == nil {
		t.Fatal("a credential from another tenant was accepted")
	}
}

func TestAIModelSettingsValidation(t *testing.T) {
	for key, raw := range map[string]string{
		"aiModels.deepseek.providerId": `"DeepSeek"`,
		"aiModels.deepseek.baseUrl":    `"https://username:password@example.com/v1"`,
		"aiModels.deepseek.protocol":   `"anthropic"`,
		"aiModels.deepseek.token":      `"invalid\nkey"`,
		"aiModels.route.workspace":     `"missing"`,
		"aiModels.default":             `"inherit"`,
	} {
		if d := FindDefinition(key); !d.Sensitive || d.Validate(raw) == nil {
			t.Errorf("expected a sensitive setting and invalid value for %s", key)
		}
	}
	for _, key := range []string{"aiModels.deepseek.token", "aiModels.custom.token"} {
		if d := FindDefinition(key); !d.Secret || d.Default != `""` {
			t.Errorf("expected an empty sealed credential for %s", key)
		}
	}
}

func TestModelChangesValidateOnlyAffectedAssignments(t *testing.T) {
	defaults := map[string]Value{}
	for _, d := range Registry {
		defaults[d.Key] = Value{Raw: d.Default}
	}
	next := make(map[string]Value)
	for key, value := range defaults {
		next[key] = value
	}
	next["aiModels.route.projectImport"] = Value{Raw: `"glm"`}
	if got := Code(ValidateAIModelChanges(defaults, next)); got != "ai_model_incomplete.glm.projectImport" {
		t.Fatalf("incomplete selection: %s", got)
	}
	next["assistant.token"] = Value{Raw: `"sealed"`, Configured: true}
	if err := ValidateAIModelChanges(defaults, next); err != nil {
		t.Fatalf("unconfigured default must not block a configured override: %v", err)
	}
	next["aiModels.glm.type"] = Value{Raw: `"embedding"`}
	if got := Code(ValidateAIModelChanges(defaults, next)); got != "ai_model_type_unsupported.glm.projectImport" {
		t.Fatalf("incompatible assignment: %s", got)
	}
	delete(next, "aiModels.route.projectImport")
	next["aiModels.route.projectImport"] = defaults["aiModels.route.projectImport"]
	if err := ValidateAIModelChanges(defaults, next); err != nil {
		t.Fatalf("an unassigned model can change type: %v", err)
	}
	next["aiModels.default"] = Value{Raw: `"glm"`}
	if got := Code(ValidateAIModelChanges(defaults, next)); got != "ai_model_type_unsupported.glm.default" {
		t.Fatalf("incompatible default: %s", got)
	}
}

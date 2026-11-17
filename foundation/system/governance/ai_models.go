package governance

import (
	"context"
	"regexp"
	"strings"

	"horizon/foundation/system/identity"
)

type AIModelUse string

const (
	ModelWorkspace     AIModelUse = "workspace"
	ModelAssistant     AIModelUse = "assistant"
	ModelKnowledge     AIModelUse = "knowledge"
	ModelWorkflow      AIModelUse = "workflow"
	ModelRuntimeDesign AIModelUse = "runtimeDesign"
	ModelProjectImport AIModelUse = "projectImport"
	ModelEventTrigger  AIModelUse = "eventTrigger"
	ModelAutomation    AIModelUse = "automation"
)

var modelUses = []AIModelUse{
	ModelWorkspace, ModelAssistant, ModelKnowledge, ModelWorkflow,
	ModelRuntimeDesign, ModelProjectImport, ModelEventTrigger, ModelAutomation,
}

const (
	ModelText       = "text"
	ModelEmbedding  = "embedding"
	ModelMultimodal = "multimodal"

	ChatCompletions = "chat-completions"
	Responses       = "responses"
)

var modelProviderID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func aiModelDefinitions() []Definition {
	var definitions []Definition
	add := func(key, kind, value string, min, max int64, choices ...string) {
		definitions = append(definitions, Definition{
			Key: "aiModels." + key, Group: "aiModels", Kind: kind, Default: value,
			Scopes: []string{"workspace"}, Choices: choices, Sensitive: true,
			Secret: kind == "secret", UI: true, Min: min, Max: max,
		})
	}
	add("default", "enum", `"deepseek"`, 0, 0, "deepseek", "glm", "custom")
	for _, use := range modelUses {
		add("route."+string(use), "enum", `"inherit"`, 0, 0, "inherit", "deepseek", "glm", "custom")
	}
	add("glm.type", "enum", `"text"`, 0, 0, ModelText, ModelEmbedding, ModelMultimodal)
	add("glm.providerId", "string", `"glm"`, 1, 64)
	add("custom.type", "enum", `"text"`, 0, 0, ModelText, ModelEmbedding, ModelMultimodal)
	add("glm.displayName", "string", `"GLM"`, 1, 100)
	for _, profile := range []string{"deepseek", "custom"} {
		id, name, endpoint, model := `"deepseek"`, `"DeepSeek-V4.1-Flash"`, `"https://api.deepseek.com"`, `"deepseek-flash"`
		if profile == "custom" {
			id, name, endpoint, model = `"custom"`, `""`, `""`, `""`
		}
		add(profile+".providerId", "string", id, 1, 64)
		add(profile+".displayName", "string", name, 0, 100)
		add(profile+".baseUrl", "string", endpoint, 0, 500)
		add(profile+".protocol", "enum", `"chat-completions"`, 0, 0, ChatCompletions)
		add(profile+".model", "string", model, 0, 100)
		add(profile+".token", "secret", `""`, 0, 500)
	}
	return definitions
}

// AIModel is resolved server-side for one trusted business use.
type AIModel struct {
	Profile, ProviderID, DisplayName, BaseURL, Protocol, Name, Token string
	Type                                                             string
	Enabled                                                          bool
}

func (m AIModel) Configured() bool {
	return m.DisplayName != "" && m.BaseURL != "" && m.Name != "" && m.Token != ""
}

func (c *Core) ResolveAIModel(ctx context.Context, p *identity.Principal, use AIModelUse) (AIModel, error) {
	if p == nil || p.Account == nil {
		return AIModel{}, Problem("forbidden")
	}
	values, err := c.Values(ctx, p.TenantID, []string{"workspace:0"})
	if err != nil {
		return AIModel{}, err
	}
	return c.resolveAIModel(p.TenantID, values, use)
}

func aiModelValue(values map[string]Value, key string) string {
	value, _ := String(values[key].Raw)
	return strings.TrimSpace(value)
}

func selectedAIModel(values map[string]Value, use AIModelUse) string {
	profile := aiModelValue(values, "aiModels.route."+string(use))
	if use == "" || profile == "inherit" {
		return aiModelValue(values, "aiModels.default")
	}
	return profile
}

func (c *Core) resolveAIModel(tenant string, values map[string]Value, use AIModelUse) (AIModel, error) {
	m, credentialKey, err := aiModelProfile(values, selectedAIModel(values, use), use)
	if err != nil {
		return AIModel{}, err
	}
	if m.Type == ModelEmbedding {
		return AIModel{}, Problem("ai_model_type_unsupported")
	}
	raw, err := c.settingValue(tenant, credentialKey, values[credentialKey])
	if err != nil {
		return AIModel{}, err
	}
	m.Token, err = String(raw)
	return m, err
}

func aiModelProfile(values map[string]Value, profile string, use AIModelUse) (AIModel, string, error) {
	read := func(key string) string { return aiModelValue(values, key) }
	enabledKey := "agent.enabled"
	if use == ModelAssistant {
		enabledKey = "assistant.enabled"
	}
	enabled, err := Bool(values[enabledKey].Raw)
	if err != nil {
		return AIModel{}, "", err
	}
	prefix := "aiModels." + profile + "."
	m := AIModel{Profile: profile, Enabled: enabled, Type: read(prefix + "type")}
	switch profile {
	case "glm":
		m.ProviderID, m.DisplayName = read(prefix+"providerId"), read(prefix+"displayName")
		m.BaseURL, m.Protocol = read("agent.baseUrl"), ChatCompletions
		if use == ModelAssistant {
			m.BaseURL, m.Protocol = read("assistant.baseUrl"), Responses
		}
		m.Name = read("assistant.model")
		return m, "assistant.token", nil
	case "deepseek", "custom":
		if profile == "deepseek" {
			m.Type = ModelMultimodal
		}
		m.ProviderID, m.DisplayName, m.BaseURL = read(prefix+"providerId"), read(prefix+"displayName"), read(prefix+"baseUrl")
		m.Protocol, m.Name = read(prefix+"protocol"), read(prefix+"model")
		return m, prefix + "token", nil
	default:
		return AIModel{}, "", Problem("invalid")
	}
}

// IsAIModelSetting identifies model configuration shared by the settings pages.
func IsAIModelSetting(key string) bool {
	return strings.HasPrefix(key, "aiModels.") || Has([]string{"agent.baseUrl", "assistant.baseUrl", "assistant.model", "assistant.token"}, key)
}

// ValidateAIModelChanges checks the affected selections against the prospective
// settings snapshot. Credential values are still sealed at this point.
func ValidateAIModelChanges(current, next map[string]Value) error {
	changed := func(key string) bool { return current[key].Raw != next[key].Raw }
	defaultChanged := changed("aiModels.default")
	for _, use := range append([]AIModelUse{""}, modelUses...) {
		profile := selectedAIModel(next, use)
		prefix := "aiModels." + profile + "."
		routeKey := "aiModels.route." + string(use)
		selectionChanged := use == "" && defaultChanged ||
			use != "" && (changed(routeKey) || (defaultChanged && aiModelValue(next, routeKey) == "inherit"))
		if !selectionChanged && !changed(prefix+"type") {
			continue
		}
		m, credentialKey, err := aiModelProfile(next, profile, use)
		if err != nil {
			return err
		}
		scene := string(use)
		if scene == "" {
			scene = "default"
		}
		if m.Type != ModelText && m.Type != ModelMultimodal {
			return Problem("ai_model_type_unsupported." + profile + "." + scene)
		}
		if m.ProviderID == "" || m.DisplayName == "" || m.BaseURL == "" || m.Name == "" || !next[credentialKey].Configured {
			return Problem("ai_model_incomplete." + profile + "." + scene)
		}
	}
	return nil
}

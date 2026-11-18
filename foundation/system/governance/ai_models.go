package governance

import (
	"context"
	"maps"
	"regexp"
	"slices"
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
var customModelID = regexp.MustCompile(`^custom-[a-z0-9][a-z0-9-]{0,55}$`)

func customModelField(key string) (profile, field string, ok bool) {
	key, ok = strings.CutPrefix(key, "aiModels.")
	if !ok {
		return "", "", false
	}
	profile, field, _ = strings.Cut(key, ".")
	return profile, field, customModelID.MatchString(profile)
}

// Custom models reuse the setting definitions of the original custom slot.
// Their keys have no effective default until the model has been saved.
func customModelDefinition(definitions []Definition, key string) *Definition {
	_, field, ok := customModelField(key)
	if !ok || field == "" {
		return nil
	}
	template := "aiModels.custom." + field
	for _, definition := range definitions {
		if definition.Key == template {
			definition.Key = key
			definition.UI = false
			return &definition
		}
	}
	return nil
}

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
	Dimensions                                                       int64
	Enabled                                                          bool
}

func (m AIModel) Configured() bool {
	return m.DisplayName != "" && m.BaseURL != "" && m.Name != "" && m.Token != ""
}

func (c *Core) ResolveAIModel(ctx context.Context, p *identity.Principal, use AIModelUse) (AIModel, error) {
	if p == nil || p.Account == nil {
		return AIModel{}, Problem("forbidden")
	}
	snapshot, err := LoadAIConfiguration(ctx, c.Q, p.TenantID)
	if err != nil {
		return AIModel{}, err
	}
	return c.resolveAIModel(p.TenantID, snapshot.Values(), use)
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
	if !isModelProfile(profile) {
		return AIModel{}, "", Problem("invalid")
	}
	field := func(name string) string { return read(modelSettingKey(profile, name)) }
	m := AIModel{
		Profile: profile, Enabled: enabled, Type: field("type"),
		ProviderID: field("providerId"), DisplayName: field("displayName"),
		BaseURL: field("baseUrl"), Name: field("model"),
		Dimensions: modelProfileDimensions(values, profile),
	}
	if m.DisplayName == "" && (profile == BailianEmbeddingProfile || profile == LegacyEmbeddingProfile) {
		m.DisplayName = m.ProviderID
	}
	m.Protocol = modelProfileProtocol(values, profile, m.Type)
	if use == ModelAssistant && m.Type != ModelEmbedding {
		if endpoint := field("responsesBaseUrl"); endpoint != "" {
			m.BaseURL, m.Protocol = endpoint, Responses
		}
	}
	return m, modelSettingKey(profile, "token"), nil
}

// IsAIModelSetting identifies model connection and assignment keys.
func IsAIModelSetting(key string) bool {
	return strings.HasPrefix(key, "aiModels.") || Has([]string{"agent.baseUrl", "assistant.baseUrl", "assistant.model", "assistant.token"}, key)
}

func IsAISetting(key string) bool {
	return IsAIModelSetting(key) || strings.HasPrefix(key, "knowledge.") ||
		key == "agent.enabled" || key == "assistant.enabled"
}

// ValidateAIModelChanges checks the affected selections against the prospective
// settings snapshot. Credential values are still sealed at this point.
func ValidateAIModelChanges(current, next map[string]Value) error {
	changed := func(key string) bool { return current[key].Raw != next[key].Raw }
	changedProfiles := map[string]bool{}
	presentProfiles := map[string]bool{}
	for key := range next {
		if profile, _, ok := customModelField(key); ok {
			presentProfiles[profile] = true
			changedProfiles[profile] = changedProfiles[profile] || changed(key)
		}
	}
	for key := range current {
		if profile, _, ok := customModelField(key); ok && changed(key) {
			changedProfiles[profile] = true
		}
	}
	for _, profile := range presetProfiles {
		for _, field := range []string{"type", "protocol", "baseUrl", "model", "dimensions"} {
			if modelFieldChanged(current, next, profile, field) {
				changedProfiles[profile] = true
				presentProfiles[profile] = true
			}
		}
	}
	for _, profile := range slices.Sorted(maps.Keys(changedProfiles)) {
		if !changedProfiles[profile] || !presentProfiles[profile] {
			continue
		}
		model, _, err := aiModelProfile(next, profile, "")
		if err != nil {
			return err
		}
		if err := validateEmbeddingProtocol(model); err != nil {
			return err
		}
	}
	defaultChanged := changed("aiModels.default")
	for _, use := range append([]AIModelUse{""}, modelUses...) {
		profile := selectedAIModel(next, use)
		routeKey := "aiModels.route." + string(use)
		selectionChanged := use == "" && defaultChanged ||
			use != "" && (changed(routeKey) || (defaultChanged && aiModelValue(next, routeKey) == "inherit"))
		if !selectionChanged && !modelFieldChanged(current, next, profile, "type") &&
			!(customModelID.MatchString(profile) && changedProfiles[profile]) {
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
		if !completeAIModel(m, next[credentialKey]) {
			return Problem("ai_model_incomplete." + profile + "." + scene)
		}
	}
	for _, profile := range slices.Sorted(maps.Keys(changedProfiles)) {
		// Restoring a snapshot from before this model existed removes all its keys.
		if !customModelID.MatchString(profile) || !changedProfiles[profile] || !presentProfiles[profile] {
			continue
		}
		m, credentialKey, err := aiModelProfile(next, profile, "")
		if err != nil {
			return err
		}
		if m.Type == "" || !completeAIModel(m, next[credentialKey]) {
			return Problem("ai_model_incomplete." + profile + ".models")
		}
	}
	return validateKnowledgeEmbedding(next)
}

func completeAIModel(model AIModel, credential Value) bool {
	return model.ProviderID != "" && model.DisplayName != "" && model.BaseURL != "" && model.Name != "" && credential.Configured
}

package governance

import "slices"

const (
	OpenAIEmbedding        = "openai-embedding"
	DashScopeEmbedding     = "dashscope-embedding"
	LegacyEmbeddingProfile = "legacy-embedding"
)

var modelProtocols = []string{ChatCompletions, OpenAIEmbedding, DashScopeEmbedding}
var presetProfiles = []string{"deepseek", "glm", "custom", LegacyEmbeddingProfile, BailianEmbeddingProfile}

var unifiedModelDefinitions = buildUnifiedModelDefinitions()

func buildUnifiedModelDefinitions() []Definition {
	var definitions []Definition
	add := func(profile, field, kind, value string, min, max int64, choices ...string) {
		definitions = append(definitions, Definition{
			Key: "aiModels." + profile + "." + field, Group: "aiModels", Kind: kind, Default: value,
			Scopes: []string{"workspace"}, Sensitive: true, Choices: choices, Min: min, Max: max,
		})
	}
	add("deepseek", "type", "enum", `"multimodal"`, 0, 0, ModelText, ModelEmbedding, ModelMultimodal)
	for _, profile := range []string{LegacyEmbeddingProfile, BailianEmbeddingProfile} {
		add(profile, "type", "enum", `"embedding"`, 0, 0, ModelText, ModelEmbedding, ModelMultimodal)
		add(profile, "displayName", "string", `""`, 0, 100)
	}
	add(LegacyEmbeddingProfile, "providerId", "string", `"openai"`, 1, 64)
	add(BailianEmbeddingProfile, "providerId", "string", `"bailian"`, 1, 64)
	add(BailianEmbeddingProfile, "model", "string", `"qwen3-vl-embedding"`, 1, 100)
	add(BailianEmbeddingProfile, "dimensions", "integer", "2560", 1, 8192)
	add(LegacyEmbeddingProfile, "protocol", "enum", `"openai-embedding"`, 0, 0, modelProtocols...)
	add(BailianEmbeddingProfile, "protocol", "enum", `"dashscope-embedding"`, 0, 0, modelProtocols...)
	add("glm", "protocol", "enum", `"chat-completions"`, 0, 0, modelProtocols...)
	for _, profile := range []string{"deepseek", "glm", "custom"} {
		add(profile, "dimensions", "integer", "1536", 1, 8192)
	}
	for _, profile := range []string{"deepseek", "custom", LegacyEmbeddingProfile, BailianEmbeddingProfile} {
		add(profile, "responsesBaseUrl", "string", `""`, 0, 500)
	}
	return definitions
}

// IsUnifiedAIModelSetting keeps retained Runtime definitions at their original
// field sets. The unified editor reads the current field catalog.
func IsUnifiedAIModelSetting(key string) bool {
	return slices.ContainsFunc(unifiedModelDefinitions, func(definition Definition) bool { return definition.Key == key })
}

func isModelProfile(profile string) bool {
	return slices.Contains(presetProfiles, profile) || customModelID.MatchString(profile)
}

func modelSettingKey(profile, field string) string {
	switch profile {
	case "glm":
		switch field {
		case "baseUrl":
			return "agent.baseUrl"
		case "responsesBaseUrl":
			return "assistant.baseUrl"
		case "model", "token":
			return "assistant." + field
		}
	case LegacyEmbeddingProfile:
		switch field {
		case "baseUrl", "model", "token", "dimensions":
			return "knowledge.embedding." + field
		}
	}
	return "aiModels." + profile + "." + field
}

func modelProfileProtocol(values map[string]Value, profile, modelType string) string {
	key := modelSettingKey(profile, "protocol")
	protocol := aiModelValue(values, key)
	// Older custom models could be designated as embeddings before an embedding
	// protocol was persisted. Treat the implicit default as OpenAI embedding.
	if protocol == "" || (protocol == ChatCompletions && modelType == ModelEmbedding && values[key].Source != "workspace:0") {
		if modelType == ModelEmbedding {
			return OpenAIEmbedding
		}
		return ChatCompletions
	}
	return protocol
}

func modelProfileDimensions(values map[string]Value, profile string) int64 {
	dimensions, _ := Number(values[modelSettingKey(profile, "dimensions")].Raw)
	if dimensions < 1 {
		return 1536
	}
	return dimensions
}

func modelFieldChanged(before, after map[string]Value, profile, field string) bool {
	key := modelSettingKey(profile, field)
	return before[key].Raw != after[key].Raw || before[key].Source != after[key].Source
}

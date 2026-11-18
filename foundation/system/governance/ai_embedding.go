package governance

import (
	"context"
	"maps"
	"net/url"
	"slices"
	"strings"

	"horizon/foundation/system/identity"
)

const (
	BailianEmbeddingProfile      = "bailian"
	BailianEmbeddingModel        = "qwen3-vl-embedding"
	BailianEmbeddingDimensions   = 2560
	bailianBaseURLKey            = "aiModels.bailian.baseUrl"
	bailianTokenKey              = "aiModels.bailian.token"
	KnowledgeEmbeddingProfileKey = "knowledge.embedding.profile"
)

func knowledgeEmbeddingDefinitions() []Definition {
	return []Definition{
		{Key: KnowledgeEmbeddingProfileKey, Group: "knowledge", Kind: "enum", Default: `"legacy-embedding"`,
			Scopes: []string{"workspace"}, UI: true, Sensitive: true, Choices: []string{LegacyEmbeddingProfile, BailianEmbeddingProfile}},
		{Key: bailianBaseURLKey, Group: "aiModels", Kind: "string", Default: `"https://dashscope.aliyuncs.com/api/v1"`,
			Scopes: []string{"workspace"}, UI: true, Sensitive: true, Min: 8, Max: 500},
		{Key: bailianTokenKey, Group: "aiModels", Kind: "secret", Default: `""`,
			Scopes: []string{"workspace"}, UI: true, Sensitive: true, Secret: true, Max: 500},
	}
}

// IsBailianEmbeddingSetting identifies fields added with the DashScope preset.
func IsBailianEmbeddingSetting(key string) bool {
	return key == KnowledgeEmbeddingProfileKey || key == bailianBaseURLKey || key == bailianTokenKey
}

func validBailianBaseURL(u *url.URL) bool {
	if u.Scheme != "https" || u.Port() != "" || u.ForceQuery || strings.TrimSuffix(u.EscapedPath(), "/") != "/api/v1" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "dashscope.aliyuncs.com" || host == "dashscope-intl.aliyuncs.com" {
		return true
	}
	for _, suffix := range []string{".cn-beijing.maas.aliyuncs.com", ".ap-southeast-1.maas.aliyuncs.com"} {
		workspace, ok := strings.CutSuffix(host, suffix)
		if ok && workspace != "" && !strings.Contains(workspace, ".") {
			return true
		}
	}
	return false
}

func validateEmbeddingProtocol(model AIModel) error {
	if model.Type != ModelEmbedding {
		if model.Protocol != ChatCompletions {
			return Problem("ai_model_protocol_mismatch")
		}
		return nil
	}
	switch model.Protocol {
	case OpenAIEmbedding:
		return nil
	case DashScopeEmbedding:
		endpoint, err := url.Parse(model.BaseURL)
		if err == nil && validBailianBaseURL(endpoint) && model.Name == BailianEmbeddingModel {
			switch model.Dimensions {
			case 256, 512, 768, 1024, 1536, 2048, BailianEmbeddingDimensions:
				return nil
			}
		}
		return Problem("ai_embedding_dashscope_invalid")
	default:
		return Problem("ai_model_protocol_mismatch")
	}
}

func validateKnowledgeEmbedding(values map[string]Value) error {
	profile := aiModelValue(values, KnowledgeEmbeddingProfileKey)
	if profile == "" {
		return nil
	}
	model, tokenKey, err := aiModelProfile(values, profile, "")
	if err != nil || model.Type != ModelEmbedding {
		return Problem("ai_embedding_model_required")
	}
	if err := validateEmbeddingProtocol(model); err != nil {
		return err
	}
	if profile != LegacyEmbeddingProfile && !completeAIModel(model, values[tokenKey]) {
		return Problem("ai_embedding_incomplete")
	}
	return nil
}

type KnowledgeVectorSettings struct {
	AIModel
	RedisURL, Index string
	LegacyIndex     bool
}

// KnowledgeVector reads the embedder and vector store from the same committed
// workspace snapshot. Secrets stay inside the server.
func (c *Core) KnowledgeVector(ctx context.Context, p *identity.Principal) (KnowledgeVectorSettings, error) {
	snapshot, err := LoadAIConfiguration(ctx, c.Q, p.TenantID)
	if err != nil {
		return KnowledgeVectorSettings{}, err
	}
	values := snapshot.Values()
	profile := aiModelValue(values, KnowledgeEmbeddingProfileKey)
	if err := validateKnowledgeEmbedding(values); err != nil {
		return KnowledgeVectorSettings{}, err
	}
	model, tokenKey, err := aiModelProfile(values, profile, "")
	if err != nil || !completeAIModel(model, values[tokenKey]) {
		return KnowledgeVectorSettings{}, Problem("ai_embedding_incomplete")
	}
	raw, err := c.settingValue(p.TenantID, tokenKey, values[tokenKey])
	if err != nil {
		return KnowledgeVectorSettings{}, err
	}
	model.Token, err = String(raw)
	if err != nil {
		return KnowledgeVectorSettings{}, err
	}
	redisURL := strings.TrimSpace(c.Options.KnowledgeRedisURL)
	if redisURL == "" {
		redisURL = aiModelValue(values, "knowledge.vector.url")
	}
	index := aiModelValue(values, "knowledge.vector.index")
	if redisURL == "" || index == "" {
		return KnowledgeVectorSettings{}, Problem("knowledge_vector_store_unavailable")
	}
	return KnowledgeVectorSettings{
		AIModel: model, RedisURL: redisURL, Index: index,
		LegacyIndex: profile == LegacyEmbeddingProfile && model.Protocol == OpenAIEmbedding &&
			values[modelSettingKey(profile, "protocol")].Source != "workspace:0",
	}, nil
}

// EmbeddingModelOptions exposes model labels and IDs only. The save executor
// separately verifies that the selected model is usable.
func EmbeddingModelOptions(values map[string]Value) []map[string]any {
	profiles := slices.Clone(presetProfiles)
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if profile, field, ok := customModelField(key); ok && field == "displayName" {
			profiles = append(profiles, profile)
		}
	}
	options := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		if aiModelValue(values, modelSettingKey(profile, "type")) != ModelEmbedding {
			continue
		}
		label := aiModelValue(values, modelSettingKey(profile, "displayName"))
		if label == "" {
			label = aiModelValue(values, modelSettingKey(profile, "model"))
		}
		options = append(options, map[string]any{"id": profile, "label": label})
	}
	return options
}

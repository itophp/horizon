package knowledge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"

	embedopenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	redisindexer "github.com/cloudwego/eino-ext/components/indexer/redis"
	redisretriever "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

const (
	vectorField       = "vector"
	hashPrefix        = "kb:"
	customHashPrefix  = "kb-model-"
	bailianHashPrefix = "kb-bailian-qwen3-vl-embedding-2560:"
)

// Vector bundles the Eino components retrieval needs. There is no bespoke
// search code here: the indexer embeds and stores, the retriever embeds the
// query and searches, both provided by eino-ext.
type Vector struct {
	Indexer   indexer.Indexer
	Retriever retriever.Retriever
	Client    *redis.Client
	Index     string
	keyPrefix string
}

// vectorFor builds the stack from a single committed settings snapshot.
// Callers close its Redis client once their operation has finished.
func (s *Store) vectorFor(ctx context.Context, p *identity.Principal) (*Vector, error) {
	if s == nil || s.Core == nil {
		return nil, governance.Problem("unavailable")
	}
	config, err := s.Core.KnowledgeVector(ctx, p)
	if err != nil {
		return nil, err
	}
	index, prefix := vectorLocation(config)
	var embedder embedding.Embedder
	if config.Protocol == governance.DashScopeEmbedding {
		embedder = dashScopeEmbedder{baseURL: config.BaseURL, token: config.Token, model: config.Name, dimensions: int(config.Dimensions)}
	} else {
		openAI := &embedopenai.EmbeddingConfig{
			APIKey: config.Token, BaseURL: strings.TrimSuffix(config.BaseURL, "/"), Model: config.Name,
		}
		if !config.LegacyIndex {
			dimensions := int(config.Dimensions)
			openAI.Dimensions = &dimensions
		}
		embedder, err = embedopenai.NewEmbedder(ctx, openAI)
		if err != nil {
			return nil, err
		}
	}
	client, err := redisClient(config.RedisURL)
	if err != nil {
		return nil, governance.Problem("knowledge_vector_store_unavailable")
	}
	stack, err := buildVector(ctx, client, embedder, index, prefix, config.Dimensions)
	if err != nil {
		client.Close()
		return nil, err
	}
	return stack, nil
}

func vectorLocation(config governance.KnowledgeVectorSettings) (index, prefix string) {
	if config.LegacyIndex {
		return config.Index, hashPrefix
	}
	if config.Profile == governance.BailianEmbeddingProfile && config.Protocol == governance.DashScopeEmbedding &&
		config.Name == governance.BailianEmbeddingModel && config.Dimensions == governance.BailianEmbeddingDimensions {
		return config.Index + "-bailian-qwen3-vl-embedding-2560", bailianHashPrefix
	}
	id := governance.Hash(strings.Join([]string{
		config.Protocol, config.BaseURL, config.Name, strconv.FormatInt(config.Dimensions, 10),
	}, "\x00"))
	return config.Index + "-model-" + id, customHashPrefix + id + ":"
}

func buildVector(ctx context.Context, client *redis.Client, embedder embedding.Embedder, index, prefix string, dimensions int64) (*Vector, error) {
	if err := ensureIndex(ctx, client, index, prefix, dimensions); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, governance.Problem("knowledge_vector_store_unavailable")
	}
	retrieved, err := redisretriever.NewRetriever(ctx, &redisretriever.RetrieverConfig{
		Client: client, Index: index, VectorField: vectorField,
		Embedding: embedder, TopK: int(Limit),
		ReturnFields: []string{"content", "title", "document_id", "document_version"},
		DocumentConverter: func(_ context.Context, result redis.Document) (*schema.Document, error) {
			return &schema.Document{
				ID: result.ID, Content: result.Fields["content"],
				MetaData: map[string]any{
					"title": result.Fields["title"], "document_id": result.Fields["document_id"],
					"document_version": result.Fields["document_version"],
				},
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}
	stored, err := redisindexer.NewIndexer(ctx, &redisindexer.IndexerConfig{
		Client: client, KeyPrefix: prefix, Embedding: embedder,
		DocumentToHashes: func(_ context.Context, doc *schema.Document) (*redisindexer.Hashes, error) {
			return hashesFor(doc)
		},
	})
	if err != nil {
		return nil, err
	}
	return &Vector{Indexer: stored, Retriever: retrieved, Client: client, Index: index, keyPrefix: prefix}, nil
}

// redisClient needs protocol 2 with UnstableResp3 enabled: the retriever parses
// raw FT.SEARCH replies, and RESP3 wraps them differently.
func redisClient(url string) (*redis.Client, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	options.Protocol = 2
	options.UnstableResp3 = true
	return redis.NewClient(options), nil
}

// hashesFor maps a fragment onto the RediSearch schema. tenant is a TAG so a
// query can be narrowed to one workspace with WithFilterQuery.
func hashesFor(doc *schema.Document) (*redisindexer.Hashes, error) {
	tenant, _ := doc.MetaData["tenant"].(string)
	if tenant == "" {
		return nil, governance.Problem("invalid")
	}
	documentID, _ := doc.MetaData["documentId"].(int64)
	title, _ := doc.MetaData["title"].(string)
	version, _ := doc.MetaData["version"].(string)
	return &redisindexer.Hashes{
		Key: doc.ID,
		Field2Value: map[string]redisindexer.FieldValue{
			"tenant":           {Value: tenant},
			"document_id":      {Value: documentID},
			"title":            {Value: title},
			"document_version": {Value: version},
			"content":          {Value: doc.Content, EmbedKey: vectorField},
		},
	}, nil
}

// ensureIndex creates the RediSearch index once. The schema is the contract the
// retriever and indexer rely on, so it is created here rather than left to an
// operator reading a README.
func ensureIndex(ctx context.Context, client *redis.Client, index, prefix string, dimensions int64) error {
	exists, err := client.Do(ctx, "FT._LIST").Result()
	if err != nil {
		return err
	}
	if list, ok := exists.([]any); ok {
		for _, name := range list {
			if text, ok := name.(string); ok && text == index {
				return nil
			}
		}
	}
	err = client.Do(ctx, "FT.CREATE", index,
		"ON", "HASH",
		"PREFIX", "1", prefix,
		"SCHEMA",
		"tenant", "TAG",
		"document_id", "NUMERIC",
		"title", "TEXT",
		"content", "TEXT",
		vectorField, "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", strconv.FormatInt(dimensions, 10),
		"DISTANCE_METRIC", "COSINE",
	).Err()
	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		return err
	}
	return nil
}

// storeDocuments writes fragments through the Eino indexer.
func (v *Vector) storeDocuments(ctx context.Context, docs []*schema.Document) error {
	if v == nil || v.Indexer == nil {
		return governance.Problem("unavailable")
	}
	_, err := v.Indexer.Store(ctx, docs)
	return err
}

func (v *Vector) discardFragments(ctx context.Context, docs []*schema.Document) {
	keys := make([]string, len(docs))
	for i, doc := range docs {
		keys[i] = v.keyPrefix + doc.ID
	}
	// Failed generations are invisible to search even if cleanup is interrupted.
	_ = v.Client.Unlink(ctx, keys...).Err()
}

func (v *Vector) cleanDocumentVersions(ctx context.Context, tenant string, id int64, shouldRemove func(string, int) bool) error {
	prefix := fmt.Sprintf("%s%s-%d-", v.keyPrefix, tenant, id)
	pattern := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]").Replace(prefix) + "*"
	iterator := v.Client.Scan(ctx, 0, pattern, 200).Iterator()
	for iterator.Next(ctx) {
		suffix, found := strings.CutPrefix(iterator.Val(), prefix)
		version, number, versioned := strings.Cut(suffix, "-")
		if !versioned {
			version, number = "", suffix
		}
		ordinal, err := strconv.Atoi(number)
		if found && err == nil && shouldRemove(version, ordinal) {
			if err := v.Client.Unlink(ctx, iterator.Val()).Err(); err != nil {
				return err
			}
		}
	}
	return iterator.Err()
}

// dropDocument removes every fragment of one document. Keys are prefixed with
// tenant and document id, so a scan is enough and no secondary index is needed.
func (v *Vector) dropDocument(ctx context.Context, tenant string, id int64) error {
	if v == nil || v.Client == nil {
		return governance.Problem("unavailable")
	}
	pattern := fmt.Sprintf("%s%s-%d-*", v.keyPrefix, tenant, id)
	iterator := v.Client.Scan(ctx, 0, pattern, 200).Iterator()
	for iterator.Next(ctx) {
		if err := v.Client.Del(ctx, iterator.Val()).Err(); err != nil {
			return err
		}
	}
	return iterator.Err()
}

// retrieve runs the Eino retriever narrowed to one workspace.
func (v *Vector) retrieve(ctx context.Context, tenant, query string, limit int) ([]*schema.Document, error) {
	if v == nil || v.Retriever == nil {
		return nil, governance.Problem("unavailable")
	}
	options := []retriever.Option{redisretriever.WithFilterQuery(workspaceTagFilter(tenant))}
	if limit > 0 {
		options = append(options, retriever.WithTopK(limit))
	}
	return v.Retriever.Retrieve(ctx, query, options...)
}

// Escape Redis's ASCII tag syntax while preserving the stored workspace ID.
func workspaceTagFilter(tenant string) string {
	var filter strings.Builder
	filter.WriteString("@tenant:{")
	for _, r := range tenant {
		if r <= unicode.MaxASCII && (unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r)) {
			filter.WriteByte('\\')
		}
		filter.WriteRune(r)
	}
	filter.WriteByte('}')
	return filter.String()
}

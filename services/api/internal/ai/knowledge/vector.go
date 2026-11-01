package knowledge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"

	embedopenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	redisindexer "github.com/cloudwego/eino-ext/components/indexer/redis"
	redisretriever "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// Configuration keys behind the vector stack. They stay deployment settings, so
// a workspace can point at its own embedding service and vector store.
const (
	keyEmbeddingBaseURL = "knowledge.embedding.baseUrl"
	keyEmbeddingModel   = "knowledge.embedding.model"
	keyEmbeddingDims    = "knowledge.embedding.dimensions"
	keyEmbeddingToken   = "knowledge.embedding.token"
	keyVectorURL        = "knowledge.vector.url"
	keyVectorIndex      = "knowledge.vector.index"

	// vectorField is the RediSearch field that holds the dense vector. The
	// retriever and the index definition must agree on it.
	vectorField = "vector"
	hashPrefix  = "kb:"
)

// Vector bundles the Eino components retrieval needs. There is no bespoke
// search code here: the indexer embeds and stores, the retriever embeds the
// query and searches, both provided by eino-ext.
type Vector struct {
	Indexer   indexer.Indexer
	Retriever retriever.Retriever
	Client    *redis.Client
	Index     string
}

// vectors caches one stack per tenant so a page request does not rebuild the
// Redis client on every keystroke. A configuration change is picked up on
// restart, which matches how the rest of the platform treats deployment config.
type vectorCache struct {
	mu      sync.Mutex
	entries map[string]*Vector
}

// vectorFor builds (or returns) the component stack for the caller's workspace.
// A missing credential is reported as unavailable, never as an internal error:
// an operator has to configure it, the request is not broken.
func (s *Store) vectorFor(ctx context.Context, p *identity.Principal) (*Vector, error) {
	if s == nil || s.Core == nil {
		return nil, governance.Problem("unavailable")
	}
	if cached, ok := s.vectors().get(p.TenantID); ok {
		return cached, nil
	}
	token, err := s.setting(ctx, p, keyEmbeddingToken)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, governance.Problem("unavailable")
	}
	baseURL, err := s.setting(ctx, p, keyEmbeddingBaseURL)
	if err != nil {
		return nil, err
	}
	model, err := s.setting(ctx, p, keyEmbeddingModel)
	if err != nil {
		return nil, err
	}
	dimensionValue, err := s.Core.Setting(ctx, p, keyEmbeddingDims)
	if err != nil {
		return nil, err
	}
	dimensions, err := governance.Number(dimensionValue)
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	url, err := s.setting(ctx, p, keyVectorURL)
	if err != nil {
		return nil, err
	}
	index, err := s.setting(ctx, p, keyVectorIndex)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(url) == "" || strings.TrimSpace(model) == "" || dimensions <= 0 {
		return nil, governance.Problem("unavailable")
	}

	embedder, err := embedopenai.NewEmbedder(ctx, &embedopenai.EmbeddingConfig{
		APIKey:  token,
		BaseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		Model:   model,
	})
	if err != nil {
		return nil, err
	}
	client, err := redisClient(strings.TrimSpace(url))
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	if err = ensureIndex(ctx, client, index, dimensions); err != nil {
		return nil, err
	}
	retrieved, err := redisretriever.NewRetriever(ctx, &redisretriever.RetrieverConfig{
		Client:      client,
		Index:       index,
		VectorField: vectorField,
		Embedding:   embedder,
		TopK:        int(Limit),
	})
	if err != nil {
		return nil, err
	}
	stored, err := redisindexer.NewIndexer(ctx, &redisindexer.IndexerConfig{
		Client:    client,
		KeyPrefix: hashPrefix,
		Embedding: embedder,
		DocumentToHashes: func(_ context.Context, doc *schema.Document) (*redisindexer.Hashes, error) {
			return hashesFor(doc)
		},
	})
	if err != nil {
		return nil, err
	}
	stack := &Vector{Indexer: stored, Retriever: retrieved, Client: client, Index: index}
	s.vectors().put(p.TenantID, stack)
	return stack, nil
}

func (s *Store) vectors() *vectorCache {
	s.vectorOnce.Do(func() { s.vectorCache = &vectorCache{entries: map[string]*Vector{}} })
	return s.vectorCache
}

func (c *vectorCache) get(tenant string) (*Vector, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.entries[tenant]
	return value, ok
}

func (c *vectorCache) put(tenant string, value *Vector) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[tenant] = value
}

func (s *Store) setting(ctx context.Context, p *identity.Principal, key string) (string, error) {
	raw, err := s.Core.Setting(ctx, p, key)
	if err != nil {
		return "", err
	}
	value, err := governance.String(raw)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
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
	return &redisindexer.Hashes{
		Key: doc.ID,
		Field2Value: map[string]redisindexer.FieldValue{
			"tenant":      {Value: tenant},
			"document_id": {Value: documentID},
			"title":       {Value: title},
			"content":     {Value: doc.Content, EmbedKey: vectorField},
		},
	}, nil
}

// ensureIndex creates the RediSearch index once. The schema is the contract the
// retriever and indexer rely on, so it is created here rather than left to an
// operator reading a README.
func ensureIndex(ctx context.Context, client *redis.Client, index string, dimensions int64) error {
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
		"PREFIX", "1", hashPrefix,
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

// dropDocument removes every fragment of one document. Keys are prefixed with
// tenant and document id, so a scan is enough and no secondary index is needed.
func (v *Vector) dropDocument(ctx context.Context, tenant string, id int64) error {
	if v == nil || v.Client == nil {
		return governance.Problem("unavailable")
	}
	pattern := fmt.Sprintf("%s%s-%d-*", hashPrefix, tenant, id)
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
	options := []retriever.Option{redisretriever.WithFilterQuery("@" + "tenant" + ":{" + tenant + "}")}
	if limit > 0 {
		options = append(options, retriever.WithTopK(limit))
	}
	return v.Retriever.Retrieve(ctx, query, options...)
}

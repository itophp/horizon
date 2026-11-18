package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"horizon/foundation/system/governance"
)

const dashScopeEmbeddingPath = "/services/embeddings/multimodal-embedding/multimodal-embedding"
const dashScopeBatchSize = 20

var dashScopeHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type dashScopeEmbedder struct {
	baseURL    string
	token      string
	model      string
	dimensions int
}

type dashScopeContent struct {
	Text string `json:"text"`
}

type dashScopeRequest struct {
	Model string `json:"model"`
	Input struct {
		Contents []dashScopeContent `json:"contents"`
	} `json:"input"`
	Parameters struct {
		Dimension int `json:"dimension"`
	} `json:"parameters"`
}

type dashScopeResponse struct {
	Code   string `json:"code"`
	Output struct {
		Embeddings []struct {
			Index  *int      `json:"index"`
			Vector []float64 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
}

func (e dashScopeEmbedder) EmbedStrings(ctx context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += dashScopeBatchSize {
		batch, err := e.embed(ctx, texts[start:min(start+dashScopeBatchSize, len(texts))])
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batch...)
	}
	return vectors, nil
}

func (e dashScopeEmbedder) embed(ctx context.Context, texts []string) ([][]float64, error) {
	payload := dashScopeRequest{Model: e.model}
	payload.Parameters.Dimension = e.dimensions
	payload.Input.Contents = make([]dashScopeContent, len(texts))
	for i, text := range texts {
		payload.Input.Contents[i] = dashScopeContent{Text: text}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(e.baseURL, "/")+dashScopeEmbeddingPath, bytes.NewReader(raw))
	if err != nil {
		return nil, governance.Problem("unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := dashScopeHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, governance.Problem("unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, governance.Problem("knowledge_embedding_authentication_failed")
	}
	var result dashScopeResponse
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&result) != nil ||
		result.Code != "" || len(result.Output.Embeddings) != len(texts) {
		return nil, governance.Problem("unavailable")
	}
	vectors := make([][]float64, len(texts))
	for _, record := range result.Output.Embeddings {
		if record.Index == nil || *record.Index < 0 || *record.Index >= len(vectors) ||
			vectors[*record.Index] != nil || len(record.Vector) != e.dimensions {
			return nil, governance.Problem("unavailable")
		}
		vectors[*record.Index] = record.Vector
	}
	return vectors, nil
}

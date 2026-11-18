package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"horizon/foundation/system/governance"
)

func TestDashScopeEmbedderBatchesAndPreservesOrder(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := int(calls.Add(1)-1) * dashScopeBatchSize
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1"+dashScopeEmbeddingPath ||
			r.Header.Get("Authorization") != "Bearer test-only-key" {
			t.Error("unexpected DashScope method, endpoint or authentication")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var request dashScopeRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil ||
			request.Model != "qwen3-vl-embedding" || request.Parameters.Dimension != 2560 ||
			len(request.Input.Contents) != min(dashScopeBatchSize, 21-start) {
			t.Errorf("invalid DashScope request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		embeddings := make([]map[string]any, 0, len(request.Input.Contents))
		for i := len(request.Input.Contents) - 1; i >= 0; i-- {
			if request.Input.Contents[i].Text != fmt.Sprintf("Text %d", start+i) {
				t.Error("wrong input")
			}
			vector := make([]float64, 2560)
			vector[0] = float64(start + i)
			embeddings = append(embeddings, map[string]any{"index": i, "embedding": vector, "type": "text"})
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"output": map[string]any{"embeddings": embeddings}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	texts := make([]string, 21)
	for i := range texts {
		texts[i] = fmt.Sprintf("Text %d", i)
	}
	vectors, err := (dashScopeEmbedder{baseURL: server.URL + "/api/v1/", token: "test-only-key", model: governance.BailianEmbeddingModel, dimensions: 2560}).EmbedStrings(t.Context(), texts)
	if err != nil || len(vectors) != len(texts) || calls.Load() != 2 {
		t.Fatalf("embedding batch failed: %v", err)
	}
	for i, vector := range vectors {
		if len(vector) != 2560 || vector[0] != float64(i) {
			t.Fatalf("unexpected vector at input %d", i)
		}
	}
}

func TestDashScopeEmbedderRejectsWrongDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"output":{"embeddings":[{"index":0,"embedding":[0.1,0.2]}]}}`))
	}))
	defer server.Close()
	if _, err := (dashScopeEmbedder{baseURL: server.URL, model: governance.BailianEmbeddingModel, dimensions: 2560}).EmbedStrings(context.Background(), []string{"query"}); governance.Code(err) != "unavailable" {
		t.Fatalf("expected a stable error for wrong vector dimensions: %v", err)
	}
}

func TestDashScopeEmbedderReportsAuthenticationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	embedder := dashScopeEmbedder{baseURL: server.URL, model: governance.BailianEmbeddingModel, dimensions: 2560}
	if _, err := embedder.EmbedStrings(t.Context(), []string{"query"}); governance.Code(err) != "knowledge_embedding_authentication_failed" {
		t.Fatalf("expected authentication failure: %v", err)
	}
}

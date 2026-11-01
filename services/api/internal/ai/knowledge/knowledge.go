// Package knowledge is the retrieval layer behind the knowledge capability and
// the grounded answering workflow: chunking, storage and tenant scoped search.
package knowledge

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const (
	chunkRunes   = 600
	chunkOverlap = 80
	maxChunks    = 200
	// Limit is the default number of fragments a query returns.
	Limit = 6
)

// Split breaks text into overlapping rune windows. Overlap keeps a sentence that
// straddles a boundary retrievable from either side.
func Split(text string, size, overlap int) []string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return nil
	}
	runes := []rune(clean)
	if size <= 0 || size > 4000 {
		size = chunkRunes
	}
	if overlap < 0 || overlap >= size {
		overlap = chunkOverlap
	}
	out := []string{}
	for start := 0; start < len(runes) && len(out) < maxChunks; {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[start:end])))
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return out
}

// Store owns the knowledge tables and the vector stack. Retrieval stays tenant
// scoped: the tenant is read from the authenticated principal on every call,
// never from the request.
type Store struct {
	Q    *dbgen.Queries
	Core *governance.Core

	vectorOnce  sync.Once
	vectorCache *vectorCache
}

// Index stores one document and its fragments. Chunking stays ours because the
// window size is a product decision; embedding and writing go through the Eino
// indexer, so the vector layout matches what the retriever expects.
func (s *Store) Index(ctx context.Context, p *identity.Principal, title, source string, content string) (int64, error) {
	chunks := Split(content, chunkRunes, chunkOverlap)
	if len(chunks) == 0 {
		return 0, governance.Problem("invalid")
	}
	if source != "manual" && source != "file" {
		source = "manual"
	}
	vector, err := s.vectorFor(ctx, p)
	if err != nil {
		return 0, err
	}
	id, err := s.Q.KBCreateDocument(ctx, dbgen.KBCreateDocumentParams{
		TenantID: p.TenantID, Title: title, Source: source,
		CreatedBy: p.Account.ID, Chunks: int32(len(chunks)), Content: content,
	})
	if err != nil {
		return 0, err
	}
	docs := make([]*schema.Document, 0, len(chunks))
	for ordinal, chunk := range chunks {
		docs = append(docs, &schema.Document{
			ID:      fmt.Sprintf("%s-%d-%d", p.TenantID, id, ordinal),
			Content: chunk,
			MetaData: map[string]any{
				"tenant": p.TenantID, "documentId": id, "title": title, "ordinal": ordinal,
			},
		})
	}
	if err = vector.storeDocuments(ctx, docs); err != nil {
		return 0, err
	}
	return id, nil
}

// Search returns the fragments the vector store considers closest to the query.
// Scoring, thresholding and ranking belong to the Eino retriever; this method only
// narrows the search to the caller's workspace and reshapes the documents.
func (s *Store) Search(ctx context.Context, p *identity.Principal, query string, limit int32) ([]*schema.Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, governance.Problem("invalid")
	}
	if limit <= 0 || limit > 20 {
		limit = Limit
	}
	vector, err := s.vectorFor(ctx, p)
	if err != nil {
		return nil, err
	}
	return vector.retrieve(ctx, p.TenantID, query, int(limit))
}

// Retriever adapts the store to the Eino retriever component, so the same
// implementation can back a graph node and a capability executor.
type Retriever struct{ Store *Store }

// Retrieve resolves the tenant from the trusted principal in the context.
func (r *Retriever) Retrieve(ctx context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, governance.Problem("unauthenticated")
	}
	return r.Store.Search(ctx, p, query, Limit)
}

// Delete removes a document and the fragments derived from it. Vectors live in
// the vector store, so they are dropped explicitly instead of by cascade.
func (s *Store) Delete(ctx context.Context, p *identity.Principal, id int64) error {
	if s == nil || s.Q == nil {
		return governance.Problem("unavailable")
	}
	if vector, err := s.vectorFor(ctx, p); err == nil {
		if err = vector.dropDocument(ctx, p.TenantID, id); err != nil {
			return err
		}
	}
	return s.Q.KBDeleteDocument(ctx, dbgen.KBDeleteDocumentParams{TenantID: p.TenantID, ID: id})
}

// Reindex rebuilds every vector of the workspace from the stored source text. It
// exists because changing the embedding model or the fragment window invalidates
// everything: re-uploading the corpus by hand is not a migration path.
func (s *Store) Reindex(ctx context.Context, p *identity.Principal) (int, error) {
	if s == nil || s.Q == nil {
		return 0, governance.Problem("unavailable")
	}
	vector, err := s.vectorFor(ctx, p)
	if err != nil {
		return 0, err
	}
	rows, err := s.Q.KBAllDocuments(ctx, p.TenantID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if err = vector.dropDocument(ctx, p.TenantID, row.ID); err != nil {
			return count, err
		}
		chunks := Split(row.Content, chunkRunes, chunkOverlap)
		if len(chunks) == 0 {
			continue
		}
		docs := make([]*schema.Document, 0, len(chunks))
		for ordinal, chunk := range chunks {
			docs = append(docs, &schema.Document{
				ID:      fmt.Sprintf("%s-%d-%d", p.TenantID, row.ID, ordinal),
				Content: chunk,
				MetaData: map[string]any{
					"tenant": p.TenantID, "documentId": row.ID, "title": row.Title, "ordinal": ordinal,
				},
			})
		}
		if err = vector.storeDocuments(ctx, docs); err != nil {
			return count, err
		}
		if err = s.Q.KBUpdateChunks(ctx, dbgen.KBUpdateChunksParams{
			TenantID: p.TenantID, ID: row.ID, Chunks: int32(len(chunks)),
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// Documents lists indexed documents for the tenant.
func (s *Store) Documents(ctx context.Context, tenant string) ([]dbgen.KBListDocumentsRow, error) {
	return s.Q.KBListDocuments(ctx, tenant)
}

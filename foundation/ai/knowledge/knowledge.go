// Package knowledge is the retrieval layer behind the knowledge capability and
// the grounded answering workflow: chunking, storage and tenant scoped search.
package knowledge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
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
	defer vector.Client.Close()
	id, err := s.Q.KBCreateDocument(ctx, dbgen.KBCreateDocumentParams{
		TenantID: p.TenantID, Title: title, Source: source,
		CreatedBy: p.Account.ID, Chunks: int32(len(chunks)), Content: content,
	})
	if err != nil {
		return 0, err
	}
	if err = vector.storeDocuments(ctx, documentFragments(p.TenantID, id, title, "", chunks)); err != nil {
		return 0, err
	}
	return id, nil
}

func documentFragments(tenant string, id int64, title, version string, chunks []string) []*schema.Document {
	prefix := fmt.Sprintf("%s-%d-", tenant, id)
	if version != "" {
		prefix += version + "-"
	}
	docs := make([]*schema.Document, 0, len(chunks))
	for ordinal, chunk := range chunks {
		docs = append(docs, &schema.Document{
			ID:      prefix + strconv.Itoa(ordinal),
			Content: chunk,
			MetaData: map[string]any{
				"tenant": tenant, "documentId": id, "title": title, "version": version,
			},
		})
	}
	return docs
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
	defer vector.Client.Close()
	result, stale, err := s.searchCurrent(ctx, vector, p.TenantID, query, int(limit))
	if err != nil || !stale || len(result) >= int(limit) {
		return result, err
	}
	// A replaced generation may still be in Redis while the outbox is relaying
	// its cleanup event. Look past those fragments only when necessary.
	result, _, err = s.searchCurrent(ctx, vector, p.TenantID, query, int(limit)+maxChunks)
	if len(result) > int(limit) {
		result = result[:limit]
	}
	return result, err
}

func (s *Store) searchCurrent(ctx context.Context, vector *Vector, tenant, query string, limit int) ([]*schema.Document, bool, error) {
	docs, err := vector.retrieve(ctx, tenant, query, limit)
	if err != nil || len(docs) == 0 {
		return docs, false, err
	}
	// PostgreSQL is authoritative for the visible document and its current
	// generation, including legacy documents whose generation is empty.
	docIDs := make([]int64, len(docs))
	for i, doc := range docs {
		id, _ := doc.MetaData["document_id"].(string)
		docIDs[i], _ = strconv.ParseInt(id, 10, 64)
	}
	versions, err := s.Q.KBCurrentDocumentVersions(ctx, dbgen.KBCurrentDocumentVersionsParams{TenantID: tenant, DocumentIds: docIDs})
	if err != nil {
		return nil, false, err
	}
	live := make(map[int64]string, len(versions))
	for _, document := range versions {
		live[document.ID] = document.Version
	}
	result := make([]*schema.Document, 0, len(docs))
	for i, doc := range docs {
		version, _ := doc.MetaData["document_version"].(string)
		if current, exists := live[docIDs[i]]; exists && current == version {
			result = append(result, doc)
		}
	}
	return result, len(docs) == limit && len(result) < len(docs), nil
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
	// Deleting the source first serializes with an edit and makes every
	// generation invisible even if vector cleanup runs later or fails.
	if err := s.Q.KBDeleteDocument(ctx, dbgen.KBDeleteDocumentParams{TenantID: p.TenantID, ID: id}); err != nil {
		return err
	}
	if vector, err := s.vectorFor(ctx, p); err == nil {
		defer vector.Client.Close()
		for _, prefix := range []string{hashPrefix, bailianHashPrefix, customHashPrefix + "*:"} {
			vector.keyPrefix = prefix
			if err = vector.dropDocument(ctx, p.TenantID, id); err != nil {
				slog.Warn("knowledge vector cleanup failed", "document_id", id)
				break
			}
		}
	}
	return nil
}

// Reindex rebuilds every vector of the workspace from the stored source text. It
// exists because changing the embedding model or the fragment window invalidates
// everything: re-uploading the corpus by hand is not a migration path.
func (s *Store) Reindex(ctx context.Context, p *identity.Principal) (int, error) {
	if s == nil || s.Q == nil || s.Core == nil || s.Core.Pool == nil {
		return 0, governance.Problem("unavailable")
	}
	vector, err := s.vectorFor(ctx, p)
	if err != nil {
		return 0, err
	}
	defer vector.Client.Close()
	ids, err := s.Q.KBAllDocumentIDs(ctx, p.TenantID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, id := range ids {
		indexed := false
		err := pgx.BeginFunc(ctx, s.Core.Pool, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			row, err := q.KBLockDocument(ctx, dbgen.KBLockDocumentParams{TenantID: p.TenantID, ID: id})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			chunks := Split(row.Content, chunkRunes, chunkOverlap)
			if len(chunks) == 0 {
				return nil
			}
			if err := vector.storeDocuments(ctx, documentFragments(p.TenantID, id, row.Title, row.Version, chunks)); err != nil {
				return err
			}
			if err := vector.cleanDocumentVersions(ctx, p.TenantID, id, func(version string, ordinal int) bool {
				return version != row.Version || ordinal >= len(chunks)
			}); err != nil {
				return err
			}
			indexed = true
			return q.KBUpdateChunks(ctx, dbgen.KBUpdateChunksParams{TenantID: p.TenantID, ID: id, Chunks: int32(len(chunks))})
		})
		if err != nil {
			return count, err
		}
		if indexed {
			count++
		}
	}
	return count, nil
}

// Documents lists indexed documents for the tenant.
func (s *Store) Documents(ctx context.Context, tenant string) ([]dbgen.KBListDocumentsRow, error) {
	return s.Q.KBListDocuments(ctx, tenant)
}

package knowledge

import (
	"context"
	"strings"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
)

// Service is the authorized facade over the knowledge store. Retrieval itself is
// exposed as a capability; document management stays an administrative action.
type Service struct{ Store *Store }

// Document is the management projection. The JSON names are the contract the
// console reads, so they stay stable even if the row shape changes.
type Document struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Chunks    int64  `json:"chunks"`
	CreatedAt string `json:"createdAt"`
}

// Ingest stores one document. It requires knowledge:create because whatever is
// written here becomes material every grounded answer may cite.
func (s *Service) Ingest(ctx context.Context, title, content string) (int64, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return 0, governance.Problem("unauthenticated")
	}
	if !p.Can("knowledge:create") {
		return 0, governance.Problem("forbidden")
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || len([]rune(title)) > 200 || content == "" {
		return 0, governance.Problem("invalid")
	}
	if s == nil || s.Store == nil || s.Store.Q == nil {
		return 0, governance.Problem("unavailable")
	}
	return s.Store.Index(ctx, p, title, "manual", content)
}

// Documents lists indexed documents for the tenant.
func (s *Service) Documents(ctx context.Context) ([]Document, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, governance.Problem("unauthenticated")
	}
	if !p.Can("knowledge:read") {
		return nil, governance.Problem("forbidden")
	}
	if s == nil || s.Store == nil || s.Store.Q == nil {
		return nil, governance.Problem("unavailable")
	}
	rows, err := s.Store.Documents(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Document, 0, len(rows))
	for _, row := range rows {
		out = append(out, Document{ID: row.ID, Title: row.Title, Source: row.Source, Chunks: row.Chunks, CreatedAt: governance.Time(row.CreatedAt)})
	}
	return out, nil
}

// Delete removes a document and, by cascade, every fragment derived from it.
func (s *Service) Delete(ctx context.Context, id int64) error {
	p, err := identity.Current(ctx)
	if err != nil {
		return governance.Problem("unauthenticated")
	}
	if !p.Can("knowledge:delete") {
		return governance.Problem("forbidden")
	}
	if id < 1 {
		return governance.Problem("invalid")
	}
	if s == nil || s.Store == nil || s.Store.Q == nil {
		return governance.Problem("unavailable")
	}
	return s.Store.Delete(ctx, p, id)
}

// Reindex rebuilds every vector in the workspace. It needs the same grant as
// writing a document: it changes what retrieval can return.
func (s *Service) Reindex(ctx context.Context) (int, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return 0, governance.Problem("unauthenticated")
	}
	if !p.Can("knowledge:create") {
		return 0, governance.Problem("forbidden")
	}
	if s == nil || s.Store == nil {
		return 0, governance.Problem("unavailable")
	}
	return s.Store.Reindex(ctx, p)
}

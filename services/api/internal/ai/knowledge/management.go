package knowledge

import (
	"context"
	"fmt"
	"time"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types of the knowledge base. They are facts other modules may react to:
// a re-index can invalidate a cached answer, a deletion belongs in the audit
// trail next to the answers that cited the document.
const (
	DocumentIndexed = "knowledge.document_indexed"
	DocumentDeleted = "knowledge.document_deleted"
)

// Entries are the management capabilities. They share the store with retrieval,
// so a document written from the console is searchable by the agent right away,
// and anything the agent cited can be inspected and removed here.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: listDefinition(), Handler: contracts.Handler{Execute: s.list}},
		{Definition: createDefinition(), Handler: contracts.Handler{Execute: s.create}},
		{Definition: deleteDefinition(), Handler: contracts.Handler{Execute: s.remove}},
		{Definition: reindexDefinition(), Handler: contracts.Handler{Execute: s.reindex}},
	}
}

func listDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.list", Version: "1.0.0", Title: "List knowledge documents",
		Description: "List the workspace knowledge documents with their fragment counts, newest first.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: "knowledge:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:knowledge.list", Tags: []string{"knowledge", "read"},
		Input:    contracts.Object(map[string]*contracts.Schema{}),
		Loggable: []string{},
	}
}

func createDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.create", Version: "1.0.0", Title: "Index a knowledge document",
		Description: "Split a pasted document into overlapping fragments and index it. The agent can cite it immediately.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: "knowledge:create",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+title"},
		SuccessEvents: []string{DocumentIndexed},
		Executor:      "go:knowledge.create", Tags: []string{"knowledge", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"title":   contracts.Str(1, 200),
			"content": contracts.Str(1, 100000),
		}, "title", "content"),
		Loggable: []string{"title"},
	}
}

func deleteDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.delete", Version: "1.0.0", Title: "Delete a knowledge document",
		Description: "Remove a document and every fragment derived from it. Answers stop citing it.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: "knowledge:delete",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+document"},
		SuccessEvents: []string{DocumentDeleted},
		Executor:      "go:knowledge.delete", Tags: []string{"knowledge", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id": contracts.Int(1, 1<<40),
		}, "id"),
		Loggable: []string{"id"},
	}
}

func reindexDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.reindex", Version: "1.0.0", Title: "Rebuild knowledge vectors",
		Description: "Re-embed every document from its stored text. Run it after changing the embedding model or the fragment window.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: "knowledge:create",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+reindex"},
		SuccessEvents: []string{DocumentIndexed},
		Executor:      "go:knowledge.reindex", Tags: []string{"knowledge", "write"},
		Input:    contracts.Object(map[string]*contracts.Schema{}),
		Loggable: []string{},
	}
}

func (s *Service) reindex(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	documents, err := s.Reindex(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"documents": documents},
		Summary: fmt.Sprintf("documents=%d reindexed", documents),
		Events: []contracts.Event{{
			Type: DocumentIndexed, AggregateType: "knowledge_document",
			AggregateID: "all", Payload: map[string]any{"documents": documents},
		}},
	}, nil
}

func (s *Service) list(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	documents, err := s.Documents(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    documents,
		Summary: fmt.Sprintf("documents=%d", len(documents)),
	}, nil
}

func (s *Service) create(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	title, _ := in["title"].(string)
	content, _ := in["content"].(string)
	id, err := s.Ingest(ctx, title, content)
	if err != nil {
		return contracts.Output{}, err
	}
	fragments := len(Split(content, chunkRunes, chunkOverlap))
	return contracts.Output{
		Data: Document{
			ID: id, Title: title, Source: "manual", Chunks: int64(fragments),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Summary: fmt.Sprintf("document=%d chunks=%d", id, fragments),
		Events: []contracts.Event{{
			Type: DocumentIndexed, AggregateType: "knowledge_document",
			AggregateID: fmt.Sprintf("%d", id),
			Payload:     map[string]any{"title": title, "chunks": fragments},
		}},
	}, nil
}

func (s *Service) remove(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	value, ok := in["id"].(float64)
	if !ok || value < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	id := int64(value)
	if err := s.Delete(ctx, id); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("document=%d deleted", id),
		Events: []contracts.Event{{
			Type: DocumentDeleted, AggregateType: "knowledge_document",
			AggregateID: fmt.Sprintf("%d", id),
		}},
	}, nil
}

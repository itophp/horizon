package knowledge

import (
	"context"
	"fmt"

	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

// Chunker is the Eino document transformer for the knowledge base. A graph node
// and a console upload run the same window, so fragments are identical however
// the text arrived.
type Chunker struct{}

func (Chunker) Transform(_ context.Context, src []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	out := make([]*schema.Document, 0, len(src))
	for _, item := range src {
		for index, fragment := range Split(item.Content, chunkRunes, chunkOverlap) {
			meta := make(map[string]any, len(item.MetaData)+1)
			for key, value := range item.MetaData {
				meta[key] = value
			}
			meta["ordinal"] = index
			out = append(out, &schema.Document{ID: item.ID, Content: fragment, MetaData: meta})
		}
	}
	return out, nil
}

// Indexer adapts the store to the Eino indexer component. The tenant always comes
// from the trusted principal, never from document metadata: a graph cannot widen
// the scope of a write by setting a field.
type Indexer struct{ Inner *Store }

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, _ ...indexer.Option) ([]string, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, contracts.Problem("unauthenticated")
	}
	if i == nil || i.Inner == nil || i.Inner.Q == nil {
		return nil, contracts.Problem("unavailable")
	}
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		title, _ := doc.MetaData["title"].(string)
		if title == "" {
			title = doc.ID
		}
		if title == "" {
			return nil, contracts.Problem("invalid")
		}
		id, err := i.Inner.Index(ctx, p, title, "manual", doc.Content)
		if err != nil {
			return nil, err
		}
		ids = append(ids, fmt.Sprintf("%d", id))
	}
	return ids, nil
}

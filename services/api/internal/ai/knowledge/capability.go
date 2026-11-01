package knowledge

import (
	"context"
	"fmt"

	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Search is the capability form of knowledge retrieval. The agent uses it as a
// tool, the console can call it as a command, and both hit the same tenant
// scoped store.
func (s *Store) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: searchDefinition(), Handler: contracts.Handler{Execute: s.search}},
	}
}

func searchDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.search", Version: "1.0.0", Title: "Knowledge search",
		Description: "Search the workspace knowledge base for internal rules, configuration and uploaded documents. Returns titled fragments.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: "assistant:use",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:knowledge.search", Tags: []string{"knowledge", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"query": contracts.Str(1, 200),
			"limit": contracts.Int(1, 10),
		}, "query"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"results": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"title": {Type: "string"}, "content": {Type: "string"},
			}, "title", "content")},
		}),
		Loggable: []string{"query", "limit"},
	}
}

func (s *Store) search(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("assistant:use") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if s == nil || s.Q == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	query, _ := in["query"].(string)
	if query == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	limit := int32(6)
	if value, ok := in["limit"].(float64); ok && value >= 1 && value <= 10 {
		limit = int32(value)
	}
	documents, err := s.Search(ctx, p, query, limit)
	if err != nil {
		return contracts.Output{}, err
	}
	results := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		title, _ := document.MetaData["title"].(string)
		results = append(results, map[string]any{"title": title, "content": truncate(document.Content, 1200)})
	}
	return contracts.Output{Data: map[string]any{"results": results}, Summary: fmt.Sprintf("results=%d", len(results))}, nil
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

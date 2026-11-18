package knowledge

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

const maxDocumentID = 1<<53 - 1
const agentDocumentPage = 1500

func documentSchema(includeContent bool) *contracts.Schema {
	fields := map[string]*contracts.Schema{
		"id": contracts.Int(1, maxDocumentID), "title": contracts.Text(),
		"source": contracts.Enum("manual", "file"), "createdAt": contracts.Text(),
		"chunks": contracts.Int(0, 1<<31-1),
	}
	required := []string{"id", "title", "source", "createdAt", "chunks"}
	if includeContent {
		fields["content"] = contracts.Text()
		fields["version"] = contracts.Str(0, 32)
		fields["nextOffset"] = contracts.Int(1, 100000)
		required = append(required, "content")
	}
	return contracts.Object(fields, required...)
}

func documentReadDefinition(name, title, description string, input, output *contracts.Schema) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: "1.0.0", Title: title, Description: description,
		Domain: "knowledge", Entity: "knowledge_document", Permission: "knowledge:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:" + name, Tags: []string{"knowledge", "read", "agent"},
		AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI, contracts.OriginAPI, contracts.OriginAgent},
		Input:          input, Output: output, Loggable: []string{},
	}
}

func queryDefinition() contracts.Definition {
	return documentReadDefinition("knowledge.query", "Query knowledge documents",
		"Find workspace documents by a literal keyword in their title or stored text. Returns document identifiers and metadata. Agent calls return at most eight results per page. Use nextBeforeId to continue.",
		contracts.Object(map[string]*contracts.Schema{
			"keyword": contracts.Str(0, 200), "beforeId": contracts.Int(1, maxDocumentID), "limit": contracts.Int(1, 100),
		}),
		contracts.Object(map[string]*contracts.Schema{
			"documents": {Type: "array", Items: documentSchema(false)}, "nextBeforeId": contracts.Int(1, maxDocumentID),
		}, "documents"))
}

func readDefinition() contracts.Definition {
	return documentReadDefinition("knowledge.read", "Read a knowledge document",
		"Read a workspace document and its current version. Agent calls return the text in pages: use nextOffset as offset for the next page. Use version as expectedVersion when submitting an edit.",
		contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, maxDocumentID), "offset": contracts.Int(0, 100000)}, "id"),
		documentSchema(true))
}

func (s *Service) readPrincipal(ctx context.Context) (*identity.Principal, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, contracts.Problem("unauthenticated")
	}
	if !p.Can("knowledge:read") {
		return nil, contracts.Problem("forbidden")
	}
	if s == nil || s.Store == nil || s.Store.Q == nil {
		return nil, contracts.Problem("unavailable")
	}
	return p, nil
}

func (s *Service) query(ctx context.Context, input contracts.Input) (contracts.Output, error) {
	p, err := s.readPrincipal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	request := struct {
		Keyword  string `json:"keyword"`
		BeforeID int64  `json:"beforeId"`
		Limit    int32  `json:"limit"`
	}{Limit: 25}
	if err := contracts.Bind(input, &request); err != nil {
		return contracts.Output{}, err
	}
	if call, _ := gateway.CurrentInvocation(ctx); call.Origin == contracts.OriginAgent {
		request.Limit = min(request.Limit, 8)
	}
	rows, err := s.Store.Q.KBQueryDocuments(ctx, dbgen.KBQueryDocumentsParams{
		TenantID: p.TenantID, BeforeID: request.BeforeID, Keyword: strings.TrimSpace(request.Keyword), RowLimit: request.Limit + 1,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	result := map[string]any{}
	if len(rows) > int(request.Limit) {
		rows = rows[:request.Limit]
		result["nextBeforeId"] = rows[len(rows)-1].ID
	}
	documents := make([]Document, 0, len(rows))
	for _, row := range rows {
		documents = append(documents, Document{ID: row.ID, Title: row.Title, Source: row.Source, Chunks: row.Chunks, CreatedAt: governance.Time(row.CreatedAt)})
	}
	result["documents"] = documents
	return contracts.Output{Data: result}, nil
}

func (s *Service) read(ctx context.Context, input contracts.Input) (contracts.Output, error) {
	p, err := s.readPrincipal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	var request struct {
		ID     int64 `json:"id"`
		Offset *int  `json:"offset"`
	}
	if err := contracts.Bind(input, &request); err != nil {
		return contracts.Output{}, err
	}
	row, err := s.Store.Q.KBReadDocument(ctx, dbgen.KBReadDocumentParams{TenantID: p.TenantID, ID: request.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	result := struct {
		Document
		Content    string `json:"content"`
		Version    string `json:"version"`
		NextOffset int    `json:"nextOffset,omitempty"`
	}{
		Document: Document{ID: row.ID, Title: row.Title, Source: row.Source, Chunks: row.Chunks, CreatedAt: governance.Time(row.CreatedAt)},
		Content:  row.Content,
		Version:  row.Version,
	}
	if call, _ := gateway.CurrentInvocation(ctx); call.Origin == contracts.OriginAgent || request.Offset != nil {
		runes := []rune(row.Content)
		offset := 0
		if request.Offset != nil {
			offset = min(*request.Offset, len(runes))
		}
		end := min(offset+agentDocumentPage, len(runes))
		result.Content = string(runes[offset:end])
		if end < len(runes) {
			result.NextOffset = end
		}
	}
	return contracts.Output{Data: result}, nil
}

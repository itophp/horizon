package knowledge

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

const (
	DocumentUpdated  = "knowledge.document_updated"
	updatePermission = "knowledge:update"
)

func updateDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "knowledge.update", Version: "1.0.0", Title: "Update a knowledge document",
		Description: "Update a knowledge document identified through knowledge_query. Read its current version with knowledge_read first. For a section edit provide replacement with exact text that occurs once and its new text. For a full rewrite provide content instead. Omit title to keep it. AI requests require human confirmation before writing.",
		Domain:      "knowledge", Entity: "knowledge_document", Permission: updatePermission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalConfirm,
		Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+document"},
		Executor:    "go:knowledge.update", Tags: []string{"knowledge", "write", "agent", contracts.TagSealedApprovalInput},
		AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI, contracts.OriginAPI, contracts.OriginAgent, contracts.OriginApproval},
		InputAccess:    []contracts.FieldAccess{{Path: "id", Permission: "knowledge:read"}},
		SuccessEvents:  []string{DocumentUpdated},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id": contracts.Int(1, maxDocumentID), "expectedVersion": contracts.Str(0, 32),
			"title": contracts.Str(1, 200), "content": contracts.Str(1, 100000),
			"replacement": contracts.Object(map[string]*contracts.Schema{
				"find": contracts.Str(1, 100000), "text": contracts.Str(0, 100000),
			}, "find", "text"),
		}, "id", "expectedVersion"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": contracts.Int(1, maxDocumentID),
		}, "id"),
		Loggable: []string{"id"},
	}
}

// CleanUpdatedDocument removes only the old generation after the outbox
// delivers the committed update. Replayed and out-of-order events are safe.
func (s *Store) CleanUpdatedDocument(ctx context.Context, event eventhub.Event) {
	if event.Type != DocumentUpdated || event.AggregateType != "knowledge_document" {
		return
	}
	id, err := strconv.ParseInt(event.AggregateID, 10, 64)
	previous, ok := event.Payload["previousVersion"].(string)
	if err != nil || id < 1 || !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	vector, err := s.vectorFor(ctx, &identity.Principal{TenantID: event.TenantID})
	if err == nil {
		defer vector.Client.Close()
		err = vector.cleanDocumentVersions(ctx, event.TenantID, id, func(version string, _ int) bool { return version == previous })
	}
	if err != nil {
		slog.Warn("knowledge vector cleanup failed", "event_id", event.EventID)
	}
}

func (s *Service) update(ctx context.Context, tx pgx.Tx, input contracts.Input) (contracts.Output, error) {
	p, err := s.readPrincipal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if !p.CanPermanent(updatePermission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	var request struct {
		ID              int64   `json:"id"`
		ExpectedVersion string  `json:"expectedVersion"`
		Title           *string `json:"title"`
		Content         *string `json:"content"`
		Replacement     *struct {
			Find string `json:"find"`
			Text string `json:"text"`
		} `json:"replacement"`
	}
	if err := contracts.Bind(input, &request); err != nil {
		return contracts.Output{}, err
	}
	if request.Content != nil && request.Replacement != nil {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	row, err := q.KBLockDocument(ctx, dbgen.KBLockDocumentParams{TenantID: p.TenantID, ID: request.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	if row.Version != request.ExpectedVersion {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	title, content := row.Title, row.Content
	if request.Title != nil {
		title = strings.TrimSpace(*request.Title)
	}
	if request.Content != nil {
		content = strings.TrimSpace(*request.Content)
	}
	if change := request.Replacement; change != nil {
		start := strings.Index(content, change.Find)
		if start < 0 || strings.Contains(content[start+1:], change.Find) {
			return contracts.Output{}, contracts.Problem("knowledge_replacement_not_unique")
		}
		content = content[:start] + change.Text + content[start+len(change.Find):]
	}
	if title == "" || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > 100000 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if row.Title == title && row.Content == content {
		return contracts.Output{}, contracts.Problem("no_changes")
	}
	chunks := Split(content, chunkRunes, chunkOverlap)
	vector, err := s.Store.vectorFor(ctx, p)
	if err != nil {
		return contracts.Output{}, err
	}
	defer vector.Client.Close()

	// The new keys cannot be served while PostgreSQL still points to the old
	// generation. The Gateway commits the version and its event in one transaction.
	version := rand.Text()
	docs := documentFragments(p.TenantID, request.ID, title, version, chunks)
	if err := vector.storeDocuments(ctx, docs); err != nil {
		vector.discardFragments(ctx, docs)
		return contracts.Output{}, err
	}
	if err := q.KBUpdateDocument(ctx, dbgen.KBUpdateDocumentParams{
		TenantID: p.TenantID, ID: request.ID, Title: title, Content: content, Chunks: int32(len(chunks)), Version: version,
	}); err != nil {
		vector.discardFragments(ctx, docs)
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data: map[string]any{"id": request.ID},
		Events: []contracts.Event{{
			Type: DocumentUpdated, AggregateType: "knowledge_document", AggregateID: strconv.FormatInt(request.ID, 10),
			Payload: map[string]any{"previousVersion": row.Version},
		}},
	}, nil
}

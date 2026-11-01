package dictionaries

import (
	"context"
	"fmt"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types published by the dictionaries module.
const (
	DictionaryDraftCreated = "dictionary.draft_created"
	DictionaryPublished    = "dictionary.published"
	DictionaryRolledBack   = "dictionary.rolled_back"
	DictionaryDeleted      = "dictionary.deleted"
	DictionaryOverridden   = "dictionary.overridden"
	DictionaryImported     = "dictionary.imported"
)

const version = "1.0.0"

// Entries is the capability catalog contributed by the dictionaries module. The
// service still owns permission checks, validation and the transaction; the
// executors only decode arguments and delegate.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: s.listDefinition(), Handler: contracts.Handler{Execute: s.entryList}},
		{Definition: s.detailDefinition(), Handler: contracts.Handler{Execute: s.entryDetail}},
		{Definition: s.optionsDefinition(), Handler: contracts.Handler{Execute: s.entryOptions}},
		{Definition: s.draftDefinition(), Handler: contracts.Handler{Execute: s.entryDraft}},
		{Definition: s.publishDefinition(), Handler: contracts.Handler{Execute: s.entryPublish}},
		{Definition: s.rollbackDefinition(), Handler: contracts.Handler{Execute: s.entryRollback}},
		{Definition: s.deleteDefinition(), Handler: contracts.Handler{Execute: s.entryDelete}},
		{Definition: s.overrideDefinition(), Handler: contracts.Handler{Execute: s.entryOverride}},
		{Definition: s.importPreviewDefinition(), Handler: contracts.Handler{Execute: s.entryImportPreview}},
		{Definition: s.importDraftDefinition(), Handler: contracts.Handler{Execute: s.entryImportDraft}},
		{Definition: s.exportDefinition(), Handler: contracts.Handler{Execute: s.entryExport}},
	}
}

// scopeFields are the optional scope selectors shared by list and detail.
// Dictionary governance is global, so the only accepted target is the workspace.
func scopeFields() map[string]*contracts.Schema {
	return map[string]*contracts.Schema{
		"targetKind": contracts.Enum("workspace"),
		"targetId":   contracts.Int(0, 0),
	}
}

func targetSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"kind": contracts.Enum("workspace"),
		"id":   contracts.Int(0, 0),
	}, "kind", "id")
}

func formatSchema() *contracts.Schema { return contracts.Enum("csv", "json") }

// freeObject accepts a reviewed payload whose detailed shape the module
// validates itself; the description tells the UI and the agent what it holds.
func freeObject(description string) *contracts.Schema {
	schema := contracts.FreeObject()
	schema.Description = description
	return schema
}

// documentSchema accepts the reviewed dictionary document; the service walks the
// item tree, codes and revisions itself.
func documentSchema() *contracts.Schema {
	return freeObject("Dictionary document: definition plus the full item list, validated by the module.")
}

func (s *Service) listDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.list.read", Version: version, Title: "List dictionaries",
		Description: "List the dictionary definitions visible in one scope.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:dictionary.list", Tags: []string{"dictionaries", "read", "agent"},
		Input: contracts.Object(scopeFields()),
		Output: contracts.Object(map[string]*contracts.Schema{
			"definitions": {Type: "array", Items: contracts.FreeObject()},
		}),
		Loggable: []string{"targetKind", "targetId"},
	}
}

func (s *Service) detailDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.detail.read", Version: version, Title: "Read a dictionary",
		Description: "Return one dictionary document with its drafts, history and scoped overrides.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:dictionary.detail", Tags: []string{"dictionaries", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":         contracts.Int(1, 1<<40),
			"targetKind": contracts.Enum("workspace"),
			"targetId":   contracts.Int(0, 0),
		}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"document": contracts.FreeObject(),
			"revision": {Type: "integer"},
		}),
		Loggable: []string{"id", "targetKind", "targetId"},
	}
}

func (s *Service) optionsDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.options.read", Version: version, Title: "Read dictionary options",
		Description: "Read the published options of one business dictionary, for example a status or category list.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:use",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:dictionary.options", Tags: []string{"dictionaries", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"code":     contracts.Str(1, 100),
			"language": contracts.Enum("en", "zh-CN"),
			"ids":      contracts.Array(contracts.Int(1, 1<<40), 0, 200),
			"search":   contracts.Str(0, 100),
		}, "code"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"options": {Type: "array", Items: contracts.FreeObject()},
		}),
		Loggable: []string{"code", "language", "search"},
	}
}

func (s *Service) draftDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.draft.create", Version: version, Title: "Create a dictionary draft",
		Description: "Stage a full dictionary document against a revision. Nothing takes effect until publish.",
		Domain:      "dictionaries", Entity: "dictionary_draft", Permission: "dictionaries:update",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryDraftCreated}, Executor: "go:dictionary.draft",
		Tags: []string{"dictionaries", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"document":     documentSchema(),
			"baseRevision": contracts.Int(0, 1<<40),
			"reason":       contracts.Str(5, 500),
		}, "document", "baseRevision", "reason"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "status": {Type: "string"},
		}),
		Loggable: []string{"baseRevision", "reason"},
	}
}

func (s *Service) publishDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.draft.publish", Version: version, Title: "Publish a dictionary draft",
		Description: "Apply a reviewed draft. Requires a short lived reauthentication proof.",
		Domain:      "dictionaries", Entity: "dictionary_version", Permission: "dictionaries:publish",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryPublished}, Executor: "go:dictionary.publish",
		Tags: []string{"dictionaries", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"baseRevision": contracts.Int(0, 1<<40),
			"proof":        contracts.Str(0, 200),
		}, "id", "baseRevision", "proof"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "revision": {Type: "integer"},
		}),
		Loggable: []string{"id", "baseRevision"},
	}
}

func (s *Service) rollbackDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.history.rollback", Version: version, Title: "Roll a dictionary back",
		Description: "Create a new dictionary version from a published version.",
		Domain:      "dictionaries", Entity: "dictionary_version", Permission: "dictionaries:rollback",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryRolledBack}, Executor: "go:dictionary.rollback",
		Tags: []string{"dictionaries", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"baseRevision": contracts.Int(0, 1<<40),
			"proof":        contracts.Str(0, 200),
			"reason":       contracts.Str(5, 500),
		}, "id", "baseRevision", "proof", "reason"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "revision": {Type: "integer"},
		}),
		Loggable: []string{"id", "baseRevision", "reason"},
	}
}

func (s *Service) deleteDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.delete", Version: version, Title: "Delete a dictionary",
		Description: "Delete a dictionary that was never published. Published dictionaries are archived through a draft.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:update",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryDeleted}, Executor: "go:dictionary.delete",
		Tags:     []string{"dictionaries", "write"},
		Input:    contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<40)}, "id"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}),
		Loggable: []string{"id"},
	}
}

func (s *Service) overrideDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.override.save", Version: version, Title: "Save dictionary overrides",
		Description: "Stage item overrides and extensions for one scope; publishing applies them as a new version.",
		Domain:      "dictionaries", Entity: "dictionary_draft", Permission: "dictionaries:update",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryOverridden}, Executor: "go:dictionary.override",
		Tags: []string{"dictionaries", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"target":       targetSchema(),
			"baseRevision": contracts.Int(0, 1<<40),
			"overrides":    contracts.Array(freeObject("Item override: itemId plus the fields it replaces, validated by the module."), 1, 500),
			"extensions":   contracts.Array(freeObject("Dictionary item owned by the target scope, validated by the module."), 0, 1000),
			"reason":       contracts.Str(0, 500),
		}, "id", "target", "overrides"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "revision": {Type: "integer"},
		}),
		Loggable: []string{"id", "baseRevision", "reason"},
	}
}

func (s *Service) importPreviewDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.import.preview", Version: version, Title: "Preview a dictionary import",
		Description: "Parse a CSV or JSON dictionary file and report the rows it would add or update without writing.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:import",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:dictionary.import.preview", Tags: []string{"dictionaries", "import"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":      contracts.Int(1, 1<<40),
			"format":  formatSchema(),
			"content": contracts.Str(0, 10<<20),
		}, "id", "format", "content"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"errors": {Type: "array", Items: &contracts.Schema{Type: "string"}},
			"added":  {Type: "integer"}, "updated": {Type: "integer"},
		}),
		Loggable: []string{"id", "format"},
	}
}

func (s *Service) importDraftDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.import.draft", Version: version, Title: "Import a dictionary draft",
		Description: "Stage the rows of a CSV or JSON dictionary file as a draft. Nothing takes effect until publish.",
		Domain:      "dictionaries", Entity: "dictionary_draft", Permission: "dictionaries:import",
		Entities: []string{"dictionary_draft", "dictionary", "file"},
		Risk:     contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{DictionaryImported}, Executor: "go:dictionary.import.draft",
		Tags: []string{"dictionaries", "import", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"format":       formatSchema(),
			"content":      contracts.Str(0, 10<<20),
			"baseRevision": contracts.Int(0, 1<<40),
			"reason":       contracts.Str(5, 500),
		}, "id", "format", "content", "baseRevision", "reason"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "status": {Type: "string"},
		}),
		Loggable: []string{"id", "format", "baseRevision", "reason"},
	}
}

func (s *Service) exportDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "dictionary.export.read", Version: version, Title: "Export a dictionary",
		Description: "Export one dictionary document as CSV or JSON.",
		Domain:      "dictionaries", Entity: "dictionary", Permission: "dictionaries:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:dictionary.export", Tags: []string{"dictionaries", "export"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":     contracts.Int(1, 1<<40),
			"format": formatSchema(),
		}, "id", "format"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"filename": {Type: "string"}, "content": {Type: "string"},
		}),
		Loggable: []string{"id", "format"},
	}
}

func (s *Service) entryList(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ListPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.TargetKind == "" {
		payload.TargetKind = "workspace"
	}
	out, err := s.List(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("definitions=%d", len(out.Definitions))}, nil
}

func (s *Service) entryDetail(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := GetPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Get(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	items := 0
	if out.Document != nil {
		items = len(out.Document.Items)
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("items=%d", items)}, nil
}

func (s *Service) entryOptions(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := OptionsPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Options(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("options=%d", len(out))}, nil
}

func (s *Service) entryDraft(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DraftPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Draft(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	dictionaryID := int64(0)
	if out.Document != nil && out.Document.Definition != nil {
		dictionaryID = out.Document.Definition.ID
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("draft=%d", out.ID), Events: []contracts.Event{{
		Type: DictionaryDraftCreated, AggregateType: "dictionary_draft", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"draftId": out.ID, "dictionaryId": dictionaryID, "baseRevision": payload.BaseRevision},
	}}}, nil
}

func (s *Service) entryPublish(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := PublishPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Publish(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("revision=%d", out.Revision), Events: []contracts.Event{{
		Type: DictionaryPublished, AggregateType: "dictionary_version", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"versionId": out.ID, "draftId": payload.ID, "revision": out.Revision},
	}}}, nil
}

func (s *Service) entryRollback(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RollbackPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Rollback(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("revision=%d", out.Revision), Events: []contracts.Event{{
		Type: DictionaryRolledBack, AggregateType: "dictionary_version", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"versionId": out.ID, "baseVersionId": payload.ID, "revision": out.Revision},
	}}}, nil
}

func (s *Service) entryDelete(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RemovePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.Remove(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("deleted=%d", payload.ID), Events: []contracts.Event{{
		Type: DictionaryDeleted, AggregateType: "dictionary", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"dictionaryId": payload.ID},
	}}}, nil
}

func (s *Service) entryOverride(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := OverridePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if _, err := s.Override(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("overrides=%d", len(payload.Overrides)), Events: []contracts.Event{{
		Type: DictionaryOverridden, AggregateType: "dictionary", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{
			"dictionaryId": payload.ID, "overrides": len(payload.Overrides),
			"extensions": len(payload.Extensions), "baseRevision": payload.BaseRevision,
		},
	}}}, nil
}

func (s *Service) entryImportPreview(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ImportPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Import(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("added=%d updated=%d errors=%d", out.Added, out.Updated, len(out.Errors)),
	}, nil
}

func (s *Service) entryImportDraft(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ImportDraftPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.ImportDraft(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("draft=%d", out.ID), Events: []contracts.Event{{
		Type: DictionaryImported, AggregateType: "dictionary_draft", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"draftId": out.ID, "dictionaryId": payload.ID, "format": payload.Format},
	}}}, nil
}

func (s *Service) entryExport(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ExportPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Export(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("format=%s", payload.Format)}, nil
}

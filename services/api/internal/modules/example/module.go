// Package example is the reference business module of the platform. It proves
// the contract end to end with three capabilities (read, create, state change),
// a transactional outbox event and an event-triggered AI review. It is
// deliberately small: a real MES/ERP module plugs into the same interfaces.
package example

import (
	"context"
	"errors"
	"fmt"

	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event types.
const (
	CreatedEvent  = "example.record.created"
	ArchivedEvent = "example.record.archived"
)

// Permissions used by the module.
const (
	ReadPermission    = "example:read"
	CreatePermission  = "example:create"
	ArchivePermission = "example:archive"
)

// Module is the example business module.
type Module struct{ pool *pgxpool.Pool }

// New builds the module.
func New(pool *pgxpool.Pool) *Module { return &Module{pool: pool} }

// Entries is the capability catalog contributed by the module.
func (m *Module) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: listDefinition(), Handler: contracts.Handler{Execute: m.list}},
		{Definition: createDefinition(), Handler: contracts.Handler{ExecuteTx: m.create}},
		{Definition: archiveDefinition(), Handler: contracts.Handler{ExecuteTx: m.archive}},
		{Definition: getDefinition(), Handler: contracts.Handler{Execute: m.get}},
	}
}

func text(min, max int) *contracts.Schema {
	low, high := min, max
	return &contracts.Schema{Type: "string", MinLength: &low, MaxLength: &high}
}

func listDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "example.record.list", Version: "1.0.0", Title: "List example records",
		Description: "List the example business records of the current tenant, newest first.",
		Domain:      "example", Entity: "example_record", Permission: ReadPermission,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		ReadOnly: true, Executor: "go:example.list", Tags: []string{"example", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"limit": {Type: "integer", Minimum: floatPointer(1), Maximum: floatPointer(100)},
		}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"records": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "title": {Type: "string"}, "status": {Type: "string"},
			}, "id", "title", "status")},
		}),
		Loggable: []string{"limit"},
	}
}

func createDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "example.record.create", Version: "1.0.0", Title: "Create example record",
		Description: "Create one example record in draft status. Emits example.record.created so AI review can run asynchronously.",
		Domain:      "example", Entity: "example_record", Permission: CreatePermission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{CreatedEvent}, Executor: "go:example.create", Tags: []string{"example", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"title": text(1, 200),
		}, "title"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "title": {Type: "string"}, "status": {Type: "string"},
		}, "id", "title", "status"),
		Loggable: []string{"title"},
	}
}

func archiveDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "example.record.archive", Version: "1.0.0", Title: "Archive example record",
		Description: "Archive one example record. High risk: an agent must obtain human confirmation before it runs.",
		Domain:      "example", Entity: "example_record", Permission: ArchivePermission,
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{ArchivedEvent}, Executor: "go:example.archive", Tags: []string{"example", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer", Minimum: floatPointer(1)},
		}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "title": {Type: "string"}, "status": {Type: "string"},
		}, "id", "title", "status"),
		Loggable: []string{"id"},
	}
}

func (m *Module) list(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can(ReadPermission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	limit := int32(30)
	if raw, ok := in["limit"]; ok {
		if value, ok := number(raw); ok && value >= 1 && value <= 100 {
			limit = int32(value)
		}
	}
	rows, err := dbgen.New(m.pool).ExampleRecordList(ctx, dbgen.ExampleRecordListParams{TenantID: p.TenantID, MaxRows: limit})
	if err != nil {
		return contracts.Output{}, err
	}
	records := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		records = append(records, map[string]any{"id": row.ID, "title": row.Title, "status": row.Status})
	}
	return contracts.Output{Data: map[string]any{"records": records}, Summary: fmt.Sprintf("records=%d", len(records))}, nil
}

func (m *Module) create(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can(CreatePermission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	title, _ := in["title"].(string)
	if title == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	row, err := dbgen.New(tx).ExampleRecordCreate(ctx, dbgen.ExampleRecordCreateParams{TenantID: p.TenantID, Title: title, CreatedBy: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	data := map[string]any{"id": row.ID, "title": row.Title, "status": row.Status}
	return contracts.Output{
		Data:    data,
		Summary: fmt.Sprintf("record=%d", row.ID),
		Events: []contracts.Event{{
			Type: CreatedEvent, AggregateType: "example_record", AggregateID: fmt.Sprintf("%d", row.ID),
			Payload: map[string]any{"recordId": row.ID, "title": row.Title, "status": row.Status},
		}},
	}, nil
}

func (m *Module) archive(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can(ArchivePermission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	id, ok := number(in["id"])
	if !ok || id < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	row, err := dbgen.New(tx).ExampleRecordArchive(ctx, dbgen.ExampleRecordArchiveParams{TenantID: p.TenantID, ID: int64(id)})
	if err != nil {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	data := map[string]any{"id": row.ID, "title": row.Title, "status": row.Status}
	return contracts.Output{
		Data:    data,
		Summary: fmt.Sprintf("record=%d", row.ID),
		Events: []contracts.Event{{
			Type: ArchivedEvent, AggregateType: "example_record", AggregateID: fmt.Sprintf("%d", row.ID),
			Payload: map[string]any{"recordId": row.ID, "status": row.Status},
		}},
	}, nil
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	}
	return 0, false
}

func floatPointer(value float64) *float64 { return &value }

// get is scoped to the owning account as well as the tenant, even for admins.
func (m *Module) get(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if !p.Can(ReadPermission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	id, ok := number(in["id"])
	if !ok || id < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	row, err := dbgen.New(m.pool).ExampleRecordGet(ctx, dbgen.ExampleRecordGetParams{TenantID: p.TenantID, ID: int64(id), CreatedBy: p.Account.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": row.ID, "title": row.Title, "status": row.Status, "ready": row.Status == "draft" || row.Status == "active"}}, nil
}
func getDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "example.record.get", Version: "1.0.0", Title: "Inspect own example record", Description: "Read your reference record and whether it is ready for review; an archived record is not ready.", Domain: "example", Entity: "example_record", Permission: ReadPermission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, Executor: "go:example.get", Input: contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"), Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "title": {Type: "string"}, "status": {Type: "string"}, "ready": {Type: "boolean"}}, "id", "title", "status", "ready"), Loggable: []string{"id"}}
}

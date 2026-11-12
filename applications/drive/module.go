// Package drive implements cloud drive capabilities.
package drive

import (
	"context"
	"fmt"
	"strings"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/files"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Module struct {
	pool  *pgxpool.Pool
	files *files.Service
}

func New(pool *pgxpool.Pool) *Module { return &Module{pool: pool} }

// NewWithFiles shares the platform file service and registered storage drivers.
func NewWithFiles(pool *pgxpool.Pool, service *files.Service) *Module {
	return &Module{pool: pool, files: service}
}

func (m *Module) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: personalDefinition(), Handler: contracts.Handler{ExecuteTx: m.ensurePersonal}},
		{Definition: sharedDefinition(), Handler: contracts.Handler{ExecuteTx: m.createShared}},
		{Definition: listDefinition(), Handler: contracts.Handler{Execute: m.listSpaces}},
		{Definition: usageDefinition(), Handler: contracts.Handler{Execute: m.spaceUsage}},
		{Definition: memberListDefinition(), Handler: contracts.Handler{Execute: m.listMembers}},
		{Definition: memberAddDefinition(), Handler: contracts.Handler{ExecuteTx: m.addMember}},
		{Definition: memberChangeDefinition("change"), Handler: contracts.Handler{ExecuteTx: m.changeMember}},
		{Definition: memberChangeDefinition("remove"), Handler: contracts.Handler{ExecuteTx: m.removeMember}},
		{Definition: folderCreateDefinition(), Handler: contracts.Handler{ExecuteTx: m.createFolder}},
		{Definition: folderListDefinition(), Handler: contracts.Handler{Execute: m.listFolders}},
		{Definition: folderPathDefinition(), Handler: contracts.Handler{Execute: m.folderPath}},
		{Definition: folderChangeDefinition("rename"), Handler: contracts.Handler{ExecuteTx: m.renameFolder}},
		{Definition: folderChangeDefinition("move"), Handler: contracts.Handler{ExecuteTx: m.moveFolder}},
		{Definition: folderTrashDefinition(), Handler: contracts.Handler{ExecuteTx: m.trashFolder}},
		{Definition: trashListDefinition(), Handler: contracts.Handler{Execute: m.listTrash}},
		{Definition: trashRestoreDefinition(), Handler: contracts.Handler{ExecuteTx: m.restoreTrash}},
		{Definition: uploadStartDefinition(), Handler: contracts.Handler{ExecuteTx: m.startUpload}},
		{Definition: uploadStatusDefinition(), Handler: contracts.Handler{Execute: m.uploadStatus}},
		{Definition: uploadChunkDefinition(), Handler: contracts.Handler{ExecuteTx: m.uploadChunk}},
		{Definition: uploadCompleteDefinition(), Handler: contracts.Handler{ExecuteTx: m.completeUpload}},
		{Definition: uploadCancelDefinition(), Handler: contracts.Handler{ExecuteTx: m.cancelUpload}},
		{Definition: fileListDefinition(), Handler: contracts.Handler{Execute: m.listFiles}},
		{Definition: fileReadChunkDefinition(), Handler: contracts.Handler{Execute: m.readFileChunk}},
		{Definition: fileChangeDefinition("rename"), Handler: contracts.Handler{ExecuteTx: m.renameFile}},
		{Definition: fileChangeDefinition("move"), Handler: contracts.Handler{ExecuteTx: m.moveFile}},
		{Definition: fileTrashDefinition(), Handler: contracts.Handler{ExecuteTx: m.trashFile}},
		{Definition: fileRestoreDefinition(), Handler: contracts.Handler{ExecuteTx: m.restoreFile}},
		{Definition: purgeRequestDefinition(), Handler: contracts.Handler{ExecuteTx: m.requestPurge}},
		{Definition: cleanupDefinition("claim"), Handler: contracts.Handler{ExecuteTx: m.claimExpiredTrash}},
		{Definition: cleanupDefinition("batch"), Handler: contracts.Handler{ExecuteTx: m.cleanupTrash}},
		{Definition: cleanupDefinition("upload"), Handler: contracts.Handler{ExecuteTx: m.cleanupUpload}},
	}
}

func writeDefinition(name, title, permission, event string) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: title,
		Description: title, Domain: "drive", Entity: "drive_space", Permission: permission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:" + name, Tags: []string{"drive", "write"},
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")}
}

func personalDefinition() contracts.Definition {
	d := writeDefinition("drive.space.personal.ensure", "Ensure personal space", "drive:use", "drive.space.personal.ensured")
	d.Input = contracts.Object(map[string]*contracts.Schema{})
	return d
}

func sharedDefinition() contracts.Definition {
	d := writeDefinition("drive.space.create", "Create shared space", "drive:create", "drive.space.created")
	d.Entities = []string{"drive_space", "drive_member"}
	d.Input = contracts.Object(map[string]*contracts.Schema{"name": contracts.Str(1, 120)}, "name")
	return d
}

func listDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.space.list", Version: "1.0.0",
		Title: "List drive spaces", Description: "List spaces accessible to the caller.", Domain: "drive", Entity: "drive_space",
		Permission: "drive:use", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.space.list", Tags: []string{"drive", "read"}, Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "kind": {Type: "string"}, "name": {Type: "string"}, "role": {Type: "string"},
		}, "id", "kind", "name", "role")}}, "items")}
}

func (m *Module) ensurePersonal(ctx context.Context, tx pgx.Tx, _ contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	id, err := dbgen.New(tx).DrivePersonalEnsure(ctx, dbgen.DrivePersonalEnsureParams{TenantID: p.TenantID, OwnerID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	return spaceOutput(id, "drive.space.personal.ensured"), nil
}

func (m *Module) createShared(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	name, _ := in["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	id, err := q.DriveSharedCreate(ctx, dbgen.DriveSharedCreateParams{TenantID: p.TenantID, Name: name, OwnerID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	if err = q.DriveCreatorGrant(ctx, dbgen.DriveCreatorGrantParams{TenantID: p.TenantID, SpaceID: id, AccountID: pgtype.Int8{Int64: p.Account.ID, Valid: true}}); err != nil {
		return contracts.Output{}, err
	}
	return spaceOutput(id, "drive.space.created"), nil
}

// First-phase writes are restricted to an interactive user. Discovery tags do
// not enforce origin and must not be used as an authorization boundary.
func requireHuman(ctx context.Context) error {
	invocation, ok := gateway.CurrentInvocation(ctx)
	if !ok || invocation.Origin != contracts.OriginHumanUI {
		return contracts.Problem("forbidden")
	}
	return nil
}

func (m *Module) listSpaces(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	rows, err := dbgen.New(m.pool).DriveSpaceList(ctx, dbgen.DriveSpaceListParams{TenantID: p.TenantID,
		AccountID: pgtype.Int8{Int64: p.Account.ID, Valid: true}, DepartmentID: pgtype.Int8{Int64: p.Account.DepartmentID, Valid: true}})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "kind": row.Kind, "name": row.Name, "role": row.Role})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

func spaceOutput(id int64, event string) contracts.Output {
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: event,
		AggregateType: "drive_space", AggregateID: fmt.Sprint(id), Payload: map[string]any{"spaceId": id}}}}
}

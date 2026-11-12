package drive

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
)

func trashWriteDefinition(name, title, event string) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: title,
		Description: title, Domain: "drive", Entity: "drive_folder", Entities: []string{"drive_folder", "drive_file", "drive_trash_batch"},
		Permission: "drive:use", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:" + name, Tags: []string{"drive", "write"},
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")}
}

func folderTrashDefinition() contracts.Definition {
	d := trashWriteDefinition("drive.folder.trash", "Move folder to trash", "drive.folder.trashed")
	d.Input = contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "folderId": contracts.Int(1, 1<<53)}, "spaceId", "folderId")
	d.Loggable = []string{"spaceId", "folderId"}
	return d
}

func trashRestoreDefinition() contracts.Definition {
	d := trashWriteDefinition("drive.trash.restore", "Restore a deleted folder tree", "drive.folder.restored")
	d.Input = contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "batchId": contracts.Int(1, 1<<53),
		"parentId": contracts.Int(0, 1<<53), "name": contracts.Str(1, 200)}, "spaceId", "batchId", "parentId", "name")
	d.Loggable = []string{"spaceId", "batchId", "parentId"}
	return d
}

func trashListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.trash.list", Version: "1.0.0",
		Title: "List deleted items", Description: "List recoverable file and folder deletions in an editable space.",
		Domain: "drive", Entity: "drive_trash_batch", Permission: "drive:use", Risk: contracts.RiskLow,
		Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true, Executor: "go:drive.trash.list", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53)}, "spaceId"),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "kind": {Type: "string"}, "folderId": {Type: "integer"}, "fileId": {Type: "integer"}, "name": {Type: "string"}, "parentId": {Type: "integer"},
			"deletedAt": {Type: "string"}, "expiresAt": {Type: "string"},
		}, "id", "kind", "folderId", "fileId", "name", "parentId", "deletedAt", "expiresAt")}}, "items"), Loggable: []string{"spaceId"}}
}

type trashInput struct {
	SpaceID  int64  `json:"spaceId"`
	FolderID int64  `json:"folderId"`
	BatchID  int64  `json:"batchId"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
}

func lockWriteSpace(ctx context.Context, q *dbgen.Queries, p *identity.Principal, spaceID int64) error {
	_, err := q.DriveSpaceWriteLock(ctx, dbgen.DriveSpaceWriteLockParams{TenantID: p.TenantID, SpaceID: spaceID,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	return memberProblem(err)
}

func (m *Module) trashFolder(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input trashInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	batch, err := q.DriveFolderTrashBatchCreate(ctx, dbgen.DriveFolderTrashBatchCreateParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, FolderID: input.FolderID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveFolderTrashMark(ctx, dbgen.DriveFolderTrashMarkParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, FolderID: input.FolderID, BatchID: idValue(batch)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if count == 0 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if _, err = q.DriveFolderTrashMarkFiles(ctx, dbgen.DriveFolderTrashMarkFilesParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, BatchID: idValue(batch)}); err != nil {
		return contracts.Output{}, err
	}
	return trashOutput(batch, input.SpaceID, input.FolderID, "drive.folder.trashed"), nil
}

func (m *Module) restoreTrash(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input trashInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	folder, err := q.DriveTrashRootPrepare(ctx, dbgen.DriveTrashRootPrepareParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		BatchID: idValue(input.BatchID), ParentID: optionalID(input.ParentID), Name: input.Name,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveTrashRestoreFolders(ctx, dbgen.DriveTrashRestoreFoldersParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: idValue(input.BatchID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if count == 0 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if _, err = q.DriveTrashRestoreFiles(ctx, dbgen.DriveTrashRestoreFilesParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, BatchID: idValue(input.BatchID)}); err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err = q.DriveTrashBatchDelete(ctx, dbgen.DriveTrashBatchDeleteParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: input.BatchID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return trashOutput(folder, input.SpaceID, folder, "drive.folder.restored"), nil
}

func (m *Module) listTrash(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input trashInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(m.pool).DriveTrashList(ctx, dbgen.DriveTrashListParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "kind": "folder", "folderId": row.FolderID, "fileId": int64(0), "name": row.Name,
			"parentId": row.ParentID, "deletedAt": row.DeletedAt.Time.UTC().Format(time.RFC3339), "expiresAt": row.ExpiresAt.Time.UTC().Format(time.RFC3339)})
	}
	fileRows, err := dbgen.New(m.pool).DriveTrashFilesList(ctx, dbgen.DriveTrashFilesListParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	for _, row := range fileRows {
		items = append(items, map[string]any{"id": row.ID, "kind": "file", "folderId": int64(0), "fileId": row.FileID, "name": row.Name,
			"parentId": row.FolderID, "deletedAt": row.DeletedAt.Time.UTC().Format(time.RFC3339), "expiresAt": row.ExpiresAt.Time.UTC().Format(time.RFC3339)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["id"].(int64) > items[j]["id"].(int64) })
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

func trashOutput(id, spaceID, folderID int64, event string) contracts.Output {
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: event,
		AggregateType: "drive_folder", AggregateID: fmt.Sprint(folderID), Payload: map[string]any{"spaceId": spaceID, "folderId": folderID}}}}
}

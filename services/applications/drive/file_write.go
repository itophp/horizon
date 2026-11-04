package drive

import (
	"context"
	"fmt"
	"strings"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

func fileWriteDefinition(name, title, event string) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: title, Description: title,
		Domain: "drive", Entity: "drive_file", Permission: "drive:use", Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:" + name, Tags: []string{"drive", "write"},
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")}
}

func fileChangeDefinition(action string) contracts.Definition {
	d := fileWriteDefinition("drive.file."+action, "Change a drive file", "drive.file."+action+"d")
	fields := map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "fileId": contracts.Int(1, 1<<53)}
	required := []string{"spaceId", "fileId"}
	if action == "rename" {
		fields["name"] = contracts.Str(1, 200)
		required = append(required, "name")
	} else {
		fields["folderId"] = contracts.Int(0, 1<<53)
		required = append(required, "folderId")
	}
	d.Input = contracts.Object(fields, required...)
	d.Loggable = []string{"spaceId", "fileId"}
	return d
}

func fileTrashDefinition() contracts.Definition {
	d := fileWriteDefinition("drive.file.trash", "Move a file to trash", "drive.file.trashed")
	d.Entities = []string{"drive_file", "drive_trash_batch"}
	d.Input = contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "fileId": contracts.Int(1, 1<<53)}, "spaceId", "fileId")
	d.Loggable = []string{"spaceId", "fileId"}
	return d
}

func fileRestoreDefinition() contracts.Definition {
	d := fileWriteDefinition("drive.trash.file.restore", "Restore a deleted file", "drive.file.restored")
	d.Entities = []string{"drive_file", "drive_trash_batch"}
	d.Input = contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "batchId": contracts.Int(1, 1<<53),
		"folderId": contracts.Int(0, 1<<53), "name": contracts.Str(1, 200)}, "spaceId", "batchId", "folderId", "name")
	d.Loggable = []string{"spaceId", "batchId", "folderId"}
	return d
}

type fileWriteInput struct {
	SpaceID  int64  `json:"spaceId"`
	FileID   int64  `json:"fileId"`
	FolderID int64  `json:"folderId"`
	BatchID  int64  `json:"batchId"`
	Name     string `json:"name"`
}

func (m *Module) renameFile(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.changeFile(ctx, tx, in, "rename")
}
func (m *Module) moveFile(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.changeFile(ctx, tx, in, "move")
}

func (m *Module) changeFile(ctx context.Context, tx pgx.Tx, in contracts.Input, action string) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input fileWriteInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	var count int64
	if action == "rename" {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		count, err = q.DriveFileRename(ctx, dbgen.DriveFileRenameParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
			FileID: input.FileID, Name: input.Name, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	} else {
		count, err = q.DriveFileMove(ctx, dbgen.DriveFileMoveParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
			FileID: input.FileID, FolderID: optionalID(input.FolderID), ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	}
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return fileOutput(input.FileID, input.SpaceID, input.FileID, "drive.file."+action+"d"), nil
}

func (m *Module) trashFile(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input fileWriteInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	batch, err := q.DriveFileTrashBatchCreate(ctx, dbgen.DriveFileTrashBatchCreateParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		FileID: input.FileID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveFileTrashMark(ctx, dbgen.DriveFileTrashMarkParams{TenantID: p.TenantID, SpaceID: input.SpaceID, FileID: input.FileID, BatchID: idValue(batch)})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return fileOutput(batch, input.SpaceID, input.FileID, "drive.file.trashed"), nil
}

func (m *Module) restoreFile(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input fileWriteInput
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
	id, err := q.DriveTrashFileRestore(ctx, dbgen.DriveTrashFileRestoreParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		BatchID: idValue(input.BatchID), FolderID: optionalID(input.FolderID), Name: input.Name,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveTrashBatchDelete(ctx, dbgen.DriveTrashBatchDeleteParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: input.BatchID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return fileOutput(id, input.SpaceID, id, "drive.file.restored"), nil
}

func fileOutput(id, spaceID, fileID int64, event string) contracts.Output {
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: event, AggregateType: "drive_file",
		AggregateID: fmt.Sprint(fileID), Payload: map[string]any{"spaceId": spaceID, "fileId": fileID}}}}
}

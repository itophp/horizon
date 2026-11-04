package drive

import (
	"context"
	"fmt"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/system/files"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

type cleanupInput struct {
	SpaceID  int64 `json:"spaceId"`
	BatchID  int64 `json:"batchId"`
	UploadID int64 `json:"uploadId"`
}

func purgeRequestDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.trash.purge.request", Version: "1.0.0",
		Title: "Permanently delete a trash item", Description: "Request irreversible cleanup of one trash batch.",
		Domain: "drive", Entity: "drive_trash_batch", Permission: "drive:use", Risk: contracts.RiskHigh, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.trash.purge.requested"}, Executor: "go:drive.trash.purge.request", Tags: []string{"drive", "write"},
		Input:  contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "batchId": contracts.Int(1, 1<<53)}, "spaceId", "batchId"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Loggable: []string{"spaceId", "batchId"}}
}

func cleanupDefinition(action string) contracts.Definition {
	entity := "drive_trash_batch"
	entities := []string{"drive_trash_batch"}
	id := "batchId"
	if action == "batch" {
		entities = []string{"drive_trash_batch", "drive_folder", "drive_file", "drive_upload", "file"}
	}
	if action == "upload" {
		entity = "drive_upload"
		entities = []string{"drive_upload"}
		id = "uploadId"
	}
	event := "drive.cleanup." + action + ".completed"
	if action == "batch" {
		event = "drive.cleanup.batch.processed"
	}
	return contracts.Definition{Layer: contracts.LayerAtomic, Effect: contracts.EffectIrreversible, RetrySafe: true,
		Name: "drive.cleanup." + action, Version: "1.0.0", Title: "Process drive retention", Description: "Internal, tenant-scoped retention processing.",
		Domain: "drive", Entity: entity, Entities: entities, Permission: "drive:cleanup", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{event}, Executor: "go:drive.cleanup." + action, Tags: []string{"drive", "internal"},
		Input:  contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), id: contracts.Int(1, 1<<53)}, "spaceId", id),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Loggable: []string{"spaceId", id}}
}

func requireSystem(ctx context.Context) (*identity.Principal, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, contracts.Problem("unauthenticated")
	}
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok || meta.Origin != contracts.OriginEvent || p.Account.ID != 0 || !p.Can("drive:cleanup") {
		return nil, contracts.Problem("forbidden")
	}
	return p, nil
}

func (m *Module) requestPurge(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input cleanupInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	count, err := q.DriveTrashPurgeRequest(ctx, dbgen.DriveTrashPurgeRequestParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		BatchID: input.BatchID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return cleanupOutput(input.BatchID, "drive.trash.purge.requested", "drive_trash_batch"), nil
}

func (m *Module) claimExpiredTrash(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := requireSystem(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	var input cleanupInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if _, err = q.DriveSystemSpaceLock(ctx, dbgen.DriveSystemSpaceLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID}); err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveTrashClaimExpired(ctx, dbgen.DriveTrashClaimExpiredParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: input.BatchID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return cleanupOutput(input.BatchID, "drive.cleanup.claim.completed", "drive_trash_batch"), nil
}

func (m *Module) cleanupTrash(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := requireSystem(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if m.files == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	var input cleanupInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if _, err = q.DriveSystemSpaceLock(ctx, dbgen.DriveSystemSpaceLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID}); err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if _, err = q.DriveTrashPurgeLock(ctx, dbgen.DriveTrashPurgeLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: input.BatchID}); err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	objects, err := q.DrivePurgeFiles(ctx, dbgen.DrivePurgeFilesParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: idValue(input.BatchID)})
	if err != nil {
		return contracts.Output{}, err
	}
	for _, object := range objects {
		profile, err := m.files.ProfileInTx(ctx, tx, p.TenantID, object.StorageProfileID)
		if err != nil {
			return contracts.Output{}, err
		}
		driver, err := m.files.Storage.Get(profile.Driver)
		if err != nil {
			return contracts.Output{}, err
		}
		if err = deleteStorageObject(ctx, driver, profile, object.StorageKey); err != nil {
			return contracts.Output{}, err
		}
		count, err := q.DrivePurgeStoredFile(ctx, dbgen.DrivePurgeStoredFileParams{TenantID: p.TenantID, FileID: object.ID})
		if err != nil {
			return contracts.Output{}, err
		}
		if count != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		count, err = q.DrivePurgeOneFile(ctx, dbgen.DrivePurgeOneFileParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
			BatchID: idValue(input.BatchID), DriveID: object.DriveID})
		if err != nil {
			return contracts.Output{}, err
		}
		if count != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
	}
	batch := idValue(input.BatchID)
	remaining, err := q.DrivePurgeFilesRemaining(ctx, dbgen.DrivePurgeFilesRemainingParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: batch})
	if err != nil {
		return contracts.Output{}, err
	}
	if remaining {
		return cleanupOutput(input.BatchID, "drive.cleanup.batch.processed", "drive_trash_batch"), nil
	}
	if _, err = q.DrivePurgeDetachFiles(ctx, dbgen.DrivePurgeDetachFilesParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: batch}); err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.DrivePurgeDetachFolders(ctx, dbgen.DrivePurgeDetachFoldersParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: batch}); err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.DrivePurgeDetachUploads(ctx, dbgen.DrivePurgeDetachUploadsParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: batch}); err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.DrivePurgeFolderRows(ctx, dbgen.DrivePurgeFolderRowsParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: batch}); err != nil {
		return contracts.Output{}, err
	}
	count, err := q.DriveTrashBatchDelete(ctx, dbgen.DriveTrashBatchDeleteParams{TenantID: p.TenantID, SpaceID: input.SpaceID, BatchID: input.BatchID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return cleanupOutput(input.BatchID, "drive.cleanup.batch.processed", "drive_trash_batch"), nil
}

func (m *Module) cleanupUpload(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := requireSystem(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if m.files == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	var input cleanupInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if _, err = q.DriveSystemSpaceLock(ctx, dbgen.DriveSystemSpaceLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID}); err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	u, err := q.DriveCleanupUploadLock(ctx, dbgen.DriveCleanupUploadLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID, UploadID: input.UploadID})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	profile, err := m.files.ProfileInTx(ctx, tx, p.TenantID, u.StorageProfileID)
	if err != nil {
		return contracts.Output{}, err
	}
	driver, err := m.files.Storage.Get(profile.Driver)
	if err != nil {
		return contracts.Output{}, err
	}
	keys, err := q.DriveCleanupUploadKeys(ctx, dbgen.DriveCleanupUploadKeysParams{TenantID: p.TenantID, UploadID: input.UploadID})
	if err != nil {
		return contracts.Output{}, err
	}
	for _, key := range keys {
		if err = deleteStorageObject(ctx, driver, profile, key); err != nil {
			return contracts.Output{}, err
		}
	}
	if u.State != "completed" {
		if err = deleteStorageObject(ctx, driver, profile, u.ObjectKey); err != nil {
			return contracts.Output{}, err
		}
	}
	count, err := q.DriveCleanupUploadFinish(ctx, dbgen.DriveCleanupUploadFinishParams{TenantID: p.TenantID, SpaceID: input.SpaceID, UploadID: input.UploadID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return cleanupOutput(input.UploadID, "drive.cleanup.upload.completed", "drive_upload"), nil
}

func deleteStorageObject(ctx context.Context, driver files.Driver, profile files.Profile, key string) error {
	if err := driver.Delete(ctx, profile, key); err != nil {
		return contracts.Problem("unavailable")
	}
	if cleaner, ok := driver.(files.TemporaryCleaner); ok {
		if err := cleaner.ClearTemporary(ctx, profile, key); err != nil {
			return contracts.Problem("unavailable")
		}
	}
	return nil
}

func cleanupOutput(id int64, event, aggregate string) contracts.Output {
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: event, AggregateType: aggregate,
		AggregateID: fmt.Sprint(id), Payload: map[string]any{"id": id}}}}
}

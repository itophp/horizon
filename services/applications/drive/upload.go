package drive

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/files"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

const maxUploadSize int64 = 1 << 30

func uploadStartDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.upload.start", Version: "1.0.0",
		Title: "Start a file upload", Description: "Create an upload session and reserve bounded chunk slots.",
		Domain: "drive", Entity: "drive_upload", Entities: []string{"drive_upload", "drive_upload_chunk"},
		Permission: "drive:use", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.upload.started"}, Executor: "go:drive.upload.start", Tags: []string{"drive", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "folderId": contracts.Int(0, 1<<53),
			"name": contracts.Str(1, 200), "size": contracts.Int(0, float64(maxUploadSize))}, "spaceId", "name", "size"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "chunkSize": {Type: "integer"}}, "id", "chunkSize"),
		Loggable: []string{"spaceId", "folderId", "size"}}
}

func uploadStatusDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.upload.status", Version: "1.0.0",
		Title: "Inspect an upload", Description: "Inspect the caller's upload session and completed chunk numbers.",
		Domain: "drive", Entity: "drive_upload", Permission: "drive:use", Risk: contracts.RiskLow,
		Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.upload.status", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "spaceId": {Type: "integer"}, "folderId": {Type: "integer"},
			"name": {Type: "string"}, "size": {Type: "integer"}, "state": {Type: "string"}, "expiresAt": {Type: "string"},
			"completedFileId": {Type: "integer"}, "completedChunks": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"index": {Type: "integer"}, "sha256": {Type: "string"},
			}, "index", "sha256")}},
			"id", "spaceId", "folderId", "name", "size", "state", "expiresAt", "completedFileId", "completedChunks"), Loggable: []string{"id"}}
}

func uploadCancelDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.upload.cancel", Version: "1.0.0",
		Title: "Cancel an upload", Description: "Cancel the caller's upload and schedule its temporary objects for cleanup.",
		Domain: "drive", Entity: "drive_upload", Permission: "drive:use", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.upload.cancelled"}, Executor: "go:drive.upload.cancel", Tags: []string{"drive", "write"},
		Input:  contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "uploadId": contracts.Int(1, 1<<53)}, "spaceId", "uploadId"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Loggable: []string{"spaceId", "uploadId"}}
}

func (m *Module) cancelUpload(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input chunkInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	count, err := q.DriveUploadCancel(ctx, dbgen.DriveUploadCancelParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		UploadID: input.UploadID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: map[string]any{"id": input.UploadID}, Events: []contracts.Event{{Type: "drive.upload.cancelled",
		AggregateType: "drive_upload", AggregateID: fmt.Sprint(input.UploadID), Payload: map[string]any{"uploadId": input.UploadID}}}}, nil
}

type uploadInput struct {
	ID       int64  `json:"id"`
	SpaceID  int64  `json:"spaceId"`
	FolderID int64  `json:"folderId"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

func (m *Module) startUpload(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if m.files == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	var input uploadInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.Size < 0 || input.Size > maxUploadSize {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	if err = m.checkUploadSettings(ctx, tx, p, input.Name, input.Size); err != nil {
		return contracts.Output{}, err
	}
	_, profileID, err := m.files.PrepareProfileInTx(ctx, tx, p)
	if err != nil {
		return contracts.Output{}, err
	}
	objectKey, err := governance.Token()
	if err != nil {
		return contracts.Output{}, err
	}
	id, err := q.DriveUploadCreate(ctx, dbgen.DriveUploadCreateParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		FolderID: optionalID(input.FolderID), ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID),
		StorageProfileID: profileID, ObjectKey: objectKey, Name: input.Name, Size: input.Size})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if err = m.checkSpaceQuota(ctx, tx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	for offset, index := int64(0), int32(0); offset < input.Size; index++ {
		length := min(files.CloudDriveChunkLimit, input.Size-offset)
		key, err := governance.Token()
		if err != nil {
			return contracts.Output{}, err
		}
		if err = q.DriveUploadChunkReserve(ctx, dbgen.DriveUploadChunkReserveParams{TenantID: p.TenantID, UploadID: id,
			ChunkIndex: index, StorageKey: key, ExpectedSize: length}); err != nil {
			return contracts.Output{}, err
		}
		offset += length
	}
	return contracts.Output{Data: map[string]any{"id": id, "chunkSize": files.CloudDriveChunkLimit},
		Events: []contracts.Event{{Type: "drive.upload.started", AggregateType: "drive_upload", AggregateID: fmt.Sprint(id),
			Payload: map[string]any{"spaceId": input.SpaceID, "uploadId": id}}}}, nil
}

func (m *Module) checkUploadSettings(ctx context.Context, tx pgx.Tx, p *identity.Principal, name string, size int64) error {
	raw, err := m.files.SettingInTx(ctx, tx, p, "drive.maxBytes")
	if err != nil {
		return err
	}
	limit, err := governance.Number(raw)
	if err != nil {
		return err
	}
	if size < 0 || size > maxUploadSize || size > limit {
		return contracts.Problem("invalid")
	}
	raw, err = m.files.SettingInTx(ctx, tx, p, "drive.blockedExtensions")
	if err != nil {
		return err
	}
	blocked, err := governance.String(raw)
	if err != nil {
		return err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	for _, item := range strings.FieldsFunc(blocked, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if value := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), "."); value != "" && value == ext {
			return contracts.Problem("invalid")
		}
	}
	return nil
}

func (m *Module) checkSpaceQuota(ctx context.Context, tx pgx.Tx, q *dbgen.Queries, p *identity.Principal, spaceID int64) error {
	raw, err := m.files.SettingInTx(ctx, tx, p, "drive.quotaBytes")
	if err != nil {
		return err
	}
	quota, err := governance.Number(raw)
	if err != nil {
		return err
	}
	if quota == 0 {
		return nil
	}
	used, err := q.DriveSpaceReservedBytes(ctx, dbgen.DriveSpaceReservedBytesParams{TenantID: p.TenantID, SpaceID: spaceID})
	if err != nil {
		return err
	}
	if used > quota {
		return contracts.Problem("conflict")
	}
	return nil
}

func (m *Module) uploadStatus(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input uploadInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(m.pool)
	params := dbgen.DriveUploadStatusParams{TenantID: p.TenantID, UploadID: input.ID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)}
	row, err := q.DriveUploadStatus(ctx, params)
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	chunks, err := q.DriveUploadCompletedChunks(ctx, dbgen.DriveUploadCompletedChunksParams{TenantID: p.TenantID,
		UploadID: input.ID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	completed := make([]map[string]any, 0, len(chunks))
	for _, chunk := range chunks {
		if !chunk.Sha256.Valid {
			return contracts.Output{}, contracts.Problem("internal")
		}
		completed = append(completed, map[string]any{"index": chunk.ChunkIndex, "sha256": chunk.Sha256.String})
	}
	return contracts.Output{Data: map[string]any{"id": row.ID, "spaceId": row.SpaceID, "folderId": row.FolderID, "name": row.Name,
		"size": row.Size, "state": row.State, "expiresAt": row.ExpiresAt.Time.UTC().Format(time.RFC3339), "completedFileId": row.CompletedFileID, "completedChunks": completed}}, nil
}

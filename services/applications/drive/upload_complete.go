package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/files"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

func uploadCompleteDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.upload.complete", Version: "1.0.0",
		Title: "Complete a file upload", Description: "Verify and assemble all uploaded chunks into a drive file.",
		Domain: "drive", Entity: "drive_upload", Entities: []string{"drive_upload", "drive_file", "file"},
		Permission: "drive:use", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.file.uploaded"}, Executor: "go:drive.upload.complete", Tags: []string{"drive", "write"},
		Input:  contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "uploadId": contracts.Int(1, 1<<53)}, "spaceId", "uploadId"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Loggable: []string{"spaceId", "uploadId"}}
}

func (m *Module) completeUpload(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
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
	var input chunkInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	u, err := q.DriveUploadCompleteLock(ctx, dbgen.DriveUploadCompleteLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		UploadID: input.UploadID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if err = m.checkUploadSettings(ctx, tx, p, u.Name, u.Size); err != nil {
		return contracts.Output{}, err
	}
	if err = m.checkSpaceQuota(ctx, tx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	chunks, err := q.DriveUploadChunksForAssembly(ctx, dbgen.DriveUploadChunksForAssemblyParams{TenantID: p.TenantID, UploadID: input.UploadID})
	if err != nil {
		return contracts.Output{}, err
	}
	if err = validateAssembly(chunks, u.Size); err != nil {
		return contracts.Output{}, err
	}
	profile, err := m.files.ProfileInTx(ctx, tx, p.TenantID, u.StorageProfileID)
	if err != nil {
		return contracts.Output{}, err
	}
	driver, err := m.files.Storage.Get(profile.Driver)
	if err != nil {
		return contracts.Output{}, err
	}
	digest, err := assembleObject(ctx, driver, profile, u.ObjectKey, chunks, u.Size)
	if err != nil {
		return contracts.Output{}, err
	}
	fileID, err := q.GovCreateFile(ctx, dbgen.GovCreateFileParams{TenantID: p.TenantID, OwnerID: p.Account.ID,
		OrganizationID: p.Account.OrganizationID, DepartmentID: p.Account.DepartmentID, StorageProfileID: u.StorageProfileID,
		StorageKey: u.ObjectKey, Name: u.Name, Mime: "application/octet-stream", Size: u.Size, Purpose: "cloud_drive", Sha256: hex.EncodeToString(digest[:])})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	driveID, err := q.DriveFileCreate(ctx, dbgen.DriveFileCreateParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		FolderID: u.FolderID, FileID: fileID, Name: u.Name, ActorID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	count, err := q.DriveUploadComplete(ctx, dbgen.DriveUploadCompleteParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		UploadID: input.UploadID, FileID: idValue(fileID)})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": driveID}, Events: []contracts.Event{{Type: "drive.file.uploaded",
		AggregateType: "drive_file", AggregateID: fmt.Sprint(driveID), Payload: map[string]any{"spaceId": input.SpaceID, "fileId": driveID}}}}, nil
}

func validateAssembly(chunks []dbgen.DriveUploadChunksForAssemblyRow, size int64) error {
	expectedCount := (size + files.CloudDriveChunkLimit - 1) / files.CloudDriveChunkLimit
	if size < 0 || size > maxUploadSize || int64(len(chunks)) != expectedCount {
		return contracts.Problem("conflict")
	}
	var total int64
	for index, chunk := range chunks {
		if int(chunk.ChunkIndex) != index || !chunk.Sha256.Valid || !chunk.UploadedAt.Valid || chunk.ExpectedSize != min(files.CloudDriveChunkLimit, size-total) {
			return contracts.Problem("conflict")
		}
		total += chunk.ExpectedSize
	}
	if total != size {
		return contracts.Problem("conflict")
	}
	return nil
}

type assemblyResult struct {
	digest [sha256.Size]byte
	err    error
}

// assembleObject may be retried after a database rollback: the reserved final
// key exists before any bytes are written, and an existing object is accepted
// only when it matches the currently verified chunks.
func assembleObject(ctx context.Context, driver files.Driver, profile files.Profile, key string, chunks []dbgen.DriveUploadChunksForAssemblyRow, size int64) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	existing, err := driver.Read(ctx, profile, key)
	if err == nil {
		digest, copyErr := copyVerifiedChunks(ctx, driver, profile, chunks, io.Discard)
		if copyErr != nil {
			_ = existing.Close()
			return empty, copyErr
		}
		h := files.NewFileDigest()
		n, readErr := io.Copy(h, io.LimitReader(existing, size+1))
		closeErr := existing.Close()
		if readErr != nil || closeErr != nil {
			return empty, contracts.Problem("unavailable")
		}
		if n != size || h.Sum() != digest {
			return empty, contracts.Problem("conflict")
		}
		return digest, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return empty, contracts.Problem("unavailable")
	}
	if cleaner, ok := driver.(files.TemporaryCleaner); ok {
		if err = cleaner.ClearTemporary(ctx, profile, key); err != nil {
			return empty, contracts.Problem("unavailable")
		}
	}
	reader, writer := io.Pipe()
	done := make(chan assemblyResult, 1)
	go func() {
		digest, err := copyVerifiedChunks(ctx, driver, profile, chunks, writer)
		_ = writer.CloseWithError(err)
		done <- assemblyResult{digest: digest, err: err}
	}()
	writeErr := driver.Write(ctx, profile, key, reader)
	if writeErr != nil {
		_ = reader.CloseWithError(writeErr)
	} else {
		_ = reader.Close()
	}
	result := <-done
	if result.err != nil {
		return empty, result.err
	}
	if writeErr != nil {
		return empty, contracts.Problem("unavailable")
	}
	return result.digest, nil
}

func copyVerifiedChunks(ctx context.Context, driver files.Driver, profile files.Profile, chunks []dbgen.DriveUploadChunksForAssemblyRow, dst io.Writer) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	full := files.NewFileDigest()
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		decoded, err := hex.DecodeString(chunk.Sha256.String)
		if err != nil || len(decoded) != sha256.Size {
			return empty, contracts.Problem("conflict")
		}
		var expected [sha256.Size]byte
		copy(expected[:], decoded)
		reader, err := driver.Read(ctx, profile, chunk.StorageKey)
		if err != nil {
			return empty, contracts.Problem("unavailable")
		}
		verifyErr := files.VerifyChunk(io.MultiWriter(dst, full), reader, chunk.ExpectedSize, expected)
		closeErr := reader.Close()
		if errors.Is(verifyErr, files.ErrChunkIntegrity) {
			return empty, contracts.Problem("conflict")
		}
		if verifyErr != nil || closeErr != nil {
			return empty, contracts.Problem("unavailable")
		}
	}
	return full.Sum(), nil
}

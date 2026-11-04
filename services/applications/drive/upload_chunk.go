package drive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	"github.com/jackc/pgx/v5/pgtype"
)

func uploadChunkDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.upload.chunk", Version: "1.0.0",
		Title: "Upload one chunk", Description: "Write one preallocated, bounded and digest-verified upload chunk.",
		Domain: "drive", Entity: "drive_upload_chunk", Permission: "drive:use", Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.upload.chunk.stored"}, Executor: "go:drive.upload.chunk", Tags: []string{"drive", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "uploadId": contracts.Int(1, 1<<53),
			"index": contracts.Int(0, 255), "sha256": contracts.Str(64, 64), "content": contracts.Str(1, 5592408)},
			"spaceId", "uploadId", "index", "sha256", "content"),
		Output:   contracts.Object(map[string]*contracts.Schema{"index": {Type: "integer"}}, "index"),
		Loggable: []string{"spaceId", "uploadId", "index"}}
}

type chunkInput struct {
	SpaceID  int64  `json:"spaceId"`
	UploadID int64  `json:"uploadId"`
	Index    int32  `json:"index"`
	SHA256   string `json:"sha256"`
	Content  string `json:"content"`
}

func (m *Module) uploadChunk(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
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
	digest, err := hex.DecodeString(input.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(input.Content)
	if err != nil || int64(len(content)) > files.CloudDriveChunkLimit {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	var expected [sha256.Size]byte
	copy(expected[:], digest)
	if err = files.VerifyChunk(io.Discard, bytes.NewReader(content), int64(len(content)), expected); err != nil {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	if err = lockWriteSpace(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	row, err := q.DriveUploadChunkLock(ctx, dbgen.DriveUploadChunkLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		UploadID: input.UploadID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID), ChunkIndex: input.Index})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if int64(len(content)) != row.ExpectedSize {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	canonicalHash := hex.EncodeToString(expected[:])
	if row.Sha256.Valid && row.Sha256.String != canonicalHash {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	profile, err := m.files.ProfileInTx(ctx, tx, p.TenantID, row.StorageProfileID)
	if err != nil {
		return contracts.Output{}, err
	}
	driver, err := m.files.Storage.Get(profile.Driver)
	if err != nil {
		return contracts.Output{}, err
	}
	reader, err := driver.Read(ctx, profile, row.StorageKey)
	if err == nil {
		verifyErr := files.VerifyChunk(io.Discard, reader, row.ExpectedSize, expected)
		closeErr := reader.Close()
		if errors.Is(verifyErr, files.ErrChunkIntegrity) {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		if verifyErr != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		if closeErr != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if row.UploadedAt.Valid {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		if cleaner, ok := driver.(files.TemporaryCleaner); ok {
			if err = cleaner.ClearTemporary(ctx, profile, row.StorageKey); err != nil {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
		}
		if err = driver.Write(ctx, profile, row.StorageKey, bytes.NewReader(content)); err != nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
	} else {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	count, err := q.DriveUploadChunkMark(ctx, dbgen.DriveUploadChunkMarkParams{TenantID: p.TenantID, UploadID: input.UploadID,
		ChunkIndex: input.Index, Sha256: pgtype.Text{String: canonicalHash, Valid: true}})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"index": input.Index}, Events: []contracts.Event{{Type: "drive.upload.chunk.stored",
		AggregateType: "drive_upload", AggregateID: fmt.Sprint(input.UploadID), Payload: map[string]any{"uploadId": input.UploadID, "index": input.Index}}}}, nil
}

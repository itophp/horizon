package drive

import (
	"context"
	"encoding/base64"
	"io"
	"time"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/files"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func fileListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.file.list", Version: "1.0.0",
		Title: "List drive files", Description: "List files in an accessible space and folder.", Domain: "drive", Entity: "drive_file",
		Permission: "drive:use", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.file.list", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "folderId": contracts.Int(0, 1<<53)}, "spaceId"),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "folderId": {Type: "integer"},
			"size": {Type: "integer"}, "sha256": {Type: "string"}, "createdAt": {Type: "string"},
		}, "id", "name", "folderId", "size", "sha256", "createdAt")}}, "items"), Loggable: []string{"spaceId", "folderId"}}
}

func fileReadChunkDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.file.chunk.read", Version: "1.0.0",
		Title: "Read file chunk", Description: "Read a bounded range of an authorized drive file.", Domain: "drive", Entity: "drive_file",
		Permission: "drive:use", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.file.chunk.read", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "fileId": contracts.Int(1, 1<<53),
			"offset": contracts.Int(0, float64(maxUploadSize))}, "spaceId", "fileId", "offset"),
		Output: contracts.Object(map[string]*contracts.Schema{"name": {Type: "string"}, "size": {Type: "integer"}, "sha256": {Type: "string"},
			"offset": {Type: "integer"}, "content": contracts.Str(0, 5592408), "eof": {Type: "boolean"}},
			"name", "size", "sha256", "offset", "content", "eof"), Loggable: []string{"spaceId", "fileId", "offset"}}
}

type fileReadInput struct {
	SpaceID  int64 `json:"spaceId"`
	FolderID int64 `json:"folderId"`
	FileID   int64 `json:"fileId"`
	Offset   int64 `json:"offset"`
}

func (m *Module) listFiles(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input fileReadInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(m.pool).DriveFileList(ctx, dbgen.DriveFileListParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		FolderID: optionalID(input.FolderID), ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "name": row.Name, "folderId": row.FolderID.Int64,
			"size": row.Size, "sha256": row.Sha256, "createdAt": row.CreatedAt.Time.UTC().Format(time.RFC3339)})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

func (m *Module) readFileChunk(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if m.files == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	var input fileReadInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(m.pool).DriveFileRead(ctx, dbgen.DriveFileReadParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		FileID: input.FileID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if input.Offset > row.Size {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	profile, err := m.files.Profile(ctx, p.TenantID, row.StorageProfileID)
	if err != nil {
		return contracts.Output{}, err
	}
	driver, err := m.files.Storage.Get(profile.Driver)
	if err != nil {
		return contracts.Output{}, err
	}
	length := min(files.CloudDriveChunkLimit, row.Size-input.Offset)
	reader, err := openRange(ctx, driver, profile, row.StorageKey, input.Offset, length)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	content := make([]byte, int(length))
	_, readErr := io.ReadFull(reader, content)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	return contracts.Output{Data: map[string]any{"name": row.Name, "size": row.Size, "sha256": row.Sha256,
		"offset": input.Offset, "content": base64.StdEncoding.EncodeToString(content), "eof": input.Offset+length == row.Size}}, nil
}

func openRange(ctx context.Context, driver files.Driver, profile files.Profile, key string, offset, length int64) (io.ReadCloser, error) {
	if ranged, ok := driver.(files.RangeReader); ok {
		return ranged.ReadRange(ctx, profile, key, offset, length)
	}
	reader, err := driver.Read(ctx, profile, key)
	if err != nil {
		return nil, err
	}
	if _, err = io.CopyN(io.Discard, reader, offset); err != nil {
		if closeErr := reader.Close(); closeErr != nil {
			return nil, closeErr
		}
		return nil, err
	}
	return reader, nil
}

package drive

import (
	"context"
	"fmt"
	"strings"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func folderCreateDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.folder.create", Version: "1.0.0",
		Title: "Create folder", Description: "Create a folder in a space the caller can edit.",
		Domain: "drive", Entity: "drive_folder", Permission: "drive:use", Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.folder.created"}, Executor: "go:drive.folder.create", Tags: []string{"drive", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53),
			"parentId": contracts.Int(0, 1<<53), "name": contracts.Str(1, 200)}, "spaceId", "name"),
		Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"), Loggable: []string{"spaceId", "parentId"}}
}

func folderListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.folder.list", Version: "1.0.0",
		Title: "List folders", Description: "List folders in one accessible space and parent.", Domain: "drive", Entity: "drive_folder",
		Permission: "drive:use", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.folder.list", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "parentId": contracts.Int(0, 1<<53)}, "spaceId"),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "parentId": {Type: "integer"},
		}, "id", "name", "parentId")}}, "items"), Loggable: []string{"spaceId", "parentId"}}
}

func folderPathDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.folder.path", Version: "1.0.0",
		Title: "Read folder path", Description: "Read the authorized parent chain of a folder.", Domain: "drive", Entity: "drive_folder",
		Permission: "drive:use", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true,
		Executor: "go:drive.folder.path", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "folderId": contracts.Int(1, 1<<53)}, "spaceId", "folderId"),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "parentId": {Type: "integer"},
		}, "id", "name", "parentId")}}, "items"), Loggable: []string{"spaceId", "folderId"}}
}

func (m *Module) folderPath(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input struct {
		SpaceID  int64 `json:"spaceId"`
		FolderID int64 `json:"folderId"`
	}
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(m.pool).DriveFolderPath(ctx, dbgen.DriveFolderPathParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, FolderID: input.FolderID, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	if len(rows) == 0 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "name": row.Name, "parentId": row.ParentID})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

type folderInput struct {
	ID       int64  `json:"id"`
	SpaceID  int64  `json:"spaceId"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
}

func folderChangeDefinition(action string) contracts.Definition {
	fields := map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "id": contracts.Int(1, 1<<53)}
	required := []string{"spaceId", "id"}
	if action == "rename" {
		fields["name"] = contracts.Str(1, 200)
		required = append(required, "name")
	} else {
		fields["parentId"] = contracts.Int(0, 1<<53)
		required = append(required, "parentId")
	}
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.folder." + action, Version: "1.0.0",
		Title: "Change folder " + action, Description: "Change a folder in an editable space.", Domain: "drive", Entity: "drive_folder",
		Permission: "drive:use", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.folder." + action + "d"}, Executor: "go:drive.folder." + action, Tags: []string{"drive", "write"},
		Input: contracts.Object(fields, required...), Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Loggable: []string{"spaceId", "id"}}
}

func (m *Module) renameFolder(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.changeFolder(ctx, tx, in, "rename")
}

func (m *Module) moveFolder(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.changeFolder(ctx, tx, in, "move")
}

func (m *Module) changeFolder(ctx context.Context, tx pgx.Tx, in contracts.Input, action string) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input folderInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	_, err = q.DriveSpaceWriteLock(ctx, dbgen.DriveSpaceWriteLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	var count int64
	if action == "rename" {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		count, err = q.DriveFolderRename(ctx, dbgen.DriveFolderRenameParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
			ID: input.ID, Name: name, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	} else {
		count, err = q.DriveFolderMove(ctx, dbgen.DriveFolderMoveParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
			ID: input.ID, ParentID: optionalID(input.ParentID), ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	}
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: map[string]any{"id": input.ID}, Events: []contracts.Event{{Type: "drive.folder." + action + "d",
		AggregateType: "drive_folder", AggregateID: fmt.Sprint(input.ID), Payload: map[string]any{"spaceId": input.SpaceID, "folderId": input.ID}}}}, nil
}

func optionalID(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: id > 0} }

func (m *Module) createFolder(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input folderInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	_, err = q.DriveSpaceWriteLock(ctx, dbgen.DriveSpaceWriteLockParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	id, err := q.DriveFolderCreate(ctx, dbgen.DriveFolderCreateParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		ParentID: optionalID(input.ParentID), Name: input.Name, ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: "drive.folder.created",
		AggregateType: "drive_folder", AggregateID: fmt.Sprint(id), Payload: map[string]any{"spaceId": input.SpaceID, "folderId": id}}}}, nil
}

func (m *Module) listFolders(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input folderInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(m.pool).DriveFolderList(ctx, dbgen.DriveFolderListParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, ParentID: optionalID(input.ParentID), ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "name": row.Name, "parentId": row.ParentID})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

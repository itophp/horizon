package drive

import (
	"context"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
)

func usageDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.space.usage", Version: "1.0.0",
		Title: "Read space usage", Description: "Read capacity and reserved bytes in an accessible space.",
		Domain: "drive", Entity: "drive_space", Permission: "drive:use", Risk: contracts.RiskLow,
		Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true, Executor: "go:drive.space.usage",
		Input:  contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53)}, "spaceId"),
		Output: contracts.Object(map[string]*contracts.Schema{"usedBytes": {Type: "integer"}, "pendingBytes": {Type: "integer"}, "quotaBytes": {Type: "integer"}}, "usedBytes", "pendingBytes", "quotaBytes"), Loggable: []string{"spaceId"}}
}

func (m *Module) spaceUsage(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if m.files == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	var input struct {
		SpaceID int64 `json:"spaceId"`
	}
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(m.pool).DriveSpaceUsage(ctx, dbgen.DriveSpaceUsageParams{TenantID: p.TenantID, SpaceID: input.SpaceID,
		ActorID: p.Account.ID, DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	raw, err := m.files.Setting(ctx, p, "drive.quotaBytes")
	if err != nil {
		return contracts.Output{}, err
	}
	quota, err := governance.Number(raw)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"usedBytes": row.UsedBytes, "pendingBytes": row.PendingBytes, "quotaBytes": quota}}, nil
}

package drive

import (
	"context"
	"errors"
	"fmt"

	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func memberListDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.member.list", Version: "1.0.0",
		Title: "List space members", Description: "List grants visible to a space administrator.",
		Domain: "drive", Entity: "drive_member", Permission: "drive:use", Risk: contracts.RiskLow,
		Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true, Executor: "go:drive.member.list", Tags: []string{"drive", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53)}, "spaceId"),
		Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "targetType": {Type: "string"}, "targetId": {Type: "integer"}, "role": {Type: "string"},
		}, "id", "targetType", "targetId", "role")}}, "items"), Loggable: []string{"spaceId"}}
}

func memberAddDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.member.add", Version: "1.0.0",
		Title: "Add space member", Description: "Grant an active account or department a role in a shared space.",
		Domain: "drive", Entity: "drive_member", Permission: "drive:use", Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.member.added"}, Executor: "go:drive.member.add", Tags: []string{"drive", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53),
			"targetType": contracts.Enum("account", "department"), "targetId": contracts.Int(1, 1<<53),
			"role": contracts.Enum("admin", "editor", "viewer")}, "spaceId", "targetType", "targetId", "role"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Loggable: []string{"spaceId", "targetType", "targetId", "role"}}
}

type memberInput struct {
	SpaceID    int64  `json:"spaceId"`
	MemberID   int64  `json:"memberId"`
	TargetType string `json:"targetType"`
	TargetID   int64  `json:"targetId"`
	Role       string `json:"role"`
}

func memberChangeDefinition(action string) contracts.Definition {
	fields := map[string]*contracts.Schema{"spaceId": contracts.Int(1, 1<<53), "memberId": contracts.Int(1, 1<<53)}
	required := []string{"spaceId", "memberId"}
	loggable := []string{"spaceId", "memberId"}
	if action == "change" {
		fields["role"] = contracts.Enum("admin", "editor", "viewer")
		required = append(required, "role")
		loggable = append(loggable, "role")
	}
	return contracts.Definition{Layer: contracts.LayerAtomic, Name: "drive.member." + action, Version: "1.0.0",
		Title: "Change space membership", Description: "Change a shared-space grant while preserving an effective administrator.",
		Domain: "drive", Entity: "drive_member", Permission: "drive:use", Risk: contracts.RiskMedium,
		Approval: contracts.ApprovalAuto, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{"drive.member." + action + "d"}, Executor: "go:drive.member." + action, Tags: []string{"drive", "write"},
		Input: contracts.Object(fields, required...), Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Loggable: loggable}
}

func (m *Module) changeMember(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.modifyMember(ctx, tx, in, "change")
}

func (m *Module) removeMember(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	return m.modifyMember(ctx, tx, in, "remove")
}

func (m *Module) modifyMember(ctx context.Context, tx pgx.Tx, in contracts.Input, action string) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input memberInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockAdmin(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	var count int64
	if action == "change" {
		count, err = q.DriveMemberChangeRole(ctx, dbgen.DriveMemberChangeRoleParams{TenantID: p.TenantID,
			SpaceID: input.SpaceID, MemberID: input.MemberID, Role: input.Role})
	} else {
		count, err = q.DriveMemberRemove(ctx, dbgen.DriveMemberRemoveParams{TenantID: p.TenantID,
			SpaceID: input.SpaceID, MemberID: input.MemberID})
	}
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err = m.requireEffectiveAdmin(ctx, tx, q, p.TenantID, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": input.MemberID}, Events: []contracts.Event{{
		Type: "drive.member." + action + "d", AggregateType: "drive_space", AggregateID: fmt.Sprint(input.SpaceID),
		Payload: map[string]any{"spaceId": input.SpaceID, "memberId": input.MemberID}}}}, nil
}

// requireEffectiveAdmin checks post-change membership inside the transaction
// and rebuilds each candidate's current identity. A grant to a disabled user or
// a user denied drive:use does not satisfy the last-administrator invariant.
func (m *Module) requireEffectiveAdmin(ctx context.Context, tx pgx.Tx, q *dbgen.Queries, tenant string, spaceID int64) error {
	candidates, err := q.DriveAdminCandidates(ctx, dbgen.DriveAdminCandidatesParams{TenantID: tenant, SpaceID: spaceID})
	if err != nil {
		return err
	}
	iam := identity.New(m.pool, false)
	for _, candidate := range candidates {
		principal, err := iam.PrincipalForTaskInTx(ctx, tx, tenant, candidate.AccountID, candidate.DepartmentID)
		if err == nil && principal.Can("drive:use") {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			var identityErr *genidentity.IAMProblem
			if !errors.As(err, &identityErr) || (identityErr.Code != "forbidden" && identityErr.Code != "unauthenticated") {
				return err
			}
		}
	}
	return contracts.Problem("conflict")
}

func (m *Module) listMembers(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input memberInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	rows, err := dbgen.New(m.pool).DriveMemberList(ctx, dbgen.DriveMemberListParams{TenantID: p.TenantID,
		SpaceID: input.SpaceID, ActorID: idValue(p.Account.ID), DepartmentID: idValue(p.Account.DepartmentID)})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		targetType, targetID := "account", row.AccountID.Int64
		if row.DepartmentID.Valid {
			targetType, targetID = "department", row.DepartmentID.Int64
		}
		items = append(items, map[string]any{"id": row.ID, "targetType": targetType, "targetId": targetID, "role": row.Role})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}

func (m *Module) addMember(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if err := requireHuman(ctx); err != nil {
		return contracts.Output{}, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	var input memberInput
	if err = contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err = lockAdmin(ctx, q, p, input.SpaceID); err != nil {
		return contracts.Output{}, err
	}
	var id int64
	switch input.TargetType {
	case "account":
		id, err = q.DriveMemberAddAccount(ctx, dbgen.DriveMemberAddAccountParams{TenantID: p.TenantID, SpaceID: input.SpaceID, AccountID: input.TargetID, Role: input.Role})
	case "department":
		id, err = q.DriveMemberAddDepartment(ctx, dbgen.DriveMemberAddDepartmentParams{TenantID: p.TenantID, SpaceID: input.SpaceID, DepartmentID: input.TargetID, Role: input.Role})
	default:
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if err != nil {
		return contracts.Output{}, memberProblem(err)
	}
	return contracts.Output{Data: map[string]any{"id": id}, Events: []contracts.Event{{Type: "drive.member.added",
		AggregateType: "drive_space", AggregateID: fmt.Sprint(input.SpaceID), Payload: map[string]any{"spaceId": input.SpaceID, "memberId": id}}}}, nil
}

func lockAdmin(ctx context.Context, q *dbgen.Queries, p *identity.Principal, spaceID int64) error {
	actor, department := idValue(p.Account.ID), idValue(p.Account.DepartmentID)
	_, err := q.DriveSharedAdminLock(ctx, dbgen.DriveSharedAdminLockParams{TenantID: p.TenantID, SpaceID: spaceID, ActorID: actor, DepartmentID: department})
	if err != nil {
		return memberProblem(err)
	}
	allowed, err := q.DriveSharedAdminAllowed(ctx, dbgen.DriveSharedAdminAllowedParams{TenantID: p.TenantID, SpaceID: spaceID, ActorID: actor, DepartmentID: department})
	if err != nil {
		return err
	}
	if !allowed {
		return contracts.Problem("not_found")
	}
	return nil
}

func idValue(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func memberProblem(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Problem("not_found")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
		return contracts.Problem("conflict")
	}
	return err
}

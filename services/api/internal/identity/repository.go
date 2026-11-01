package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type Repository struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func newRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: dbgen.New(pool)}
}
func decodeGrants(value []byte) ([]*genidentity.PermissionGrant, error) {
	out := []*genidentity.PermissionGrant{}
	if err := json.Unmarshal(value, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*genidentity.PermissionGrant{}
	}
	return out, nil
}
func timeValue(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return nowString(value.Time)
}

func (r *Repository) mapAccount(ctx context.Context, tenant string, row dbgen.IAMGetAccountRow) (*genidentity.AccountRecord, error) {
	grants, err := decodeGrants(row.Overrides)
	if err != nil {
		return nil, err
	}
	roles, err := r.q.IAMAccountRoles(ctx, dbgen.IAMAccountRolesParams{TenantID: tenant, AccountID: row.ID})
	if err != nil {
		return nil, err
	}
	memberships, err := r.memberships(ctx, tenant, row.ID)
	if err != nil {
		return nil, err
	}
	primary, err := r.department(ctx, Scope{TenantID: tenant, All: true}, row.DepartmentID)
	if err != nil {
		return nil, err
	}
	name := primary.Name
	memberships = append([]*genidentity.DepartmentMembership{{OrganizationID: row.OrganizationID, DepartmentID: row.DepartmentID, DepartmentName: &name, RoleIds: roles, Overrides: grants}}, memberships...)
	return &genidentity.AccountRecord{Memberships: memberships, ID: row.ID, Username: row.Username, FullName: row.FullName, Email: row.Email, Status: row.Status, OrganizationID: row.OrganizationID, DepartmentID: row.DepartmentID, IsAdmin: row.IsAdmin, Overrides: grants, RoleIds: roles, CreatedAt: timeValue(row.CreatedAt), LastLogin: timeValue(row.LastLogin)}, nil
}

func (r *Repository) account(ctx context.Context, scope Scope, id int64) (*genidentity.AccountRecord, error) {
	row, err := r.q.IAMGetAccount(ctx, dbgen.IAMGetAccountParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	return r.mapAccount(ctx, scope.TenantID, row)
}

func (r *Repository) accounts(ctx context.Context, scope Scope) ([]*genidentity.AccountRecord, error) {
	rows, err := r.q.IAMListAccounts(ctx, dbgen.IAMListAccountsParams{TenantID: scope.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	out := make([]*genidentity.AccountRecord, 0, len(rows))
	for _, row := range rows {
		item, err := r.mapAccount(ctx, scope.TenantID, dbgen.IAMGetAccountRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) deleteAccount(ctx context.Context, scope Scope, id int64) error {
	_, err := r.q.IAMDeleteAccount(ctx, dbgen.IAMDeleteAccountParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	return err
}

func (r *Repository) mapOrganization(ctx context.Context, tenant string, row dbgen.IAMGetOrganizationRow) (*genidentity.OrganizationRecord, error) {
	return &genidentity.OrganizationRecord{ID: row.ID, Name: row.Name, Code: row.Code, Kind: row.Kind, ParentID: row.ParentID, Status: row.Status, Description: row.Description, CreatedAt: timeValue(row.CreatedAt)}, nil
}

func (r *Repository) organization(ctx context.Context, scope Scope, id int64) (*genidentity.OrganizationRecord, error) {
	row, err := r.q.IAMGetOrganization(ctx, dbgen.IAMGetOrganizationParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs})
	if err != nil {
		return nil, err
	}
	return r.mapOrganization(ctx, scope.TenantID, row)
}

func (r *Repository) organizations(ctx context.Context, scope Scope) ([]*genidentity.OrganizationRecord, error) {
	rows, err := r.q.IAMListOrganizations(ctx, dbgen.IAMListOrganizationsParams{TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs})
	if err != nil {
		return nil, err
	}
	out := make([]*genidentity.OrganizationRecord, 0, len(rows))
	for _, row := range rows {
		item, err := r.mapOrganization(ctx, scope.TenantID, dbgen.IAMGetOrganizationRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) deleteOrganization(ctx context.Context, scope Scope, id int64) error {
	_, err := r.q.IAMDeleteOrganization(ctx, dbgen.IAMDeleteOrganizationParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs})
	return err
}

func (r *Repository) mapDepartment(ctx context.Context, tenant string, row dbgen.IAMGetDepartmentRow) (*genidentity.DepartmentRecord, error) {
	return &genidentity.DepartmentRecord{ID: row.ID, Name: row.Name, Code: row.Code, OrganizationID: row.OrganizationID, ParentID: row.ParentID, Manager: row.Manager, Status: row.Status, Description: row.Description, CreatedAt: timeValue(row.CreatedAt)}, nil
}

func (r *Repository) department(ctx context.Context, scope Scope, id int64) (*genidentity.DepartmentRecord, error) {
	row, err := r.q.IAMGetDepartment(ctx, dbgen.IAMGetDepartmentParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	return r.mapDepartment(ctx, scope.TenantID, row)
}

func (r *Repository) departments(ctx context.Context, scope Scope) ([]*genidentity.DepartmentRecord, error) {
	rows, err := r.q.IAMListDepartments(ctx, dbgen.IAMListDepartmentsParams{TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	out := make([]*genidentity.DepartmentRecord, 0, len(rows))
	for _, row := range rows {
		item, err := r.mapDepartment(ctx, scope.TenantID, dbgen.IAMGetDepartmentRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) deleteDepartment(ctx context.Context, scope Scope, id int64) error {
	_, err := r.q.IAMDeleteDepartment(ctx, dbgen.IAMDeleteDepartmentParams{ID: id, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	return err
}

func (r *Repository) mapRole(ctx context.Context, tenant string, row dbgen.IAMGetRoleRow) (*genidentity.RoleRecord, error) {
	grants, err := decodeGrants(row.Grants)
	if err != nil {
		return nil, err
	}
	count, err := r.q.IAMRoleMemberCount(ctx, dbgen.IAMRoleMemberCountParams{TenantID: tenant, RoleID: row.ID})
	if err != nil {
		return nil, err
	}
	return &genidentity.RoleRecord{ID: row.ID, Name: row.Name, Code: row.Code, Status: row.Status, Description: row.Description, Grants: grants, MemberCount: count, CreatedAt: timeValue(row.CreatedAt)}, nil
}

func (r *Repository) role(ctx context.Context, scope Scope, id int64) (*genidentity.RoleRecord, error) {
	row, err := r.q.IAMGetRole(ctx, dbgen.IAMGetRoleParams{ID: id, TenantID: scope.TenantID, AllData: scope.All})
	if err != nil {
		return nil, err
	}
	return r.mapRole(ctx, scope.TenantID, row)
}

func (r *Repository) roles(ctx context.Context, scope Scope) ([]*genidentity.RoleRecord, error) {
	rows, err := r.q.IAMListRoles(ctx, dbgen.IAMListRolesParams{TenantID: scope.TenantID, AllData: scope.All})
	if err != nil {
		return nil, err
	}
	out := make([]*genidentity.RoleRecord, 0, len(rows))
	for _, row := range rows {
		item, err := r.mapRole(ctx, scope.TenantID, dbgen.IAMGetRoleRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) deleteRole(ctx context.Context, scope Scope, id int64) error {
	_, err := r.q.IAMDeleteRole(ctx, dbgen.IAMDeleteRoleParams{ID: id, TenantID: scope.TenantID, AllData: scope.All})
	return err
}

func (r *Repository) departmentTree(ctx context.Context, tenant string, id int64) ([]int64, error) {
	return r.q.IAMDepartmentTree(ctx, dbgen.IAMDepartmentTreeParams{TenantID: tenant, ID: id})
}
func (r *Repository) organizationTree(ctx context.Context, tenant string, id int64) ([]int64, error) {
	return r.q.IAMOrganizationTree(ctx, dbgen.IAMOrganizationTreeParams{TenantID: tenant, ID: id})
}
func (r *Repository) rootAnchors(ctx context.Context, tenant string) ([]dbgen.IAMRootAnchorsRow, error) {
	return r.q.IAMRootAnchors(ctx, tenant)
}
func (r *Repository) session(ctx context.Context, hash string) (dbgen.IAMSessionRow, error) {
	return r.q.IAMSession(ctx, hash)
}
func (r *Repository) deleteSession(ctx context.Context, hash string) error {
	return r.q.IAMDeleteSession(ctx, hash)
}
func (r *Repository) roleGrants(ctx context.Context, tenant string, id int64) ([]*genidentity.PermissionGrant, error) {
	rows, err := r.q.IAMRoleGrants(ctx, dbgen.IAMRoleGrantsParams{TenantID: tenant, AccountID: id})
	if err != nil {
		return nil, err
	}
	out := []*genidentity.PermissionGrant{}
	for _, value := range rows {
		grants, err := decodeGrants(value)
		if err != nil {
			return nil, err
		}
		out = append(out, grants...)
	}
	return out, nil
}
func (r *Repository) loginAccount(ctx context.Context, username string) (dbgen.IAMLoginRow, error) {
	return r.q.IAMLogin(ctx, username)
}
func (r *Repository) loginBlocked(ctx context.Context, key string) (bool, error) {
	row, err := r.q.IAMLoginThrottle(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Failures >= 5 && row.LockedUntil.Time.After(time.Now()), nil
}
func (r *Repository) loginFailure(ctx context.Context, key string) error {
	return r.q.IAMLoginFailure(ctx, key)
}
func (r *Repository) loginSuccess(ctx context.Context, tenant string, id int64, key string) error {
	if err := r.q.IAMTouchLogin(ctx, dbgen.IAMTouchLoginParams{TenantID: tenant, ID: id}); err != nil {
		return err
	}
	return r.q.IAMLoginClear(ctx, key)
}
func (r *Repository) createSession(ctx context.Context, tenant string, id int64, hash, csrf string, minutes int) error {
	return r.q.IAMCreateSession(ctx, dbgen.IAMCreateSessionParams{TenantID: tenant, AccountID: id, TokenHash: hash, CsrfToken: csrf, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(minutes) * time.Minute), Valid: true}})
}

func (r *Repository) createOrganization(ctx context.Context, scope Scope, payload *OrganizationInput) (*genidentity.OrganizationRecord, error) {
	idResult, err := r.q.IAMCreateOrganization(ctx, dbgen.IAMCreateOrganizationParams{Name: payload.Name, Code: payload.Code, Kind: payload.Kind, ParentID: payload.ParentID, Status: payload.Status, Description: payload.Description, TenantID: scope.TenantID})
	if err != nil {
		return nil, err
	}
	return r.organization(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) updateOrganization(ctx context.Context, scope Scope, id int64, payload *OrganizationInput) (*genidentity.OrganizationRecord, error) {
	idResult, err := r.q.IAMUpdateOrganization(ctx, dbgen.IAMUpdateOrganizationParams{Name: payload.Name, Code: payload.Code, Kind: payload.Kind, ParentID: payload.ParentID, Status: payload.Status, Description: payload.Description, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs, ID: id})
	if err != nil {
		return nil, err
	}
	return r.organization(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) createDepartment(ctx context.Context, scope Scope, payload *DepartmentInput) (*genidentity.DepartmentRecord, error) {
	idResult, err := r.q.IAMCreateDepartment(ctx, dbgen.IAMCreateDepartmentParams{Name: payload.Name, Code: payload.Code, OrganizationID: payload.OrganizationID, ParentID: payload.ParentID, Manager: payload.Manager, Status: payload.Status, Description: payload.Description, TenantID: scope.TenantID})
	if err != nil {
		return nil, err
	}
	return r.department(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) updateDepartment(ctx context.Context, scope Scope, id int64, payload *DepartmentInput) (*genidentity.DepartmentRecord, error) {
	idResult, err := r.q.IAMUpdateDepartment(ctx, dbgen.IAMUpdateDepartmentParams{Name: payload.Name, Code: payload.Code, OrganizationID: payload.OrganizationID, ParentID: payload.ParentID, Manager: payload.Manager, Status: payload.Status, Description: payload.Description, TenantID: scope.TenantID, AllData: scope.All, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs, ID: id})
	if err != nil {
		return nil, err
	}
	return r.department(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) createRole(ctx context.Context, scope Scope, payload *RoleInput) (*genidentity.RoleRecord, error) {
	grants, err := json.Marshal(payload.Grants)
	if err != nil {
		return nil, err
	}
	idResult, err := r.q.IAMCreateRole(ctx, dbgen.IAMCreateRoleParams{Name: payload.Name, Code: payload.Code, Description: payload.Description, Status: payload.Status, Grants: grants, TenantID: scope.TenantID})
	if err != nil {
		return nil, err
	}
	return r.role(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) updateRole(ctx context.Context, scope Scope, id int64, payload *RoleInput) (*genidentity.RoleRecord, error) {
	grants, err := json.Marshal(payload.Grants)
	if err != nil {
		return nil, err
	}
	idResult, err := r.q.IAMUpdateRole(ctx, dbgen.IAMUpdateRoleParams{Name: payload.Name, Code: payload.Code, Description: payload.Description, Status: payload.Status, Grants: grants, TenantID: scope.TenantID, AllData: scope.All, ID: id})
	if err != nil {
		return nil, err
	}
	return r.role(ctx, Scope{TenantID: scope.TenantID, All: true}, idResult)
}

func (r *Repository) writeAccount(ctx context.Context, scope Scope, id int64, payload *AccountInput, passwordHash string) (*genidentity.AccountRecord, error) {
	grants, err := json.Marshal(payload.Overrides)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("identity rollback failed", "error", err)
		}
	}()
	q := r.q.WithTx(tx)
	if id == 0 {
		id, err = q.IAMCreateAccount(ctx, dbgen.IAMCreateAccountParams{TenantID: scope.TenantID, Username: payload.Username, FullName: payload.FullName, Email: payload.Email, Status: payload.Status, OrganizationID: payload.OrganizationID, DepartmentID: payload.DepartmentID, Overrides: grants, PasswordHash: passwordHash})
	} else {
		id, err = q.IAMUpdateAccount(ctx, dbgen.IAMUpdateAccountParams{TenantID: scope.TenantID, ID: id, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs, Username: payload.Username, FullName: payload.FullName, Email: payload.Email, Status: payload.Status, OrganizationID: payload.OrganizationID, DepartmentID: payload.DepartmentID, Overrides: grants})
		if err == nil && passwordHash != "" {
			err = q.IAMSetPassword(ctx, dbgen.IAMSetPasswordParams{TenantID: scope.TenantID, ID: id, PasswordHash: passwordHash})
		}
	}
	if err != nil {
		return nil, err
	}
	if err = q.IAMClearAccountRoles(ctx, dbgen.IAMClearAccountRolesParams{TenantID: scope.TenantID, AccountID: id}); err != nil {
		return nil, err
	}
	for _, role := range payload.RoleIds {
		if err = q.IAMAssignRole(ctx, dbgen.IAMAssignRoleParams{TenantID: scope.TenantID, AccountID: id, RoleID: role}); err != nil {
			return nil, err
		}
	}
	if payload.DepartmentAccess != nil {
		if err = q.IAMClearMemberships(ctx, dbgen.IAMClearMembershipsParams{TenantID: scope.TenantID, AccountID: id}); err != nil {
			return nil, err
		}
		for _, m := range payload.DepartmentAccess.Memberships {
			raw, marshalErr := json.Marshal(m.Overrides)
			if marshalErr != nil {
				return nil, marshalErr
			}
			if err = q.IAMCreateMembership(ctx, dbgen.IAMCreateMembershipParams{TenantID: scope.TenantID, AccountID: id, OrganizationID: m.OrganizationID, DepartmentID: m.DepartmentID, Overrides: raw}); err != nil {
				return nil, err
			}
			for _, role := range m.RoleIds {
				if err = q.IAMAssignMembershipRole(ctx, dbgen.IAMAssignMembershipRoleParams{TenantID: scope.TenantID, AccountID: id, DepartmentID: m.DepartmentID, RoleID: role}); err != nil {
					return nil, err
				}
			}
		}
	}
	if err = q.IAMInvalidateDetachedTemporary(ctx, dbgen.IAMInvalidateDetachedTemporaryParams{TenantID: scope.TenantID, ApplicantID: id}); err != nil {
		return nil, err
	}
	if passwordHash != "" || payload.Status != "active" {
		if err = q.IAMRevokeSessions(ctx, dbgen.IAMRevokeSessionsParams{TenantID: scope.TenantID, AccountID: id}); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.account(ctx, Scope{TenantID: scope.TenantID, All: true}, id)
}

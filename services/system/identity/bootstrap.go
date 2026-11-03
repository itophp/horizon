package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"regexp"
	"strings"
)

// Bootstrap creates a workspace's first administrator under a transaction lock. It never replaces existing credentials.
func Bootstrap(ctx context.Context, pool *pgxpool.Pool, tenant, username, password string) (*genidentity.AccountRecord, error) {
	if tenant == "" || !regexp.MustCompile(`^[A-Za-z0-9_.@-]{3,80}$`).MatchString(username) {
		return nil, errors.New("invalid bootstrap identity")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("bootstrap rollback failed", "error", err)
		}
	}()
	q := dbgen.New(tx)
	if err = q.IAMBootstrapLock(ctx, tenant); err != nil {
		return nil, err
	}
	count, err := q.IAMCountAccounts(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("workspace already has accounts; bootstrap refused")
	}
	enterprise, err := q.IAMCreateOrganization(ctx, dbgen.IAMCreateOrganizationParams{TenantID: tenant, Name: "Horizon Group", Code: "horizon-group", Kind: "enterprise", Status: "active", Description: ""})
	if err != nil {
		return nil, err
	}
	company, err := q.IAMCreateOrganization(ctx, dbgen.IAMCreateOrganizationParams{TenantID: tenant, Name: "Horizon Technology", Code: "horizon-tech", Kind: "company", ParentID: enterprise, Status: "active", Description: ""})
	if err != nil {
		return nil, err
	}
	department, err := q.IAMCreateDepartment(ctx, dbgen.IAMCreateDepartmentParams{TenantID: tenant, OrganizationID: company, Name: "Administration", Code: "administration", Status: "active"})
	if err != nil {
		return nil, err
	}
	if _, err = q.IAMCreateDepartment(ctx, dbgen.IAMCreateDepartmentParams{TenantID: tenant, OrganizationID: company, Name: "Engineering", Code: "engineering", Status: "active"}); err != nil {
		return nil, err
	}
	all := []*genidentity.PermissionGrant{}
	read := []*genidentity.PermissionGrant{}
	for _, permission := range Permissions {
		if permission == "temporary:approve" || (permission != "access:manage" && !strings.HasPrefix(permission, "accounts:") && !strings.HasPrefix(permission, "organizations:") && !strings.HasPrefix(permission, "departments:") && !strings.HasPrefix(permission, "roles:")) {
			continue
		}
		g := &genidentity.PermissionGrant{Permission: permission, Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}
		all = append(all, g)
		if seedViewerPermission(permission) {
			read = append(read, &genidentity.PermissionGrant{Permission: permission, Effect: "allow", Scope: "DEPARTMENT", DepartmentIds: []int64{}})
		}
	}
	allJSON, err := json.Marshal(all)
	if err != nil {
		return nil, err
	}
	readJSON, err := json.Marshal(read)
	if err != nil {
		return nil, err
	}
	adminRole, err := q.IAMCreateRole(ctx, dbgen.IAMCreateRoleParams{TenantID: tenant, Name: "System administrator", Code: "system-admin", Status: "active", Description: "Full workspace administration", Grants: allJSON})
	if err != nil {
		return nil, err
	}
	if _, err = q.IAMCreateRole(ctx, dbgen.IAMCreateRoleParams{TenantID: tenant, Name: "Department viewer", Code: "department-viewer", Status: "active", Description: "Read-only access within the assigned department", Grants: readJSON}); err != nil {
		return nil, err
	}
	id, err := q.IAMCreateAccount(ctx, dbgen.IAMCreateAccountParams{TenantID: tenant, Username: strings.ToLower(username), FullName: "Administrator", Status: "active", OrganizationID: company, DepartmentID: department, PasswordHash: hash, Overrides: []byte("[]")})
	if err != nil {
		return nil, err
	}
	if err = q.IAMMarkAdmin(ctx, dbgen.IAMMarkAdminParams{TenantID: tenant, ID: id}); err != nil {
		return nil, err
	}
	if err = q.IAMAssignRole(ctx, dbgen.IAMAssignRoleParams{TenantID: tenant, AccountID: id, RoleID: adminRole}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return newRepository(pool).account(ctx, Scope{TenantID: tenant, All: true}, id)
}

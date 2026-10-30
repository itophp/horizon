package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repository) memberships(ctx context.Context, tenant string, id int64) ([]*genidentity.DepartmentMembership, error) {
	rows, err := r.q.IAMMemberships(ctx, dbgen.IAMMembershipsParams{TenantID: tenant, AccountID: id})
	if err != nil {
		return nil, err
	}
	out := []*genidentity.DepartmentMembership{}
	for _, row := range rows {
		grants, err := decodeGrants(row.Overrides)
		if err != nil {
			return nil, err
		}
		roles, err := r.q.IAMMembershipRoles(ctx, dbgen.IAMMembershipRolesParams{TenantID: tenant, AccountID: id, DepartmentID: row.DepartmentID})
		if err != nil {
			return nil, err
		}
		name := row.DepartmentName
		out = append(out, &genidentity.DepartmentMembership{OrganizationID: row.OrganizationID, DepartmentID: row.DepartmentID, DepartmentName: &name, RoleIds: roles, Overrides: grants})
	}
	return out, nil
}

// Build the authorization identity from exactly one verified membership, never a union.
func (s *Service) departmentAccount(ctx context.Context, tenant string, account *genidentity.AccountRecord, dept int64) (*genidentity.AccountRecord, error) {
	for _, m := range account.Memberships {
		if m.DepartmentID != dept {
			continue
		}
		if _, err := s.repo.q.IAMActivePlacement(ctx, dbgen.IAMActivePlacementParams{TenantID: tenant, ID: dept, OrganizationID: m.OrganizationID}); err != nil {
			return nil, problem("unauthenticated")
		}
		reference := *account
		reference.DepartmentID = m.DepartmentID
		reference.OrganizationID = m.OrganizationID
		reference.RoleIds = m.RoleIds
		reference.Overrides = m.Overrides
		// Built-in administrator authority belongs to its protected primary membership.
		reference.IsAdmin = account.IsAdmin && dept == account.DepartmentID
		return &reference, nil
	}
	return nil, problem("unauthenticated")
}
func (s *Service) departmentGrants(ctx context.Context, tenant string, account *genidentity.AccountRecord) ([]*genidentity.PermissionGrant, error) {
	primary, err := s.repo.account(ctx, Scope{TenantID: tenant, All: true}, account.ID)
	if err != nil {
		return nil, err
	}
	if primary.DepartmentID == account.DepartmentID {
		return s.repo.roleGrants(ctx, tenant, account.ID)
	}
	rows, err := s.repo.q.IAMMembershipRoleGrants(ctx, dbgen.IAMMembershipRoleGrantsParams{TenantID: tenant, AccountID: account.ID, DepartmentID: account.DepartmentID})
	if err != nil {
		return nil, err
	}
	out := []*genidentity.PermissionGrant{}
	for _, raw := range rows {
		grants, err := decodeGrants(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, grants...)
	}
	return out, nil
}
func (s *Service) SwitchDepartment(ctx context.Context, payload *genidentity.SwitchDepartmentPayload) (*genidentity.SessionProfile, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	primary, err := s.repo.account(ctx, tenantScope(p), p.Account.ID)
	if err != nil {
		return nil, safe(err)
	}
	account, err := s.departmentAccount(ctx, p.TenantID, primary, payload.DepartmentID)
	if err != nil {
		return nil, problem("forbidden")
	}
	grants, err := s.departmentGrants(ctx, p.TenantID, account)
	if err != nil {
		return nil, safe(err)
	}
	next, err := NewPrincipal(account, p.TenantID, MergeGrants(grants, account.Overrides))
	if err != nil {
		return nil, safe(err)
	}
	if err = s.loadTemporary(ctx, next); err != nil {
		return nil, err
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, safe(err)
	}
	if _, err = s.repo.q.IAMSwitchDepartment(ctx, dbgen.IAMSwitchDepartmentParams{TokenHash: p.TokenHash, ActiveDepartmentID: pgtype.Int8{Int64: payload.DepartmentID, Valid: true}, CsrfToken: csrf}); err != nil {
		return nil, safe(err)
	}
	next.CSRF = csrf
	next.TokenHash = p.TokenHash
	return profile(next), nil
}

// Membership replacement is a durable authorization change and requires formal authority
// over every old and new placement, including account-wide changes from a secondary scope.
func (s *Service) validateMembershipChange(ctx context.Context, p *Principal, scope Scope, id int64, payload *genidentity.CreateAccountPayload, old *genidentity.AccountRecord) error {
	if payload.DepartmentAccess == nil || (old == nil && len(payload.DepartmentAccess.Memberships) == 0) {
		return nil
	}
	candidates := payload.DepartmentAccess.Memberships
	if old != nil && sameMemberships(old, candidates, payload.DepartmentID) {
		return nil
	}
	if !p.Can("access:manage") {
		return problem("forbidden")
	}
	base, err := p.permanent()
	if err != nil {
		return problem("forbidden")
	}
	permission := "accounts:create"
	if id != 0 {
		permission = "accounts:update"
	}
	formal, err := s.scope(ctx, base, permission)
	if err != nil {
		return problem("forbidden")
	}
	check := func(org, dept int64) error {
		if !formal.All && !contains(formal.OrgIDs, org) && !contains(formal.DeptIDs, dept) {
			return problem("forbidden")
		}
		return s.validatePlacement(ctx, p, scope, org, dept, 0)
	}
	if old != nil {
		for _, m := range old.Memberships {
			// Removing an inactive affiliation still needs formal authority, but must not
			// require that old affiliation to be active.
			if !formal.All && !contains(formal.OrgIDs, m.OrganizationID) && !contains(formal.DeptIDs, m.DepartmentID) {
				return problem("forbidden")
			}
		}
	}
	if err = check(payload.OrganizationID, payload.DepartmentID); err != nil {
		return err
	}
	seen := map[int64]bool{payload.DepartmentID: true}
	for _, m := range candidates {
		if m == nil || seen[m.DepartmentID] {
			return problem("invalid")
		}
		seen[m.DepartmentID] = true
		if err = check(m.OrganizationID, m.DepartmentID); err != nil {
			return err
		}
		if err = s.validateGrants(ctx, p, m.Overrides, false); err != nil {
			return err
		}
		roles := map[int64]bool{}
		for _, role := range m.RoleIds {
			if roles[role] {
				return problem("invalid")
			}
			roles[role] = true
			if _, err = s.repo.role(ctx, tenantScope(p), role); err != nil {
				return problem("invalid")
			}
		}
	}
	return nil
}
func sameMemberships(old *genidentity.AccountRecord, additional []*genidentity.DepartmentMembership, primary int64) bool {
	current := []*genidentity.DepartmentMembership{}
	for _, m := range old.Memberships {
		if m.DepartmentID != primary {
			copy := *m
			copy.DepartmentName = nil
			current = append(current, &copy)
		}
	}
	input := []*genidentity.DepartmentMembership{}
	for _, m := range additional {
		if m == nil {
			return false
		}
		copy := *m
		copy.DepartmentName = nil
		input = append(input, &copy)
	}
	return sameJSON(current, input)
}

func (s *Service) allMembershipAuthority(ctx context.Context, p *Principal, permission string, account *genidentity.AccountRecord) error {
	base, err := p.permanent()
	if err != nil {
		return problem("forbidden")
	}
	scope, err := s.scope(ctx, base, permission)
	if err != nil {
		return problem("forbidden")
	}
	for _, m := range account.Memberships {
		if !scope.All && !contains(scope.OrgIDs, m.OrganizationID) && !contains(scope.DeptIDs, m.DepartmentID) {
			return problem("forbidden")
		}
	}
	return nil
}

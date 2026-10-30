package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"context"
	"net/mail"
	"strings"
)

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func placementAllowed(scope Scope, org, dept, id int64) bool {
	return scope.All || contains(scope.OrgIDs, org) || contains(scope.DeptIDs, dept) || (id != 0 && scope.OwnID == id)
}
func (s *Service) validatePlacement(ctx context.Context, p *Principal, scope Scope, orgID, deptID, id int64) error {
	if !placementAllowed(scope, orgID, deptID, id) {
		return problem("forbidden")
	}
	org, err := s.repo.organization(ctx, tenantScope(p), orgID)
	if err != nil {
		return problem("invalid")
	}
	if org.Kind != "company" || org.Status != "active" {
		return problem("invalid")
	}
	dept, err := s.repo.department(ctx, tenantScope(p), deptID)
	if err != nil {
		return problem("invalid")
	}
	if dept.OrganizationID != orgID || dept.Status != "active" {
		return problem("invalid")
	}
	return nil
}
func (s *Service) validateAccount(ctx context.Context, p *Principal, scope Scope, id int64, payload *genidentity.CreateAccountPayload) error {
	if err := s.validatePlacement(ctx, p, scope, payload.OrganizationID, payload.DepartmentID, id); err != nil {
		return err
	}
	if payload.Email != "" {
		if _, err := mail.ParseAddress(payload.Email); err != nil {
			return problem("invalid")
		}
	}
	if err := s.validateGrants(ctx, p, payload.Overrides, false); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, role := range payload.RoleIds {
		if seen[role] {
			return problem("invalid")
		}
		seen[role] = true
		if _, err := s.repo.role(ctx, tenantScope(p), role); err != nil {
			return problem("invalid")
		}
	}
	return nil
}

// Temporary business operations cannot create lasting identity or authorization changes.
func (s *Service) permanentAccountAuthority(ctx context.Context, p *Principal, permission string, id, org, dept int64) error {
	base, err := p.permanent()
	if err != nil {
		return problem("forbidden")
	}
	scope, err := s.scope(ctx, base, permission)
	if err != nil {
		return problem("forbidden")
	}
	if id != 0 {
		old, err := s.repo.account(ctx, scope, id)
		if err != nil {
			return problem("forbidden")
		}
		if (old.OrganizationID != org || old.DepartmentID != dept) && !scope.All && !contains(scope.OrgIDs, org) && !contains(scope.DeptIDs, dept) {
			return problem("forbidden")
		}
	}
	return s.validatePlacement(ctx, base, scope, org, dept, id)
}
func (s *Service) permanentStructureAuthority(ctx context.Context, p *Principal, permission string, id, parent, org int64) error {
	base, err := p.permanent()
	if err != nil {
		return problem("forbidden")
	}
	scope, err := s.scope(ctx, base, permission)
	if err != nil {
		return problem("forbidden")
	}
	if permission == "departments:update" {
		old, err := s.repo.department(ctx, scope, id)
		if err != nil {
			return problem("forbidden")
		}
		if old.ParentID != parent && !scope.All && !contains(scope.OrgIDs, org) && !contains(scope.DeptIDs, parent) {
			return problem("forbidden")
		}
	} else {
		old, err := s.repo.organization(ctx, scope, id)
		if err != nil {
			return problem("forbidden")
		}
		if old.ParentID != parent && !scope.All {
			return problem("forbidden")
		}
	}
	return nil
}
func (s *Service) CreateAccount(ctx context.Context, payload *genidentity.CreateAccountPayload) (*genidentity.AccountRecord, error) {
	p, scope, err := s.authorized(ctx, "accounts:create")
	if err != nil {
		return nil, err
	}
	if (len(payload.RoleIds) > 0 || len(payload.Overrides) > 0) && !p.Can("access:manage") {
		return nil, problem("forbidden")
	}
	if len(payload.RoleIds) > 0 || len(payload.Overrides) > 0 {
		if err = s.permanentAccountAuthority(ctx, p, "accounts:create", 0, payload.OrganizationID, payload.DepartmentID); err != nil {
			return nil, err
		}
	}
	if err = s.validateMembershipChange(ctx, p, scope, 0, payload, nil); err != nil {
		return nil, err
	}
	if err = s.validateAccount(ctx, p, scope, 0, payload); err != nil {
		return nil, err
	}
	if err = s.validatePolicyPassword(ctx, p, payload.Password); err != nil {
		return nil, err
	}
	hash, err := HashPassword(payload.Password)
	if err != nil {
		return nil, problem("invalid")
	}
	payload.Username = strings.ToLower(strings.TrimSpace(payload.Username))
	record, err := s.repo.writeAccount(ctx, scope, 0, payload, hash)
	return record, safe(err)
}
func (s *Service) UpdateAccount(ctx context.Context, payload *genidentity.UpdateAccountPayload) (*genidentity.AccountRecord, error) {
	p, scope, err := s.authorized(ctx, "accounts:update")
	if err != nil {
		return nil, err
	}
	old, err := s.repo.account(ctx, scope, payload.ID)
	if err != nil {
		return nil, safe(err)
	}
	if payload.DepartmentAccess == nil && old.DepartmentID != payload.DepartmentID {
		for _, m := range old.Memberships {
			if m.DepartmentID == payload.DepartmentID {
				return nil, problem("invalid")
			}
		}
	}
	if old.IsAdmin && (old.ID != p.Account.ID || payload.Status != "active") {
		return nil, problem("forbidden")
	}
	if !p.Can("access:manage") && (!sameJSON(old.RoleIds, payload.RoleIds) || !sameJSON(old.Overrides, payload.Overrides)) {
		return nil, problem("forbidden")
	}
	sensitive := !sameJSON(old.RoleIds, payload.RoleIds) || !sameJSON(old.Overrides, payload.Overrides) || old.OrganizationID != payload.OrganizationID || old.DepartmentID != payload.DepartmentID || old.Username != strings.ToLower(strings.TrimSpace(payload.Username)) || old.Email != payload.Email || old.Status != payload.Status || payload.Password != ""
	if sensitive {
		if err = s.permanentAccountAuthority(ctx, p, "accounts:update", payload.ID, payload.OrganizationID, payload.DepartmentID); err != nil {
			return nil, err
		}
	}
	// An OWN grant never authorizes moving one's account into another company/department.
	if !scope.All && !contains(scope.OrgIDs, payload.OrganizationID) && !contains(scope.DeptIDs, payload.DepartmentID) && (old.OrganizationID != payload.OrganizationID || old.DepartmentID != payload.DepartmentID) {
		return nil, problem("forbidden")
	}
	write := &genidentity.CreateAccountPayload{Username: strings.ToLower(strings.TrimSpace(payload.Username)), FullName: payload.FullName, Email: payload.Email, Status: payload.Status, OrganizationID: payload.OrganizationID, DepartmentID: payload.DepartmentID, RoleIds: payload.RoleIds, Overrides: payload.Overrides, Password: payload.Password, DepartmentAccess: payload.DepartmentAccess}
	if err = s.validateMembershipChange(ctx, p, scope, payload.ID, write, old); err != nil {
		return nil, err
	}
	if sensitive && len(old.Memberships) > 1 {
		if err = s.allMembershipAuthority(ctx, p, "accounts:update", old); err != nil {
			return nil, err
		}
	}
	if old.IsAdmin && (old.DepartmentID != payload.DepartmentID || old.OrganizationID != payload.OrganizationID) {
		return nil, problem("forbidden")
	}
	if err = s.validateAccount(ctx, p, scope, payload.ID, write); err != nil {
		return nil, err
	}
	hash := ""
	if payload.Password != "" {
		if err = s.validatePolicyPassword(ctx, p, payload.Password); err != nil {
			return nil, err
		}
		hash, err = HashPassword(payload.Password)
		if err != nil {
			return nil, problem("invalid")
		}
	}
	record, err := s.repo.writeAccount(ctx, scope, payload.ID, write, hash)
	return record, safe(err)
}
func (s *Service) DeleteAccount(ctx context.Context, payload *genidentity.DeleteAccountPayload) error {
	p, scope, err := s.authorized(ctx, "accounts:delete")
	if err != nil {
		return err
	}
	record, err := s.repo.account(ctx, scope, payload.ID)
	if err != nil {
		return safe(err)
	}
	if record.IsAdmin || record.ID == p.Account.ID {
		return problem("conflict")
	}
	if len(record.Memberships) > 1 {
		if err = s.allMembershipAuthority(ctx, p, "accounts:delete", record); err != nil {
			return err
		}
	}
	return safe(s.repo.deleteAccount(ctx, scope, payload.ID))
}
func (s *Service) validateOrg(ctx context.Context, p *Principal, id int64, kind, status string, parentID int64) error {
	if kind == "enterprise" && parentID != 0 {
		return problem("invalid")
	}
	if kind == "company" {
		parent, err := s.repo.organization(ctx, tenantScope(p), parentID)
		if err != nil || parent.Kind != "enterprise" || parent.Status != "active" {
			return problem("invalid")
		}
	}
	if status != "active" && id > 0 {
		anchors, err := s.repo.rootAnchors(ctx, p.TenantID)
		if err != nil {
			return safe(err)
		}
		tree, err := s.repo.organizationTree(ctx, p.TenantID, id)
		if err != nil {
			return safe(err)
		}
		for _, anchor := range anchors {
			if contains(tree, anchor.OrganizationID) {
				return problem("conflict")
			}
		}
	}
	return nil
}
func (s *Service) CreateOrganization(ctx context.Context, payload *genidentity.CreateOrganizationPayload) (*genidentity.OrganizationRecord, error) {
	p, scope, err := s.authorized(ctx, "organizations:create")
	if err != nil {
		return nil, err
	}
	if !scope.All {
		return nil, problem("forbidden")
	}
	if err = s.validateOrg(ctx, p, 0, payload.Kind, payload.Status, payload.ParentID); err != nil {
		return nil, err
	}
	record, err := s.repo.createOrganization(ctx, scope, payload)
	return record, safe(err)
}
func (s *Service) UpdateOrganization(ctx context.Context, payload *genidentity.UpdateOrganizationPayload) (*genidentity.OrganizationRecord, error) {
	p, scope, err := s.authorized(ctx, "organizations:update")
	if err != nil {
		return nil, err
	}
	old, err := s.repo.organization(ctx, scope, payload.ID)
	if err != nil {
		return nil, safe(err)
	}
	if old.ParentID != payload.ParentID || old.Status != payload.Status {
		if err = s.permanentStructureAuthority(ctx, p, "organizations:update", payload.ID, payload.ParentID, 0); err != nil {
			return nil, err
		}
	}
	if old.Kind != payload.Kind {
		return nil, problem("invalid")
	}
	if !scope.All && old.ParentID != payload.ParentID {
		return nil, problem("forbidden")
	}
	if err = s.validateOrg(ctx, p, payload.ID, payload.Kind, payload.Status, payload.ParentID); err != nil {
		return nil, err
	}
	record, err := s.repo.updateOrganization(ctx, scope, payload.ID, payload)
	return record, safe(err)
}
func (s *Service) DeleteOrganization(ctx context.Context, payload *genidentity.DeleteOrganizationPayload) error {
	_, scope, err := s.authorized(ctx, "organizations:delete")
	if err != nil {
		return err
	}
	return safe(s.repo.deleteOrganization(ctx, scope, payload.ID))
}
func (s *Service) validateDepartment(ctx context.Context, p *Principal, id, org, parent int64, status string) error {
	company, err := s.repo.organization(ctx, tenantScope(p), org)
	if err != nil || company.Kind != "company" || company.Status != "active" {
		return problem("invalid")
	}
	if parent > 0 {
		record, err := s.repo.department(ctx, tenantScope(p), parent)
		if err != nil || record.OrganizationID != org || record.Status != "active" {
			return problem("invalid")
		}
		if id > 0 {
			tree, err := s.repo.departmentTree(ctx, p.TenantID, id)
			if err != nil {
				return safe(err)
			}
			if contains(tree, parent) {
				return problem("invalid")
			}
		}
	}
	if status != "active" && id > 0 {
		anchors, err := s.repo.rootAnchors(ctx, p.TenantID)
		if err != nil {
			return safe(err)
		}
		for _, anchor := range anchors {
			if anchor.DepartmentID == id {
				return problem("conflict")
			}
		}
	}
	return nil
}
func (s *Service) CreateDepartment(ctx context.Context, payload *genidentity.CreateDepartmentPayload) (*genidentity.DepartmentRecord, error) {
	p, scope, err := s.authorized(ctx, "departments:create")
	if err != nil {
		return nil, err
	}
	if !scope.All && !contains(scope.OrgIDs, payload.OrganizationID) && !contains(scope.DeptIDs, payload.ParentID) {
		return nil, problem("forbidden")
	}
	if err = s.validateDepartment(ctx, p, 0, payload.OrganizationID, payload.ParentID, payload.Status); err != nil {
		return nil, err
	}
	record, err := s.repo.createDepartment(ctx, scope, payload)
	return record, safe(err)
}
func (s *Service) UpdateDepartment(ctx context.Context, payload *genidentity.UpdateDepartmentPayload) (*genidentity.DepartmentRecord, error) {
	p, scope, err := s.authorized(ctx, "departments:update")
	if err != nil {
		return nil, err
	}
	old, err := s.repo.department(ctx, scope, payload.ID)
	if err != nil {
		return nil, safe(err)
	}
	if old.ParentID != payload.ParentID || old.Status != payload.Status {
		if err = s.permanentStructureAuthority(ctx, p, "departments:update", payload.ID, payload.ParentID, payload.OrganizationID); err != nil {
			return nil, err
		}
	}
	if old.OrganizationID != payload.OrganizationID {
		return nil, problem("invalid")
	}
	if old.ParentID != payload.ParentID && !scope.All && !contains(scope.OrgIDs, payload.OrganizationID) && !contains(scope.DeptIDs, payload.ParentID) {
		return nil, problem("forbidden")
	}
	if err = s.validateDepartment(ctx, p, payload.ID, payload.OrganizationID, payload.ParentID, payload.Status); err != nil {
		return nil, err
	}
	record, err := s.repo.updateDepartment(ctx, scope, payload.ID, payload)
	return record, safe(err)
}
func (s *Service) DeleteDepartment(ctx context.Context, payload *genidentity.DeleteDepartmentPayload) error {
	_, scope, err := s.authorized(ctx, "departments:delete")
	if err != nil {
		return err
	}
	return safe(s.repo.deleteDepartment(ctx, scope, payload.ID))
}
func (s *Service) roleAuthority(ctx context.Context, permission string) (*Principal, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	if !p.Can(permission) || !p.Can("access:manage") {
		return nil, problem("forbidden")
	}
	return p, nil
}
func (s *Service) CreateRole(ctx context.Context, payload *genidentity.CreateRolePayload) (*genidentity.RoleRecord, error) {
	p, err := s.roleAuthority(ctx, "roles:create")
	if err != nil {
		return nil, err
	}
	if err = s.validateGrants(ctx, p, payload.Grants, true); err != nil {
		return nil, err
	}
	record, err := s.repo.createRole(ctx, tenantScope(p), payload)
	return record, safe(err)
}
func (s *Service) UpdateRole(ctx context.Context, payload *genidentity.UpdateRolePayload) (*genidentity.RoleRecord, error) {
	p, err := s.roleAuthority(ctx, "roles:update")
	if err != nil {
		return nil, err
	}
	if err = s.validateGrants(ctx, p, payload.Grants, true); err != nil {
		return nil, err
	}
	record, err := s.repo.updateRole(ctx, tenantScope(p), payload.ID, payload)
	return record, safe(err)
}
func (s *Service) DeleteRole(ctx context.Context, payload *genidentity.DeleteRolePayload) error {
	p, err := s.roleAuthority(ctx, "roles:delete")
	if err != nil {
		return err
	}
	return safe(s.repo.deleteRole(ctx, tenantScope(p), payload.ID))
}

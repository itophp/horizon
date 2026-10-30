//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	"context"
	"fmt"
	"testing"
)

func TestDepartmentMembershipIsolation(t *testing.T) {
	handler, pool, admin := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	response := root.request(t, "GET", "/api/admin/snapshot", nil, false)
	checkStatus(t, response, 200)
	snapshot := decodeBody[genidentity.AdminSnapshot](t, response)
	engineering := snapshot.Departments[1].ID
	grant := func(code, effect, scope string) *genidentity.PermissionGrant {
		return &genidentity.PermissionGrant{Permission: code, Effect: effect, Scope: scope, DepartmentIds: []int64{}}
	}
	roleResponse := root.request(t, "POST", "/api/admin/roles", &genidentity.CreateRolePayload{Name: "Engineering reader", Code: "engineering-reader", Status: "active", Grants: []*genidentity.PermissionGrant{grant("accounts:read", "allow", "DEPARTMENT")}}, true)
	checkStatus(t, roleResponse, 201)
	role := decodeBody[genidentity.RoleRecord](t, roleResponse)
	extra := &genidentity.DepartmentMembership{OrganizationID: admin.OrganizationID, DepartmentID: engineering, RoleIds: []int64{role.ID}, Overrides: []*genidentity.PermissionGrant{grant("roles:read", "deny", "ALL")}}
	create := &genidentity.CreateAccountPayload{Username: "multi-department", FullName: "Multi Department", Status: "active", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, RoleIds: []int64{}, Overrides: []*genidentity.PermissionGrant{grant("departments:read", "allow", "DEPARTMENT")}, Password: "TestOnly!2026Secure", DepartmentAccess: &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{extra}}}
	response = root.request(t, "POST", "/api/admin/accounts", create, true)
	checkStatus(t, response, 201)
	member := decodeBody[genidentity.AccountRecord](t, response)
	if len(member.Memberships) != 2 {
		t.Fatal("membership persistence missing")
	}
	client := loginIAM(t, handler, member.Username)
	me := func(c iamClient) *genidentity.SessionProfile {
		r := c.request(t, "GET", "/api/auth/me", nil, false)
		checkStatus(t, r, 200)
		return decodeBody[*genidentity.SessionProfile](t, r)
	}
	has := func(p *genidentity.SessionProfile, code string) bool {
		for _, v := range p.Permissions {
			if v == code {
				return true
			}
		}
		return false
	}
	switchTo := func(c *iamClient, id int64) *genidentity.SessionProfile {
		r := c.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: id}, true)
		checkStatus(t, r, 200)
		p := decodeBody[*genidentity.SessionProfile](t, r)
		c.csrf = p.CsrfToken
		return p
	}
	t.Run("switch requires authentication", func(t *testing.T) {
		checkStatus(t, (iamClient{handler: handler}).request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: engineering}, true), 401)
	})
	t.Run("same tenant cross company membership uses selected company and rejects mismatched placement", func(t *testing.T) {
		var enterprise int64
		for _, org := range snapshot.Organizations {
			if org.Kind == "enterprise" {
				enterprise = org.ID
				break
			}
		}
		r := root.request(t, "POST", "/api/admin/organizations", &genidentity.CreateOrganizationPayload{Name: "Second company", Code: "second-company", Kind: "company", ParentID: enterprise, Status: "active"}, true)
		checkStatus(t, r, 201)
		company := decodeBody[genidentity.OrganizationRecord](t, r)
		r = root.request(t, "POST", "/api/admin/departments", &genidentity.CreateDepartmentPayload{Name: "Second team", Code: "second-team", OrganizationID: company.ID, Status: "active"}, true)
		checkStatus(t, r, 201)
		dept := decodeBody[genidentity.DepartmentRecord](t, r)
		copy := *create
		copy.Username = "cross-company-member"
		membership := *extra
		membership.DepartmentID = dept.ID
		membership.OrganizationID = company.ID
		copy.DepartmentAccess = &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{&membership}}
		r = root.request(t, "POST", "/api/admin/accounts", &copy, true)
		checkStatus(t, r, 201)
		crossClient := loginIAM(t, handler, copy.Username)
		current := switchTo(&crossClient, dept.ID)
		if current.Account.OrganizationID != company.ID || current.ActiveDepartmentID != dept.ID || !has(current, "accounts:read") {
			t.Fatal("selected company not used for identity")
		}
		membership.OrganizationID = admin.OrganizationID
		copy.Username = "mismatched-company-member"
		checkStatus(t, root.request(t, "POST", "/api/admin/accounts", &copy, true), 400)
	})
	t.Run("independent role and override context with CSRF rotation", func(t *testing.T) {
		initial := me(client)
		if has(initial, "accounts:read") || !has(initial, "departments:read") {
			t.Fatal("primary merged additional authority")
		}
		stale := client
		next := switchTo(&client, engineering)
		if next.ActiveDepartmentID != engineering || !has(next, "accounts:read") || has(next, "departments:read") || has(next, "roles:read") || next.CsrfToken == stale.csrf {
			t.Fatal("department identity or policy isolation failed")
		}
		checkStatus(t, stale.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: admin.DepartmentID}, true), 403)
		checkStatus(t, client.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: admin.DepartmentID}, false), 403)
		checkStatus(t, client.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: 987654321}, true), 403)
		s := client.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, s, 200)
		accounts := decodeBody[genidentity.AdminSnapshot](t, s).Accounts
		found := false
		for _, a := range accounts {
			if a.ID == admin.ID {
				t.Fatal("engineering scope read primary admin")
			}
			found = found || a.ID == member.ID
		}
		if !found {
			t.Fatal("additional department membership not matched by SQL")
		}
	})
	t.Run("override deny only affects its department", func(t *testing.T) {
		replacement := *extra
		replacement.Overrides = []*genidentity.PermissionGrant{grant("accounts:read", "deny", "ALL")}
		write := &genidentity.UpdateAccountPayload{Username: member.Username, FullName: member.FullName, Status: "active", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, RoleIds: create.RoleIds, Overrides: create.Overrides, DepartmentAccess: &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{&replacement}}}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", member.ID), write, true), 200)
		if has(me(client), "accounts:read") {
			t.Fatal("deny did not immediately override department role")
		}
		if !has(switchTo(&client, admin.DepartmentID), "departments:read") {
			t.Fatal("secondary deny polluted primary")
		}
		write.DepartmentAccess.Memberships = []*genidentity.DepartmentMembership{extra}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", member.ID), write, true), 200)
	})
	t.Run("temporary rights bind to application department", func(t *testing.T) {
		r := client.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Grants: []*genidentity.PermissionGrant{grant("accounts:read", "allow", "ALL")}, Minutes: 60, Reason: "Department scoped support incident"}, true)
		checkStatus(t, r, 200)
		request := decodeBody[genidentity.TemporaryAccessRecord](t, r)
		if request.DepartmentID != admin.DepartmentID {
			t.Fatal("untrusted application context")
		}
		r = root.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/review", request.ID), &genidentity.ReviewTemporaryAccessPayload{Decision: "approve", Grants: request.RequestedGrants, Minutes: 30}, true)
		checkStatus(t, r, 200)
		s := client.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, s, 200)
		if len(decodeBody[genidentity.AdminSnapshot](t, s).Accounts) < 3 {
			t.Fatal("primary temporary lease missing")
		}
		switchTo(&client, engineering)
		s = client.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, s, 200)
		for _, a := range decodeBody[genidentity.AdminSnapshot](t, s).Accounts {
			if a.ID == admin.ID {
				t.Fatal("temporary ALL crossed context")
			}
		}
	})
	t.Run("inactive primary falls back at login and inactive additional membership can be removed", func(t *testing.T) {
		r := root.request(t, "POST", "/api/admin/departments", &genidentity.CreateDepartmentPayload{Name: "Retired team", Code: "retired-team", OrganizationID: admin.OrganizationID, Status: "active"}, true)
		checkStatus(t, r, 201)
		retired := decodeBody[genidentity.DepartmentRecord](t, r)
		fallback := *create
		fallback.Username = "fallback-member"
		fallback.DepartmentID = retired.ID
		checkStatus(t, root.request(t, "POST", "/api/admin/accounts", &fallback, true), 201)
		retiring := *create
		retiring.Username = "retiring-extra-member"
		retiringExtra := *extra
		retiringExtra.DepartmentID = retired.ID
		retiring.DepartmentAccess = &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{&retiringExtra}}
		r = root.request(t, "POST", "/api/admin/accounts", &retiring, true)
		checkStatus(t, r, 201)
		record := decodeBody[genidentity.AccountRecord](t, r)
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/departments/%d", retired.ID), &genidentity.UpdateDepartmentPayload{Name: retired.Name, Code: retired.Code, OrganizationID: retired.OrganizationID, Status: "inactive"}, true), 200)
		fallbackClient := loginIAM(t, handler, fallback.Username)
		if me(fallbackClient).ActiveDepartmentID != engineering {
			t.Fatal("inactive primary prevented valid alternate login")
		}
		checkStatus(t, fallbackClient.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: retired.ID}, true), 403)
		write := &genidentity.UpdateAccountPayload{Username: record.Username, FullName: record.FullName, Status: record.Status, OrganizationID: record.OrganizationID, DepartmentID: record.DepartmentID, RoleIds: record.RoleIds, Overrides: record.Overrides, DepartmentAccess: &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{}}}
		r = root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", record.ID), write, true)
		checkStatus(t, r, 200)
		if len(decodeBody[genidentity.AccountRecord](t, r).Memberships) != 1 {
			t.Fatal("inactive membership was retained")
		}
	})
	t.Run("duplicate or foreign tenant membership rejected atomically", func(t *testing.T) {
		copy := *create
		copy.Username = "duplicate-membership"
		copy.DepartmentAccess = &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{extra, extra}}
		checkStatus(t, root.request(t, "POST", "/api/admin/accounts", &copy, true), 400)
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM iam_accounts WHERE username=$1", copy.Username).Scan(&count); err != nil || count != 0 {
			t.Fatal("invalid create partially persisted", err)
		}
		// A department in another tenant is never an affiliation option or switch identity.
		var enterprise, company, dept int64
		if err := pool.QueryRow(context.Background(), "INSERT INTO iam_organizations(tenant_id,name,code,kind,status) VALUES('tenant-other','Other','other-enterprise','enterprise','active') RETURNING id").Scan(&enterprise); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(), "INSERT INTO iam_organizations(tenant_id,name,code,kind,status,parent_id) VALUES('tenant-other','Other company','other-company','company','active',$1) RETURNING id", enterprise).Scan(&company); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(), "INSERT INTO iam_departments(tenant_id,organization_id,name,code,status) VALUES('tenant-other',$1,'Other department','other-dept','active') RETURNING id", company).Scan(&dept); err != nil {
			t.Fatal(err)
		}
		foreign := *extra
		foreign.DepartmentID = dept
		foreign.OrganizationID = company
		copy.Username = "foreign-membership"
		copy.DepartmentAccess.Memberships = []*genidentity.DepartmentMembership{&foreign}
		checkStatus(t, root.request(t, "POST", "/api/admin/accounts", &copy, true), 400)
		checkStatus(t, client.request(t, "POST", "/api/auth/department", &genidentity.SwitchDepartmentPayload{DepartmentID: dept}, true), 403)
		if _, err := pool.Exec(context.Background(), "INSERT INTO iam_department_memberships(tenant_id,account_id,organization_id,department_id) VALUES($1,$2,$3,$4)", "tenant-a", member.ID, company, dept); err == nil {
			t.Fatal("cross tenant membership FK absent")
		}
	})
	t.Run("department manager cannot rewrite other department policies or delete multi department account", func(t *testing.T) {
		m := *create
		m.Username = "limited-membership-manager"
		m.DepartmentAccess = nil
		m.Overrides = []*genidentity.PermissionGrant{grant("accounts:read", "allow", "DEPARTMENT"), grant("accounts:update", "allow", "DEPARTMENT"), grant("accounts:delete", "allow", "DEPARTMENT"), grant("access:manage", "allow", "ALL")}
		r := root.request(t, "POST", "/api/admin/accounts", &m, true)
		checkStatus(t, r, 201)
		manager := loginIAM(t, handler, m.Username)
		replacement := *extra
		replacement.Overrides = []*genidentity.PermissionGrant{grant("accounts:delete", "allow", "ALL")}
		payload := &genidentity.UpdateAccountPayload{Username: member.Username, FullName: member.FullName, Status: "active", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, RoleIds: create.RoleIds, Overrides: create.Overrides, DepartmentAccess: &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{&replacement}}}
		checkStatus(t, manager.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", member.ID), payload, true), 403)
		checkStatus(t, manager.request(t, "DELETE", fmt.Sprintf("/api/admin/accounts/%d", member.ID), nil, true), 403)
	})
	t.Run("removed membership fails closed and old lease cannot resurrect", func(t *testing.T) {
		// Request from the extra context, approve, then detach it.
		r := client.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Grants: []*genidentity.PermissionGrant{grant("departments:read", "allow", "ALL")}, Minutes: 60, Reason: "Temporary department support task"}, true)
		checkStatus(t, r, 200)
		request := decodeBody[genidentity.TemporaryAccessRecord](t, r)
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/review", request.ID), &genidentity.ReviewTemporaryAccessPayload{Decision: "approve", Grants: request.RequestedGrants, Minutes: 30}, true), 200)
		write := &genidentity.UpdateAccountPayload{Username: member.Username, FullName: member.FullName, Status: "active", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, RoleIds: create.RoleIds, Overrides: create.Overrides, DepartmentAccess: &genidentity.DepartmentAccess{Memberships: []*genidentity.DepartmentMembership{}}}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", member.ID), write, true), 200)
		checkStatus(t, client.request(t, "GET", "/api/auth/me", nil, false), 401)
		write.DepartmentAccess.Memberships = []*genidentity.DepartmentMembership{extra}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", member.ID), write, true), 200)
		if has(me(client), "departments:read") {
			t.Fatal("detached temporary grant resurrected")
		}
		var status string
		if err := pool.QueryRow(context.Background(), "SELECT status FROM iam_temporary_requests WHERE id=$1", request.ID).Scan(&status); err != nil || status != "revoked" {
			t.Fatal("detachment not persisted", err, status)
		}
	})
}

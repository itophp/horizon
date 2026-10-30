//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/identity"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTemporaryAccessLifecycleAndBoundaries(t *testing.T) {
	handler, pool, adminAccount := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	initial := decodeBody[genidentity.AdminSnapshot](t, root.request(t, "GET", "/api/admin/snapshot", nil, false))
	company := adminAccount.OrganizationID
	engineering := initial.Departments[1].ID
	grant := func(permission, scope string, ids ...int64) *genidentity.PermissionGrant {
		return &genidentity.PermissionGrant{Permission: permission, Effect: "allow", Scope: scope, DepartmentIds: append([]int64{}, ids...)}
	}
	account := func(username string, dept int64, overrides ...*genidentity.PermissionGrant) *genidentity.AccountRecord {
		response := root.request(t, "POST", "/api/admin/accounts", &genidentity.CreateAccountPayload{Username: username, FullName: username, OrganizationID: company, DepartmentID: dept, Status: "active", RoleIds: []int64{}, Overrides: append([]*genidentity.PermissionGrant{}, overrides...), Password: "TestOnly!2026Secure"}, true)
		checkStatus(t, response, 201)
		return decodeBody[*genidentity.AccountRecord](t, response)
	}
	applicant := account("temporary-applicant", engineering)
	peer := account("temporary-peer", engineering)
	reviewerAccount := account("limited-reviewer", engineering, grant("temporary:approve", "ALL"), grant("accounts:read", "DEPARTMENT"))
	managerAccount := account("policy-manager", engineering, grant("access:manage", "ALL"))
	applicantClient := loginIAM(t, handler, applicant.Username)
	reviewer := loginIAM(t, handler, reviewerAccount.Username)
	manager := loginIAM(t, handler, "policy-manager")
	create := func(client iamClient, grants ...*genidentity.PermissionGrant) *genidentity.TemporaryAccessRecord {
		response := client.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Grants: grants, Minutes: 60, Reason: "Support incident requiring scoped access"}, true)
		checkStatus(t, response, 200)
		return decodeBody[*genidentity.TemporaryAccessRecord](t, response)
	}
	reviewPayload := func(minutes int, grants ...*genidentity.PermissionGrant) *genidentity.ReviewTemporaryAccessPayload {
		return &genidentity.ReviewTemporaryAccessPayload{Decision: "approve", Minutes: minutes, Grants: grants, Note: "Adjusted by approver"}
	}
	reviewPath := func(id int64) string { return fmt.Sprintf("/api/admin/temporary-access/%d/review", id) }
	snapshot := func(client iamClient) *genidentity.AdminSnapshot {
		response := client.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, response, 200)
		return decodeBody[*genidentity.AdminSnapshot](t, response)
	}
	updateApplicant := func(dept int64, overrides []*genidentity.PermissionGrant) {
		response := root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", applicant.ID), &genidentity.UpdateAccountPayload{ID: applicant.ID, Username: applicant.Username, FullName: applicant.FullName, Status: "active", OrganizationID: company, DepartmentID: dept, RoleIds: []int64{}, Overrides: overrides}, true)
		checkStatus(t, response, 200)
	}
	if len(snapshot(applicantClient).Accounts) != 0 {
		t.Fatal("unprivileged applicant can already read accounts")
	}
	t.Run("directory and catalog available without protected resource permissions", func(t *testing.T) {
		response := applicantClient.request(t, "GET", "/api/admin/temporary-access", nil, false)
		checkStatus(t, response, 200)
		workspace := decodeBody[genidentity.TemporaryAccessWorkspace](t, response)
		if workspace.CanReview || len(workspace.Catalog) != 41 || len(workspace.Departments) != 2 {
			t.Fatal("request directory or independent approval permission incorrect")
		}
		response = manager.request(t, "GET", "/api/admin/temporary-access", nil, false)
		checkStatus(t, response, 200)
		if decodeBody[genidentity.TemporaryAccessWorkspace](t, response).CanReview {
			t.Fatal("access manager implicitly received approval")
		}
	})
	request := create(applicantClient, grant("accounts:read", "ALL"))
	t.Run("reject missing approval permission, self approval, administrative leases and invalid input", func(t *testing.T) {
		checkStatus(t, manager.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 403)
		self := create(reviewer, grant("accounts:read", "ALL"))
		checkStatus(t, reviewer.request(t, "POST", reviewPath(self.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 403)
		for _, permission := range []string{"access:manage", "temporary:approve", "roles:update"} {
			checkStatus(t, applicantClient.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Minutes: 60, Reason: "Valid request reason", Grants: []*genidentity.PermissionGrant{grant(permission, "ALL")}}, true), 400)
		}
		checkStatus(t, applicantClient.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Minutes: 61, Reason: "Valid request reason", Grants: []*genidentity.PermissionGrant{grant("accounts:read", "ALL")}}, true), 400)
	})
	t.Run("approval cannot exceed permanent authority", func(t *testing.T) {
		checkStatus(t, reviewer.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 403)
		// Even an approved broader temporary range cannot be re-delegated.
		broader := create(reviewer, grant("accounts:read", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(broader.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 200)
		checkStatus(t, reviewer.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 403)
	})
	t.Run("approver edits duration and scope without altering original request", func(t *testing.T) {
		response := root.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "DEPARTMENT")), true)
		checkStatus(t, response, 200)
		approved := decodeBody[genidentity.TemporaryAccessRecord](t, response)
		if approved.RequestedMinutes != 60 || approved.RequestedGrants[0].Scope != "ALL" || approved.ApprovedMinutes != 30 || approved.ApprovedGrants[0].Scope != "DEPARTMENT" || approved.ReviewerID != adminAccount.ID {
			t.Fatal("original request or approved amendment lost")
		}
		start, err := time.Parse(time.RFC3339, approved.ReviewedAt)
		if err != nil {
			t.Fatal(err)
		}
		end, err := time.Parse(time.RFC3339, approved.ExpiresAt)
		if err != nil || end.Sub(start) != 30*time.Minute {
			t.Fatal("validity did not start at approval", err)
		}
		visible := snapshot(applicantClient).Accounts
		foundPeer := false
		for _, row := range visible {
			if row.ID == adminAccount.ID {
				t.Fatal("scope expanded to admin department")
			}
			foundPeer = foundPeer || row.ID == peer.ID
		}
		if !foundPeer {
			t.Fatal("approved temporary permission did not activate")
		}
		checkStatus(t, root.request(t, "POST", reviewPath(request.ID), reviewPayload(60, grant("accounts:read", "ALL")), true), 409)
	})
	t.Run("leaving a department revokes its lease and returning does not resurrect it", func(t *testing.T) {
		updateApplicant(adminAccount.DepartmentID, []*genidentity.PermissionGrant{})
		if len(snapshot(applicantClient).Accounts) != 0 {
			t.Fatal("lease crossed department contexts")
		}
		updateApplicant(engineering, []*genidentity.PermissionGrant{})
		if len(snapshot(applicantClient).Accounts) != 0 {
			t.Fatal("detached lease resurrected")
		}
		request = create(applicantClient, grant("accounts:read", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "DEPARTMENT")), true), 200)
	})
	t.Run("database clock expiration immediately removes operation and data access", func(t *testing.T) {
		if _, err := pool.Exec(context.Background(), "UPDATE iam_temporary_requests SET reviewed_at=now()-interval '31 minutes',expires_at=now()-interval '1 second' WHERE id=$1", request.ID); err != nil {
			t.Fatal(err)
		}
		if len(snapshot(applicantClient).Accounts) != 0 {
			t.Fatal("expired grant still authorizes")
		}
		response := applicantClient.request(t, "GET", "/api/admin/temporary-access", nil, false)
		checkStatus(t, response, 200)
		records := decodeBody[genidentity.TemporaryAccessWorkspace](t, response).Requests
		if len(records) != 2 || records[0].Status != "expired" {
			t.Fatal("expiration not visible in history")
		}
	})
	t.Run("existing limited data scope may temporarily expand and then return", func(t *testing.T) {
		updateApplicant(adminAccount.DepartmentID, []*genidentity.PermissionGrant{grant("accounts:read", "OWN")})
		if len(snapshot(applicantClient).Accounts) != 1 {
			t.Fatal("permanent OWN missing")
		}
		lease := create(applicantClient, grant("accounts:read", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(lease.ID), reviewPayload(60, grant("accounts:read", "ALL")), true), 200)
		if len(snapshot(applicantClient).Accounts) < 4 {
			t.Fatal("temporary scope did not union with permanent own range")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/revoke", lease.ID), nil, true), 200)
		if len(snapshot(applicantClient).Accounts) != 1 {
			t.Fatal("revocation did not restore permanent scope")
		}
		var actor int64
		if err := pool.QueryRow(context.Background(), "SELECT revoked_by FROM iam_temporary_requests WHERE id=$1 AND revoked_at IS NOT NULL", lease.ID).Scan(&actor); err != nil || actor != adminAccount.ID {
			t.Fatal("revocation audit missing", err)
		}
	})
	t.Run("explicit account deny remains stronger than active temporary allow", func(t *testing.T) {
		lease := create(applicantClient, grant("accounts:read", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(lease.ID), reviewPayload(60, grant("accounts:read", "ALL")), true), 200)
		denied := grant("accounts:read", "ALL")
		denied.Effect = "deny"
		updateApplicant(adminAccount.DepartmentID, []*genidentity.PermissionGrant{denied})
		if len(snapshot(applicantClient).Accounts) != 0 {
			t.Fatal("temporary allow bypassed explicit deny")
		}
		second := create(applicantClient, grant("accounts:read", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(second.ID), reviewPayload(60, grant("accounts:read", "ALL")), true), 403)
		updateApplicant(adminAccount.DepartmentID, []*genidentity.PermissionGrant{})
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/revoke", lease.ID), nil, true), 200)
	})
	t.Run("withdrawal, rejection and owner boundaries", func(t *testing.T) {
		pending := create(applicantClient, grant("departments:read", "CUSTOM", engineering))
		checkStatus(t, reviewer.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/cancel", pending.ID), nil, true), 404)
		checkStatus(t, applicantClient.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/cancel", pending.ID), nil, true), 200)
		checkStatus(t, root.request(t, "POST", reviewPath(pending.ID), reviewPayload(30, grant("departments:read", "ALL")), true), 409)
		reject := create(applicantClient, grant("departments:read", "ALL"))
		response := root.request(t, "POST", reviewPath(reject.ID), &genidentity.ReviewTemporaryAccessPayload{Decision: "reject", Minutes: 30, Grants: []*genidentity.PermissionGrant{}, Note: "Not required"}, true)
		checkStatus(t, response, 200)
		if decodeBody[genidentity.TemporaryAccessRecord](t, response).Status != "rejected" {
			t.Fatal("rejection missing")
		}
	})
	t.Run("parallel approval has one winner and cannot double activate", func(t *testing.T) {
		pending := create(applicantClient, grant("departments:read", "ALL"))
		codes := make(chan int, 2)
		for range 2 {
			go func() {
				codes <- root.request(t, "POST", reviewPath(pending.ID), reviewPayload(30, grant("departments:read", "ALL")), true).Code
			}()
		}
		first, second := <-codes, <-codes
		if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
			t.Fatal("approval race was not atomic", first, second)
		}
	})
	t.Run("temporary writes cannot leave permanent identity or role escalation", func(t *testing.T) {
		lease := create(manager, grant("accounts:update", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(lease.ID), reviewPayload(30, grant("accounts:update", "ALL")), true), 200)
		payload := &genidentity.UpdateAccountPayload{ID: managerAccount.ID, Username: managerAccount.Username, FullName: "Updated display name", Email: managerAccount.Email, Status: "active", OrganizationID: company, DepartmentID: engineering, RoleIds: []int64{}, Overrides: managerAccount.Overrides}
		checkStatus(t, manager.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", managerAccount.ID), payload, true), 200)
		payload.RoleIds = []int64{initial.Roles[0].ID}
		checkStatus(t, manager.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", managerAccount.ID), payload, true), 403)
		payload.RoleIds = []int64{}
		payload.DepartmentID = adminAccount.DepartmentID
		checkStatus(t, manager.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", managerAccount.ID), payload, true), 403)
		payload.ID = reviewerAccount.ID
		payload.Username = reviewerAccount.Username
		payload.FullName = reviewerAccount.FullName
		payload.DepartmentID = engineering
		payload.Overrides = reviewerAccount.Overrides
		payload.Password = "A-ResetPassword!2026"
		checkStatus(t, manager.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", reviewerAccount.ID), payload, true), 403)
	})
	t.Run("temporary creation cannot issue permanently privileged identities", func(t *testing.T) {
		lease := create(manager, grant("accounts:create", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(lease.ID), reviewPayload(30, grant("accounts:create", "ALL")), true), 200)
		payload := &genidentity.CreateAccountPayload{Username: "lease-created-account", FullName: "Lease created account", Status: "active", OrganizationID: company, DepartmentID: engineering, RoleIds: []int64{initial.Roles[0].ID}, Overrides: []*genidentity.PermissionGrant{}, Password: "TestOnly!2026Secure"}
		checkStatus(t, manager.request(t, "POST", "/api/admin/accounts", payload, true), 403)
		payload.RoleIds = []int64{}
		checkStatus(t, manager.request(t, "POST", "/api/admin/accounts", payload, true), 201)
	})
	t.Run("temporary hierarchy changes cannot expand permanent department trees", func(t *testing.T) {
		actor := account("tree-actor", engineering, grant("accounts:read", "DEPARTMENT_TREE"))
		actorClient := loginIAM(t, handler, actor.Username)
		lease := create(actorClient, grant("departments:update", "ALL"))
		checkStatus(t, root.request(t, "POST", reviewPath(lease.ID), reviewPayload(30, grant("departments:update", "ALL")), true), 200)
		department := initial.Departments[0]
		payload := &genidentity.UpdateDepartmentPayload{ID: department.ID, Name: department.Name, Code: department.Code, OrganizationID: company, ParentID: engineering, Manager: department.Manager, Status: "active", Description: department.Description}
		checkStatus(t, actorClient.request(t, "PUT", fmt.Sprintf("/api/admin/departments/%d", department.ID), payload, true), 403)
		for _, row := range snapshot(actorClient).Accounts {
			if row.ID == adminAccount.ID {
				t.Fatal("hierarchy mutation created lasting scope escalation")
			}
		}
	})
	t.Run("tenant boundary applies to inbox, approval, cancellation and scope IDs", func(t *testing.T) {
		if _, err := identity.Bootstrap(context.Background(), pool, "tenant-b", "foreign-reviewer", "TestOnly!2026Secure"); err != nil {
			t.Fatal(err)
		}
		foreign := loginIAM(t, handler, "foreign-reviewer")
		response := foreign.request(t, "GET", "/api/admin/temporary-access", nil, false)
		checkStatus(t, response, 200)
		if len(decodeBody[genidentity.TemporaryAccessWorkspace](t, response).Requests) != 0 {
			t.Fatal("cross tenant inbox leak")
		}
		checkStatus(t, foreign.request(t, "POST", reviewPath(request.ID), reviewPayload(30, grant("accounts:read", "ALL")), true), 404)
		checkStatus(t, foreign.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/revoke", request.ID), nil, true), 404)
		checkStatus(t, foreign.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Minutes: 60, Reason: "Valid request reason", Grants: []*genidentity.PermissionGrant{grant("accounts:read", "CUSTOM", engineering)}}, true), 400)
	})
}

package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"context"
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "Correct-Horse-2026!"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("salt was reused")
	}
	if !VerifyPassword(password, first) || VerifyPassword("wrong", first) || VerifyPassword(password, "not-an-argon-hash") {
		t.Fatal("password verification mismatch")
	}
	if _, err = HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if _, err = HashPassword("四个汉字"); err == nil {
		t.Fatal("password policy counted bytes instead of characters")
	}
}
func TestAccountOverridesReplaceRoleScopeAndDenyWins(t *testing.T) {
	account := &genidentity.AccountRecord{ID: 10, OrganizationID: 7, DepartmentID: 8}
	roles := []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "ALL"}, {Permission: "accounts:delete", Effect: "allow", Scope: "ALL"}}
	overrides := []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "DEPARTMENT"}, {Permission: "accounts:delete", Effect: "deny", Scope: "ALL"}, {Permission: "accounts:update", Effect: "allow", Scope: "CUSTOM", DepartmentIds: []int64{12}}}
	p, err := NewPrincipal(account, "tenant-a", MergeGrants(roles, overrides))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Can("accounts:read") || p.Can("accounts:delete") || p.Can("roles:update") {
		t.Fatal("incorrect operation decision")
	}
	scope, err := (&Service{}).scope(context.Background(), p, "accounts:read")
	if err != nil || scope.All || !contains(scope.DeptIDs, 8) || scope.TenantID != "tenant-a" {
		t.Fatal("account override did not narrow inherited scope", scope, err)
	}
	custom, err := (&Service{}).scope(context.Background(), p, "accounts:update")
	if err != nil || custom.All || !contains(custom.DeptIDs, 12) {
		t.Fatal("custom scope missing", custom, err)
	}
	if _, err = (&Service{}).scope(context.Background(), p, "accounts:delete"); err == nil {
		t.Fatal("explicit denial bypassed")
	}
}
func TestPolicyUnionsRolesAndRejectsMissingContext(t *testing.T) {
	p, err := NewPrincipal(&genidentity.AccountRecord{ID: 1}, "t", []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "OWN"}, {Permission: "roles:read", Effect: "allow", Scope: "ALL"}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Can("accounts:read") || !p.Can("roles:read") || p.Can("accounts:update") {
		t.Fatal("role union is incorrect")
	}
	if _, err = Current(context.Background()); err == nil {
		t.Fatal("untrusted identity accepted")
	}
	if CSRFMatches(p, "") {
		t.Fatal("empty CSRF accepted")
	}
	p.CSRF = "server-nonce"
	if !CSRFMatches(p, "server-nonce") || CSRFMatches(p, "attacker") {
		t.Fatal("CSRF mismatch")
	}
}

func TestGovernanceCatalogDoesNotMakeManagementTemporary(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range PermissionCatalog() {
		if seen[d.Code] {
			t.Fatal("duplicate permission", d.Code)
		}
		seen[d.Code] = true
		if d.Module == "system" {
			expected := d.Code == "files:read" || d.Code == "dictionaries:use"
			if d.TemporaryEligible != expected {
				t.Fatal("management became temporary", d.Code)
			}
			if seedViewerPermission(d.Code) {
				t.Fatal("new privilege entered old viewer seed", d.Code)
			}
		}
	}
	if len(seen) != 41 {
		t.Fatal("unexpected catalog", len(seen))
	}
}

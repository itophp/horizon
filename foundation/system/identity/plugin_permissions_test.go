package identity

import (
	"context"
	"testing"

	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/plugin"
)

func TestPluginPermissionCatalogIsInstanceScopedAndFailClosed(t *testing.T) {
	permission := plugin.Permission{Code: "widget:read", Module: "business", Function: "widget", Action: "read", ScopeConfigurable: true}
	service := New(nil, false)
	if err := service.ConfigurePluginPermissions([]plugin.Permission{permission}); err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigurePluginPermissions([]plugin.Permission{permission}); err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigurePluginPermissions([]plugin.Permission{{Code: "widget:read", Module: "business", Function: "widget", Action: "read", TemporaryEligible: true}}); err == nil {
		t.Fatal("accepted catalog replacement")
	}
	grant := &genidentity.PermissionGrant{Permission: "widget:read", Effect: "allow", Scope: "ALL"}
	account := &genidentity.AccountRecord{ID: 42}
	p, err := service.NewPrincipal(account, "tenant-a", []*genidentity.PermissionGrant{grant})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Can("widget:read") || !containsString(p.Effective(), "widget:read") {
		t.Fatal("configured permission not effective")
	}
	if err := service.validateGrants(context.Background(), p, []*genidentity.PermissionGrant{grant}, true); err != nil {
		t.Fatalf("configured permission rejected: %v", err)
	}
	permanent, err := p.permanent()
	if err != nil || !permanent.Can("widget:read") {
		t.Fatal("principal rebuild lost plugin catalog")
	}
	other, err := New(nil, false).NewPrincipal(account, "tenant-a", []*genidentity.PermissionGrant{grant})
	if err != nil {
		t.Fatal(err)
	}
	if other.Can("widget:read") || containsString(other.Effective(), "widget:read") {
		t.Fatal("plugin permission leaked to another identity service")
	}
	if err := New(nil, false).validateGrants(context.Background(), other, []*genidentity.PermissionGrant{grant}, true); err == nil {
		t.Fatal("unknown permission accepted")
	}
	admin, err := New(nil, false).NewPrincipal(&genidentity.AccountRecord{ID: 1, IsAdmin: true}, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Can("widget:read") {
		t.Fatal("admin received unknown permission")
	}
	denied, err := service.NewPrincipal(account, "tenant-a", []*genidentity.PermissionGrant{grant, {Permission: "widget:read", Effect: "deny", Scope: "ALL"}})
	if err != nil {
		t.Fatal(err)
	}
	if denied.Can("widget:read") {
		t.Fatal("deny did not override plugin grant")
	}
}
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
func TestPluginPermissionCatalogRejectsConflicts(t *testing.T) {
	s := New(nil, false)
	for _, definitions := range [][]plugin.Permission{
		{{Code: "accounts:read", Module: "business", Function: "accounts", Action: "read"}},
		{{Code: "x:read", Module: "business", Function: "y", Action: "read"}},
		{{Code: "x:read", Module: "business", Function: "x", Action: "read"}, {Code: "x:read", Module: "business", Function: "x", Action: "read"}},
	} {
		if err := s.ConfigurePluginPermissions(definitions); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
	if s.permissionDefinition("x:read") != nil {
		t.Fatal("failed configuration published partial catalog")
	}
}

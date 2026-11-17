package applicationruntime

import (
	"testing"

	gen "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/identity"
)

func TestSettingsDefinitionRequiresBothPermanentGrants(t *testing.T) {
	app := ar.ApplicationV2{ID: "system-settings", Source: "builtin:system-settings", Permission: ar.AuthenticatedPermission}
	module := &Module{Applications: []ar.ApplicationV2{app}}
	for _, tc := range []struct {
		permissions []string
		want        bool
	}{
		{nil, false},
		{[]string{"applications:design"}, false},
		{[]string{"settings:read"}, false},
		{[]string{"applications:design", "settings:read"}, true},
	} {
		var grants []*gen.PermissionGrant
		for _, permission := range tc.permissions {
			grants = append(grants, &gen.PermissionGrant{Permission: permission, Effect: "allow", Scope: "ALL"})
		}
		p, err := identity.NewPrincipal(&gen.AccountRecord{ID: 1}, "tenant", grants)
		if err != nil {
			t.Fatal(err)
		}
		if got := module.canManageSettingsDefinition(p, app, "applications:design"); got != tc.want {
			t.Errorf("grants %v: want authorization %t, got %t", tc.permissions, tc.want, got)
		}
		if module.canManageSettingsDefinition(p, app, "applications:manage") {
			t.Errorf("grants %v must not authorize publishing", tc.permissions)
		}
	}
}

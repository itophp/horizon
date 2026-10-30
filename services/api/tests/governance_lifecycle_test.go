//go:build integration

package tests

import (
	filegen "baseline/services/api/gen/files"
	iamgen "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/settings"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/transport"
	"context"
	"fmt"
	"testing"
)

func TestDeletingAccountClearsPreferencesAndPreservesSharedHistory(t *testing.T) {
	_, pool, account := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	options := governance.Options{StorageRoot: t.TempDir()}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, options, identity.New(pool, false))
	root := loginIAM(t, handler, "root-admin")
	grants := []*iamgen.PermissionGrant{}
	for _, code := range []string{"settings:read", "settings:update", "settings:publish", "files:create", "files:read"} {
		grants = append(grants, &iamgen.PermissionGrant{Permission: code, Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}})
	}
	w := root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: "lifecycle-user", FullName: "Lifecycle user", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, Status: "active", Password: "TestOnly!2026Secure", RoleIds: []int64{}, Overrides: grants}, true)
	checkStatus(t, w, 201)
	created := decodeBody[iamgen.AccountRecord](t, w)
	user := loginIAM(t, handler, "lifecycle-user")
	checkStatus(t, user.request(t, "PUT", "/api/auth/preferences", &gen.PreferencesPayload{Changes: []*gen.ChangeEntry{{Key: "display.theme", Operation: "set", Value: `"light"`}}}, true), 200)
	w = user.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, Reason: "Retain shared publication after account deletion", Changes: []*gen.ChangeEntry{{Key: "display.pageSize", Operation: "set", Value: "5"}}}, true)
	checkStatus(t, w, 201)
	draft := decodeBody[gen.ChangeRecord](t, w)
	checkStatus(t, user.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", draft.ID), &gen.PublishPayload{}, true), 200)
	w = user.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "retained.txt", Purpose: "attachment", Content: []byte("retained after owner deletion")}, true)
	checkStatus(t, w, 201)
	file := decodeBody[filegen.ManagedFile](t, w)
	checkStatus(t, root.request(t, "DELETE", fmt.Sprintf("/api/admin/accounts/%d", created.ID), nil, true), 204)
	checkStatus(t, user.request(t, "GET", "/api/auth/me", nil, false), 401)
	checkStatus(t, root.request(t, "GET", fmt.Sprintf("/api/admin/files/%d/content", file.ID), nil, false), 200)
	var personal, shared int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM gov_scopes WHERE kind='account' AND entity_id=$1", created.ID).Scan(&personal); err != nil || personal != 0 {
		t.Fatal("personal preferences survived deleted account", personal, err)
	}
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM gov_versions WHERE actor_id=$1 AND scope_key=$2", created.ID, "workspace:0").Scan(&shared); err != nil || shared != 1 {
		t.Fatal("shared history lost", shared, err)
	}
	other, err := identity.Bootstrap(context.Background(), pool, "other-tenant", "lifecycle-other-root", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), "INSERT INTO gov_audit(tenant_id,scope_key,actor_id,action,resource_key,revision,detail) VALUES($1,$2,$3,'forged','test',0,'{}')", "tenant-a", "workspace:0", other.ID); err == nil {
		t.Fatal("historical identity foreign key allowed cross-tenant actor")
	}

}

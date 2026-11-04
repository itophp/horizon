//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"

	"baseline/services/api/applications/projects"
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	"baseline/services/api/kernel/plugin"
	"baseline/services/api/runtime/app"
	"baseline/services/api/runtime/transport"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type extraPermissionPlugin struct{ pool *pgxpool.Pool }

func (p extraPermissionPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{ID: "test.extra", Version: "1.0.0", APIVersion: plugin.APIVersion, Mode: "builtin", Extensions: []string{"capability", "permission"}}
}
func (p extraPermissionPlugin) Register(_ context.Context, r *plugin.Registrar) error {
	for _, entry := range projects.New(p.pool).Entries() {
		if entry.Definition.Name == "project.get" {
			entry.Definition.Name = "plugin.test.project.get"
			entry.Definition.Permission = "plugin_test:read"
			r.Capabilities([]registry.Entry{entry})
			r.Permissions(plugin.Permission{Code: "plugin_test:read", Module: "business", Function: "plugin_test", Action: "read"})
			return nil
		}
	}
	return contracts.Problem("not_found")
}
func (extraPermissionPlugin) Start(context.Context) error { return nil }
func (extraPermissionPlugin) Stop(context.Context) error  { return nil }

func TestPluginPermissionGatewayTenantAndDeny(t *testing.T) {
	h := newProjectHarness(t)
	projectID := rowID(t, h.ok(t, h.root, "project.create", map[string]any{"code": "PLUGIN-PERM", "name": "Plugin permission"}))
	actor := h.account(t, "plugin-reader", projects.ReadPermission)
	h.ok(t, h.root, "project.member.add", map[string]any{"projectId": projectID, "accountId": actor.id})
	manager := plugin.New(projects.Plugin{Module: projects.New(h.pool)}, extraPermissionPlugin{pool: h.pool})
	if err := manager.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	iam := identity.New(h.pool, false)
	if err := iam.ConfigurePluginPermissions(manager.Catalog.Permissions); err != nil {
		t.Fatal(err)
	}
	core := governance.New(h.pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir(), PublicWorkspace: "tenant-a"})
	handler, _ := transport.HandlerWithPlugins(app.System{}, core, manager, iam)
	administrator := loginIAM(t, handler, "root-admin")
	snapshotResponse := administrator.request(t, "GET", "/api/admin/snapshot", nil, false)
	checkStatus(t, snapshotResponse, 200)
	snapshot := decodeBody[genidentity.AdminSnapshot](t, snapshotResponse)
	found := false
	for _, definition := range snapshot.Catalog {
		if definition.Code == "plugin_test:read" {
			found = true
		}
	}
	if !found {
		t.Fatal("plugin permission missing from authenticated identity catalog")
	}
	catalog := registry.New(manager.Catalog.Entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	gw := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: h.pool, Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	account := &genidentity.AccountRecord{ID: actor.id, Status: "active", OrganizationID: h.rootAccount.OrganizationID, DepartmentID: h.rootAccount.DepartmentID}
	invoke := func(tenant string, grants ...*genidentity.PermissionGrant) (*contracts.Result, error) {
		principal, err := iam.NewPrincipal(account, tenant, grants)
		if err != nil {
			t.Fatal(err)
		}
		return gw.Invoke(identity.WithPrincipal(context.Background(), principal), gateway.Request{Name: "plugin.test.project.get", Version: "1.0.0", Input: contracts.Input{"id": projectID}, Origin: contracts.OriginHumanUI})
	}
	// A stale or incorrectly assembled principal cannot bypass an unknown catalog
	// permission merely because it is marked as an administrator.
	unknownAdmin, err := identity.New(nil, false).NewPrincipal(&genidentity.AccountRecord{ID: 9001, IsAdmin: true}, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.Invoke(identity.WithPrincipal(context.Background(), unknownAdmin), gateway.Request{Name: "plugin.test.project.get", Version: "1.0.0", Input: contracts.Input{"id": projectID}, Origin: contracts.OriginHumanUI})
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("unknown admin permission: %v", err)
	}
	definitions, err := gw.Definitions(identity.WithPrincipal(context.Background(), unknownAdmin))
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if definition.Name == "plugin.test.project.get" {
			t.Fatal("unknown admin permission was discoverable")
		}
	}

	grant := &genidentity.PermissionGrant{Permission: "plugin_test:read", Effect: "allow", Scope: "ALL"}
	result, err := invoke("tenant-a", grant)
	succeeded(t, result, err)
	// Approval/workflow/trigger execution reconstructs the principal from storage.
	saved, err := json.Marshal([]*genidentity.PermissionGrant{grant})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides=$1 WHERE tenant_id=$2 AND id=$3`, saved, "tenant-a", actor.id); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := iam.PrincipalForTask(context.Background(), "tenant-a", actor.id, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuilt.Can("plugin_test:read") {
		t.Fatal("reconstructed principal lost plugin permission")
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides='[]' WHERE tenant_id=$1 AND id=$2`, "tenant-a", actor.id); err != nil {
		t.Fatal(err)
	}
	rebuilt, err = iam.PrincipalForTask(context.Background(), "tenant-a", actor.id, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Can("plugin_test:read") {
		t.Fatal("revoked plugin permission remained effective")
	}
	_, err = invoke("tenant-a")
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("missing permission: %v", err)
	}
	_, err = invoke("tenant-a", grant, &genidentity.PermissionGrant{Permission: "plugin_test:read", Effect: "deny", Scope: "ALL"})
	if contracts.Code(err) != "forbidden" {
		t.Fatalf("explicit deny: %v", err)
	}
	result, err = invoke("tenant-other", grant)
	rejected(t, result, err, "not_found")
}

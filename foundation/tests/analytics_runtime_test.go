//go:build integration

package tests

import (
	"testing"

	"horizon/foundation/db/migrations"
	gencap "horizon/foundation/gen/capability"
	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/runtime/app"
	"horizon/foundation/runtime/transport"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestAnalyticsRuntimeGatewayPermissions(t *testing.T) {
	handler, pool, rootAccount := iamFixture(t)
	anonymous := iamClient{handler: handler}
	checkStatus(t, anonymous.invoke(t, "identity.analytics.read", map[string]any{}, false), 401)

	root := loginIAM(t, handler, "root-admin")
	read := func(client iamClient) map[string]any {
		t.Helper()
		response := client.invoke(t, "identity.analytics.read", map[string]any{"timeZone": "America/New_York"}, true)
		checkSucceeded(t, response)
		return decodeBody[gencap.CapabilityResult](t, response).Data.(map[string]any)
	}
	resolve := func(client iamClient) map[string]any {
		t.Helper()
		response := client.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": "analytics"}, true)
		checkSucceeded(t, response)
		view := decodeBody[gencap.CapabilityResult](t, response).Data.(map[string]any)
		if view["applicationVersion"] != "1.0.1.0" {
			t.Fatal(view)
		}
		return view["protocol"].(map[string]any)
	}
	rootData := read(root)
	for _, field := range []string{"accounts", "organizations", "roles", "departments", "trend", "distribution", "recentAccounts", "activity"} {
		if rootData[field] == nil {
			t.Fatalf("missing authorized field %s", field)
		}
	}
	departmentResponse := root.invoke(t, "identity.department.create", map[string]any{
		"name": "Analytics alternate", "code": "analytics-alt", "organizationId": rootAccount.OrganizationID,
		"parentId": 0, "manager": "", "status": "active", "description": "",
	}, true)
	checkSucceeded(t, departmentResponse)
	alternate := decodeData[genidentity.DepartmentRecord](t, departmentResponse)

	roleInput := map[string]any{
		"name": "Analytics reader", "code": "analytics-reader", "description": "", "status": "active",
		"grants": []map[string]any{{"permission": "accounts:read", "effect": "allow", "scope": "OWN"}},
	}
	response := root.invoke(t, "identity.role.create", roleInput, true)
	checkSucceeded(t, response)
	role := decodeData[genidentity.RoleRecord](t, response)
	response = root.invoke(t, "identity.account.create", map[string]any{
		"username": "analytics-reader", "fullName": "Analytics Reader", "email": "private@example.test", "status": "active",
		"organizationId": rootAccount.OrganizationID, "departmentId": rootAccount.DepartmentID,
		"roleIds": []int64{role.ID}, "overrides": []map[string]any{}, "password": "TestOnly!2026Secure",
		"departmentAccess": map[string]any{"memberships": []map[string]any{{
			"departmentId": alternate.ID, "organizationId": rootAccount.OrganizationID,
			"roleIds": []int64{}, "overrides": []any{},
		}}},
	}, true)
	checkSucceeded(t, response)
	reader := loginIAM(t, handler, "analytics-reader")
	protocol := resolve(reader)
	providers := protocol["providers"].([]any)
	if len(providers) != 1 || providers[0].(map[string]any)["permission"] != "authenticated" {
		t.Fatal(providers)
	}
	data := read(reader)
	if len(data) != 4 || data["accounts"].(map[string]any)["total"] != float64(1) ||
		len(data["trend"].([]any)) != 7 || len(data["activity"].([]any)) != 1 {
		t.Fatal(data)
	}
	row := data["recentAccounts"].([]any)[0].(map[string]any)
	if row["username"] != "analytics-reader" || len(row) != 6 {
		t.Fatal(row)
	}

	switchDepartment := func(department int64) {
		t.Helper()
		w := reader.request(t, "POST", "/api/auth/department", map[string]any{"departmentId": department}, true)
		checkStatus(t, w, 200)
		reader.csrf = decodeBody[genidentity.SessionProfile](t, w).CsrfToken
		if cookies := w.Result().Cookies(); len(cookies) > 0 {
			reader.cookie = cookies[0]
		}
	}
	switchDepartment(alternate.ID)
	if data := read(reader); len(data) != 1 || len(data["activity"].([]any)) != 0 {
		t.Fatal("original department grants were merged into the active department", data)
	}
	switchDepartment(rootAccount.DepartmentID)
	if data := read(reader); data["accounts"] == nil {
		t.Fatal("original department scope was not restored", data)
	}

	roleInput["id"] = role.ID
	roleInput["grants"] = []any{}
	checkSucceeded(t, root.invoke(t, "identity.role.update", roleInput, true))
	resolve(reader)
	if denied := read(reader); len(denied) != 1 || len(denied["activity"].([]any)) != 0 {
		t.Fatal("revoked scope remained visible", denied)
	}

	if _, err := identity.Bootstrap(t.Context(), pool, "analytics-other", "analytics-other-admin", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	otherIAM := identity.New(pool, false)
	otherHandler := transport.HandlerWithCore(app.System{}, governance.New(pool, otherIAM, governance.Options{
		EncryptionKey: []byte("01234567890123456789012345678901"), PublicWorkspace: "analytics-other",
	}), otherIAM)
	other := loginIAM(t, otherHandler, "analytics-other-admin")
	resolve(other)
	otherData := read(other)
	if otherData["accounts"].(map[string]any)["total"] != float64(1) ||
		otherData["recentAccounts"].([]any)[0].(map[string]any)["username"] != "analytics-other-admin" {
		t.Fatal("unexpected tenant projection", otherData)
	}
	checkStatus(t, root.invoke(t, "application.runtime.application.disable", map[string]any{
		"applicationId": "analytics", "pluginId": "builtin:analytics", "revision": 1,
	}, true), 403)
	resolve(root)
}

func TestAnalyticsRuntimeUpgradesExistingTenant(t *testing.T) {
	_, pool, _ := iamFixture(t)
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	migrator, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.DownTo(t.Context(), 33); err != nil {
		t.Fatal(err)
	}
	const tenant = "analytics-existing"
	const username = "analytics-existing-admin"
	if _, err := identity.Bootstrap(t.Context(), pool, tenant, username, "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	client := func() iamClient {
		iam := identity.New(pool, false)
		handler := transport.HandlerWithCore(app.System{}, governance.New(pool, iam, governance.Options{
			EncryptionKey: []byte("01234567890123456789012345678901"), PublicWorkspace: tenant,
		}), iam)
		return loginIAM(t, handler, username)
	}
	assertVersion := func(c iamClient, version string) {
		t.Helper()
		w := c.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": "analytics"}, true)
		checkSucceeded(t, w)
		if got := decodeBody[gencap.CapabilityResult](t, w).Data.(map[string]any)["applicationVersion"]; got != version {
			t.Fatalf("expected analytics version %s, got %v", version, got)
		}
	}
	assertVersion(client(), "1.0.0.0")
	if _, err := migrator.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	restarted := client()
	assertVersion(restarted, "1.0.0.0")
	var revision int64
	if err := pool.QueryRow(t.Context(), `SELECT revision FROM runtime_installations WHERE tenant_id=$1 AND plugin_id='builtin:analytics'`, tenant).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	checkSucceeded(t, restarted.invoke(t, "application.runtime.installation.upgrade", map[string]any{
		"pluginId": "builtin:analytics", "revision": revision, "version": "1.0.1",
	}, true))
	assertVersion(restarted, "1.0.1.0")
	assertVersion(client(), "1.0.1.0")
}

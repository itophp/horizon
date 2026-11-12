//go:build integration

package tests

import (
	"net/http"
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

func TestPlatformRuntimeInstallationsAndProviders(t *testing.T) {
	_, pool, _ := iamFixture(t)
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	migrator, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.DownTo(t.Context(), 34); err != nil {
		t.Fatal(err)
	}
	const existing = "platform-existing"
	const fresh = "platform-fresh"
	newHandler := func(tenant string) http.Handler {
		iam := identity.New(pool, false)
		core := governance.New(pool, iam, governance.Options{
			EncryptionKey: []byte("01234567890123456789012345678901"), PublicWorkspace: tenant,
		})
		return transport.HandlerWithCore(app.System{}, core, iam)
	}
	for index, tenant := range []string{existing, fresh} {
		if index == 1 {
			if _, err := migrator.Up(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := identity.Bootstrap(t.Context(), pool, tenant, tenant, "TestOnly!2026Secure"); err != nil {
			t.Fatal(err)
		}
		if tenant == existing {
			// Reconcile the compiled deployment before migration 00035 installs the pages.
			newHandler(tenant)
		}
	}

	routes := map[string]string{
		"system-settings": "/settings", "personal-preferences": "/preferences", "agent-collaboration": "/mcp",
	}
	protocol := func(client iamClient, id string) map[string]any {
		t.Helper()
		res := client.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": id}, true)
		checkSucceeded(t, res)
		return decodeBody[gencap.CapabilityResult](t, res).Data.(map[string]any)["protocol"].(map[string]any)
	}
	for _, tenant := range []string{existing, fresh} {
		handler := newHandler(tenant)
		root := loginIAM(t, handler, tenant)
		for id, route := range routes {
			pages := protocol(root, id)["pages"].([]any)
			if len(pages) != 1 || pages[0].(map[string]any)["route"] != route {
				t.Fatalf("%s has unexpected pages: %v", tenant, pages)
			}
		}
		if providers := protocol(root, "agent-collaboration")["providers"].([]any); len(providers) != 2 {
			t.Fatalf("administrator missing audit provider: %v", providers)
		}
		for _, name := range []string{"mcp.token.list", "mcp.audit.list"} {
			res := root.invoke(t, name, map[string]any{}, true)
			checkSucceeded(t, res)
			if data, ok := decodeBody[gencap.CapabilityResult](t, res).Data.([]any); !ok || len(data) != 0 {
				t.Fatalf("%s must return an empty array: %v", name, data)
			}
		}
		account := decodeBody[genidentity.SessionProfile](t, root.request(t, "GET", "/api/auth/me", nil, false)).Account
		if tenant == existing {
			checkSucceeded(t, root.invoke(t, "mcp.token.create", map[string]any{
				"name": "platform test", "password": "TestOnly!2026Secure", "days": 1, "departmentId": account.DepartmentID,
			}, true))
			res := root.invoke(t, "mcp.token.list", map[string]any{}, true)
			checkSucceeded(t, res)
			rows := decodeBody[gencap.CapabilityResult](t, res).Data.([]any)
			if len(rows) != 1 {
				t.Fatal("expected exactly one personal token")
			}
			row := rows[0].(map[string]any)
			if _, present := row["secret"]; present || row["revokedBy"] != float64(0) {
				t.Fatal("unexpected token list projection")
			}
		}
		restarted := loginIAM(t, newHandler(tenant), tenant)
		for id := range routes {
			protocol(restarted, id)
		}
		if tenant != fresh {
			continue
		}
		readerName := "platform-reader"
		checkSucceeded(t, root.invoke(t, "identity.account.create", map[string]any{
			"username": readerName, "fullName": "Platform Reader", "email": "reader@example.test",
			"status": "active", "organizationId": account.OrganizationID, "departmentId": account.DepartmentID,
			"roleIds": []int64{}, "overrides": []any{}, "password": "TestOnly!2026Secure",
		}, true))
		reader := loginIAM(t, handler, readerName)
		preferences := reader.invoke(t, "application.runtime.resolve", map[string]any{"applicationId": "personal-preferences"}, true)
		checkSucceeded(t, preferences)
		definition := decodeBody[gencap.CapabilityResult](t, preferences).Data.(map[string]any)
		view := definition["protocol"].(map[string]any)
		page := view["pages"].([]any)[0].(map[string]any)
		children := page["root"].(map[string]any)["layout"].(map[string]any)["children"].([]any)
		form := children[1].(map[string]any)["layout"].(map[string]any)["children"].([]any)[0].(map[string]any)["component"].(map[string]any)
		if form["kind"] != "change-list-form" || len(form["events"].([]any)) != 1 {
			t.Fatal("personal preferences require a declared form and projected action")
		}
		checkSucceeded(t, reader.request(t, "POST", "/api/capabilities/settings.preferences.update/invocations", map[string]any{
			"version": "1.0.0", "idempotencyKey": "runtime-test-preferences",
			"input": map[string]any{"changes": []map[string]string{{"key": "display.theme", "operation": "set", "value": "\"dark\""}}},
			"runtime": map[string]any{
				"applicationId": "personal-preferences", "pageId": page["id"],
				"metadataVersion": definition["metadataVersion"], "installationRevision": definition["installationRevision"],
			},
		}, true))
		providers := protocol(reader, "agent-collaboration")["providers"].([]any)
		if len(providers) != 1 || providers[0].(map[string]any)["id"] != "tokens" {
			t.Fatalf("unexpected providers for ordinary account: %v", providers)
		}
		checkSucceeded(t, reader.invoke(t, "mcp.token.list", map[string]any{}, true))
		if result := decodeBody[gencap.CapabilityResult](t, reader.invoke(t, "mcp.audit.list", map[string]any{}, true)); result.Status == "succeeded" {
			t.Fatal("ordinary account read audit data")
		}
	}
}

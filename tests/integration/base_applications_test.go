//go:build integration

package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	gencap "horizon/foundation/gen/capability"
	"horizon/foundation/runtime/app"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
)

// newRuntimeHandler rebuilds the runtime composition so startup reconciliation
// runs with the current deployment contributions.
func newRuntimeHandler(pool *pgxpool.Pool, tenant string) http.Handler {
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{
		EncryptionKey:   []byte("01234567890123456789012345678901"),
		PublicWorkspace: tenant,
	})
	return deploymentHandler(app.System{}, core, iam)
}

func resolveSystemSettings(t *testing.T, client iamClient) map[string]any {
	t.Helper()
	active := invokeCapability(t, client, "application.runtime.resolve", map[string]any{
		"input":          map[string]any{"applicationId": "system-settings"},
		"idempotencyKey": "resolve-system-settings",
	})
	if active.Status != "succeeded" {
		t.Fatal(active)
	}
	return active.Data.(map[string]any)
}

// Platform base applications ship with the deployment: they are always
// installed and enabled, cannot be managed per tenant and always resolve the
// deployment baseline.
func TestPlatformBaseApplicationsAreManagedByDeployment(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	catalog := decodeData[map[string]any](t, client.invoke(t, "application.runtime.installation.list", map[string]any{}, true))
	candidates := catalog["candidates"].([]any)
	baseSeen := map[string]bool{}
	for _, raw := range candidates {
		candidate := raw.(map[string]any)
		pluginID, _ := candidate["pluginId"].(string)
		base, _ := candidate["base"].(bool)
		switch pluginID {
		case "builtin:system-settings", "builtin:ai-settings", "system.identity", "builtin:capabilities", "builtin:analytics", "builtin:agent-collaboration", "builtin:personal-preferences", "builtin:automation":
			if !base {
				t.Fatalf("platform package %s must be classified as base", pluginID)
			}
			baseSeen[pluginID] = true
		case "builtin:articles", "com.horizon.projects":
			if base {
				t.Fatalf("business package %s must stay installable", pluginID)
			}
		}
		if pluginID == "builtin:system-settings" {
			labels := candidate["labelKeys"].(map[string]any)
			if labels["system-settings"] != "governance:settings" {
				t.Fatalf("missing application label projection: %v", labels)
			}
		}
	}
	for _, pluginID := range []string{"builtin:system-settings", "builtin:ai-settings", "system.identity", "builtin:capabilities", "builtin:analytics", "builtin:agent-collaboration", "builtin:personal-preferences"} {
		if !baseSeen[pluginID] {
			t.Fatalf("base package %s missing from the catalog", pluginID)
		}
	}
	items := catalog["items"].([]any)
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["pluginId"] == "builtin:system-settings" {
			if item["status"] != "enabled" || item["version"] != "1.0.4" {
				t.Fatalf("base application is not provisioned: %v", item)
			}
		}
	}
	installation := decodeBody[gencap.CapabilityResult](t, client.invoke(t, "application.runtime.installation.get", map[string]any{"pluginId": "builtin:system-settings"}, true))
	if installation.Status != "succeeded" {
		t.Fatal(installation)
	}
	revision := installation.Data.(map[string]any)["installation"].(map[string]any)["revision"]
	for _, input := range []map[string]any{
		{"pluginId": "builtin:system-settings", "revision": revision},
		{"pluginId": "system.identity", "revision": 2},
	} {
		for _, name := range []string{"application.runtime.installation.disable", "application.runtime.installation.uninstall", "application.runtime.installation.upgrade"} {
			body := map[string]any{"pluginId": input["pluginId"], "revision": input["revision"]}
			if name == "application.runtime.installation.upgrade" {
				body["version"] = "1.0.2"
			}
			response := client.invoke(t, name, body, true)
			if response.Code == 200 {
				result := decodeBody[gencap.CapabilityResult](t, response)
				if result.Status != "failed" || result.ErrorCode != "forbidden" {
					t.Fatalf("%s on %v must be forbidden: %+v", name, input["pluginId"], result)
				}
			} else if response.Code != 403 && response.Code != 400 {
				t.Fatalf("%s on %v: HTTP %d %s", name, input["pluginId"], response.Code, response.Body.String())
			}
		}
	}
	propose := client.invoke(t, "application.runtime.definition.propose", map[string]any{
		"applicationId": "system-settings", "baseVersion": "1.0.4.0", "revision": 1, "changes": []map[string]any{},
	}, true)
	if propose.Code == 200 {
		disallowed := decodeBody[gencap.CapabilityResult](t, propose)
		if disallowed.Status != "failed" || disallowed.ErrorCode != "forbidden" {
			t.Fatalf("base application definitions must not be editable: %+v", disallowed)
		}
	} else if propose.Code != 403 && propose.Code != 400 {
		t.Fatalf("base application definition propose: HTTP %d %s", propose.Code, propose.Body.String())
	}
	design := decodeData[map[string]any](t, client.invoke(t, "application.runtime.definition.list", map[string]any{}, true))
	for _, raw := range design["items"].([]any) {
		if raw.(map[string]any)["applicationId"] == "system-settings" {
			t.Fatal("base applications must not appear in the design catalog")
		}
	}

	// A deployment release advances the installation and republishes its
	// baseline without a tenant action.
	if _, err := pool.Exec(t.Context(), `UPDATE runtime_installations SET version='1.0.2', digest='', previous_version='', revision=revision+1 WHERE tenant_id='tenant-a' AND plugin_id='builtin:system-settings'`); err != nil {
		t.Fatal(err)
	}
	upgraded := loginIAM(t, newRuntimeHandler(pool, "tenant-a"), "root-admin")
	installation = decodeBody[gencap.CapabilityResult](t, upgraded.invoke(t, "application.runtime.installation.get", map[string]any{"pluginId": "builtin:system-settings"}, true))
	got := installation.Data.(map[string]any)["installation"].(map[string]any)
	if got["version"] != "1.0.4" || got["status"] != "enabled" {
		t.Fatalf("base application did not upgrade automatically: %v", got)
	}
	history := installation.Data.(map[string]any)["history"].([]any)
	if len(history) == 0 || history[0].(map[string]any)["Action"] != "auto_upgrade" {
		t.Fatalf("base application upgrade was not recorded: %v", history)
	}
	if resolved := resolveSystemSettings(t, upgraded); resolved["applicationVersion"] != "1.0.4.0" {
		t.Fatalf("base application definition did not advance: %v", resolved["applicationVersion"])
	}

	// A tenant customization of a base definition is replaced by the deployment
	// baseline on the next reconciliation.
	published := resolveSystemSettings(t, upgraded)["applicationVersion"].(string)
	var baseDigest string
	if err := pool.QueryRow(t.Context(), `SELECT base_digest FROM runtime_application_heads WHERE tenant_id='tenant-a' AND application_id='system-settings'`).Scan(&baseDigest); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := pool.QueryRow(t.Context(), `SELECT definition FROM runtime_application_versions WHERE tenant_id='tenant-a' AND application_id='system-settings' AND version=$1`, published).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var definition map[string]any
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	definition["labelKey"] = "custom:base-override"
	customized, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(t.Context(), `INSERT INTO runtime_application_versions(tenant_id,application_id,version,parent_version,base_digest,definition,digest,source,created_by) VALUES('tenant-a','system-settings','1.0.4.1',$1,$2,$3,'custom-base-digest','human',0)`, published, baseDigest, customized); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(t.Context(), `UPDATE runtime_application_heads SET published_version='1.0.4.1', proposal_version='', next_configuration=2, revision=revision+1 WHERE tenant_id='tenant-a' AND application_id='system-settings'`); err != nil {
		t.Fatal(err)
	}
	if custom := resolveSystemSettings(t, upgraded); custom["labelKey"] != "custom:base-override" {
		t.Fatalf("test fixture did not publish the customization: %v", custom["labelKey"])
	}
	reconciled := loginIAM(t, newRuntimeHandler(pool, "tenant-a"), "root-admin")
	restored := resolveSystemSettings(t, reconciled)
	if restored["labelKey"] == "custom:base-override" || restored["applicationVersion"] == "1.0.4.1" {
		t.Fatalf("deployment baseline did not replace the base customization: %v", restored)
	}
}

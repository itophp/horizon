//go:build integration

package tests

import (
	"net/http"
	"strings"
	"testing"

	genidentity "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/identity"
)

func TestRuntimeAIRejectsUnauthorizedAndStaleInvocations(t *testing.T) {
	handler, pool, rootAccount := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	const plugin = "com.horizon.projects"
	upgradeProjects := func(client iamClient) {
		t.Helper()
		installation := invokeCapability(t, client, "application.runtime.installation.get", map[string]any{"input": map[string]any{"pluginId": plugin}})
		current := installation.Data.(map[string]any)["installation"].(map[string]any)
		if current["version"] != "2.0.2" {
			upgrade := invokeCapability(t, client, "application.runtime.installation.upgrade", map[string]any{
				"input": map[string]any{"pluginId": plugin, "version": "2.0.2", "revision": current["revision"]}, "idempotencyKey": "runtime-ai-upgrade",
			})
			if upgrade.Status != "succeeded" {
				t.Fatal(upgrade)
			}
		}
	}
	upgradeProjects(root)
	project := invokeCapability(t, root, "project.create", map[string]any{
		"input": map[string]any{"code": "AI-SCOPE", "name": "AI scope"}, "idempotencyKey": "runtime-ai-project",
	})
	id := project.Data.(map[string]any)["id"]
	resolve := func(client iamClient) map[string]any {
		result := invokeCapability(t, client, "application.runtime.resolve", map[string]any{"input": map[string]any{"applicationId": "projects"}})
		app := result.Data.(map[string]any)
		return map[string]any{
			"applicationId": "projects", "pageId": "project.list",
			"applicationVersion": app["applicationVersion"], "definitionDigest": app["definitionDigest"],
			"metadataVersion": app["metadataVersion"], "catalogRevision": app["catalogRevision"],
			"installationRevision": app["installationRevision"],
		}
	}
	ref := resolve(root)
	body := map[string]any{"version": "1.0.0", "input": map[string]any{"componentId": "project-list", "sceneId": "aiRisk", "recordId": id}}
	path := "/api/capabilities/" + ar.AnalyzeCapability + "/invocations"
	checkStatus(t, root.request(t, http.MethodPost, path, body, true), http.StatusForbidden)

	body["runtime"] = ref
	input := body["input"].(map[string]any)
	input["sceneId"] = "forged"
	result := invokeCapability(t, root, ar.AnalyzeCapability, body)
	if result.Status != "failed" || result.ErrorCode != "not_found" {
		t.Fatal(result)
	}
	input["sceneId"] = "aiRisk"
	input["recordId"] = 900000
	result = invokeCapability(t, root, ar.AnalyzeCapability, body)
	if result.Status != "failed" || result.ErrorCode != "not_found" {
		t.Fatal(result)
	}
	input["recordId"] = id
	ref["definitionDigest"] = strings.Repeat("0", 64)
	checkStatus(t, root.request(t, http.MethodPost, path, body, true), http.StatusConflict)

	roleResponse := root.invoke(t, "identity.role.create", map[string]any{
		"name": "Project reader", "code": "runtime-ai-reader", "description": "", "status": "active",
		"grants": []map[string]any{{"permission": "projects:read", "effect": "allow", "scope": "ALL"}},
	}, true)
	checkSucceeded(t, roleResponse)
	role := decodeData[genidentity.RoleRecord](t, roleResponse)
	checkSucceeded(t, root.invoke(t, "identity.account.create", map[string]any{
		"username": "ai-project-reader", "fullName": "Project reader", "email": "", "status": "active",
		"organizationId": rootAccount.OrganizationID, "departmentId": rootAccount.DepartmentID,
		"roleIds": []int64{role.ID}, "overrides": []map[string]any{}, "password": "TestOnly!2026Secure",
	}, true))
	reader := loginIAM(t, handler, "ai-project-reader")
	body["runtime"] = resolve(reader)
	checkStatus(t, reader.request(t, http.MethodPost, path, body, true), http.StatusForbidden)

	if _, err := identity.Bootstrap(t.Context(), pool, "tenant-ai-other", "ai-other-admin", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	other := loginIAM(t, handler, "ai-other-admin")
	upgradeProjects(other)
	body["runtime"] = resolve(other)
	failed := invokeCapability(t, other, ar.AnalyzeCapability, body)
	if failed.Status != "failed" || failed.ErrorCode != "not_found" {
		t.Fatal(failed)
	}
}

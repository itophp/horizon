//go:build integration

package tests

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"

	gen "horizon/foundation/gen/capability"
	"horizon/foundation/runtime/app"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/settings"
)

func TestSystemSettingsRuntimeUpgradesWithReauthentication(t *testing.T) {
	freshHandler, pool, _ := iamFixture(t)
	fresh := loginIAM(t, freshHandler, "root-admin")
	assertAISettingsPublished(t, fresh, "1.0.3")

	// Bootstrap the older installation in a separate tenant before reconciling.
	if _, err := pool.Exec(t.Context(), `UPDATE runtime_installation_templates SET version = '1.0.2' WHERE plugin_id = 'builtin:system-settings'`); err != nil {
		t.Fatal(err)
	}
	account, err := identity.Bootstrap(t.Context(), pool, "ai-upgrade", "upgrade-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{PublicWorkspace: "ai-upgrade", EncryptionKey: []byte("01234567890123456789012345678901")})
	handler := deploymentHandler(app.System{}, core, iam)
	root := loginIAM(t, handler, "upgrade-admin")
	const plugin = "builtin:system-settings"
	const testKey = "ai-config-test-key"
	temp := map[string]any{"kind": "workspace", "id": 0}
	workspace := func() *settings.SettingsWorkspace {
		reply := root.invoke(t, "settings.workspace.read", map[string]any{"targetKind": "workspace", "targetId": 0}, true)
		if strings.Contains(reply.Body.String(), testKey) {
			t.Fatal("workspace projection exposed the model key")
		}
		data := decodeData[settings.SettingsWorkspace](t, reply)
		return &data
	}
	change := func(key, raw string) map[string]any {
		return map[string]any{"key": key, "value": raw, "operation": "set"}
	}
	draft := func(changes ...map[string]any) map[string]any {
		return map[string]any{"target": temp, "baseRevision": workspace().Revision, "reason": "verify AI settings", "changes": changes}
	}
	assertRejected := func(code string, changes ...map[string]any) {
		reply := root.invoke(t, "settings.draft.create", draft(changes...), true)
		result := decodeBody[gen.CapabilityResult](t, reply)
		if result.Status != "failed" || result.ErrorCode != code {
			t.Fatalf("wanted %s, got %s (%s)", code, result.ErrorCode, result.Status)
		}
	}
	proof := func(id, revision int64, operation string) string {
		return decodeData[settings.ReauthProof](t, root.invoke(t, "settings.password.verify", map[string]any{
			"password": "TestOnly!2026Secure", "operation": operation, "resourceId": id, "revision": revision,
		}, true)).Proof
	}
	publish := func(changes ...map[string]any) settings.ChangeRecord {
		d := decodeData[settings.ChangeRecord](t, root.invoke(t, "settings.draft.create", draft(changes...), true))
		return decodeData[settings.ChangeRecord](t, root.invoke(t, "settings.draft.publish", map[string]any{
			"id": d.ID, "baseRevision": d.BaseRevision, "proof": proof(d.ID, d.BaseRevision, "settings.publish"),
		}, true))
	}
	assignment := change("aiModels.route.projectImport", `"glm"`)
	assertRejected("ai_model_incomplete.glm.projectImport", assignment)
	original := publish(
		change("assistant.token", `"`+testKey+`"`),
		change("aiModels.route.workspace", `"glm"`),
		change("agent.enabled", "true"),
		change("agent.baseUrl", `"http://127.0.0.1:1"`),
		assignment,
	)

	// Platform-managed base applications follow the deployment baseline: the
	// startup reconciliation already aligned the un-customized 1.0.2 installation
	// with 1.0.3, and tenants cannot propose definition revisions for it.
	assertAISettingsPublished(t, root, "1.0.3")
	propose := root.invoke(t, "application.runtime.definition.propose", map[string]any{
		"applicationId": "system-settings", "baseVersion": "1.0.3.0", "revision": 1,
		"changes": []map[string]any{{"section": "navigation", "id": "system-settings.home", "operation": "upsert", "value": `{"pageId":"system-settings.home","labelKey":"governance:settings","icon":"sliders"}`}},
	}, true)
	if propose.Code == 200 {
		denied := decodeBody[gen.CapabilityResult](t, propose)
		if denied.Status != "failed" || denied.ErrorCode != "forbidden" {
			t.Fatalf("base application definition write must be forbidden: %+v", denied)
		}
	} else if propose.Code != 403 && propose.Code != 400 {
		t.Fatalf("base application definition propose: HTTP %d %s", propose.Code, propose.Body.String())
	}
	for _, item := range decodeData[map[string]any](t, root.invoke(t, "application.runtime.definition.list", map[string]any{}, true))["items"].([]any) {
		if item.(map[string]any)["applicationId"] == "system-settings" {
			t.Fatal("base applications must not be designable")
		}
	}
	assertRejected("ai_model_type_unsupported.glm.workspace", change("aiModels.glm.type", `"embedding"`))
	updated := publish(change("aiModels.glm.type", `"multimodal"`))
	principal, err := iam.PrincipalForTask(t.Context(), "ai-upgrade", account.ID, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	checkModel := func(wantType string) {
		t.Helper()
		model, err := core.ResolveAIModel(t.Context(), principal, governance.ModelProjectImport)
		if err != nil || model.Profile != "glm" || model.Token != testKey || model.Type != wantType {
			t.Fatalf("published GLM route, credential, or type mismatch: %v", err)
		}
		workspace()
	}
	checkModel(governance.ModelMultimodal)
	checkSucceeded(t, root.invoke(t, "settings.history.rollback", map[string]any{
		"id": original.ID, "baseRevision": updated.Revision,
		"proof": proof(original.ID, updated.Revision, "settings.rollback"), "reason": "restore model type",
	}, true))
	checkModel(governance.ModelText)
	var imageBuffer bytes.Buffer
	if err := png.Encode(&imageBuffer, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	response := root.request(t, "POST", "/api/ai/runs", map[string]any{
		"question":   "Describe this image",
		"attachment": map[string]any{"name": "fixture.png", "content": base64.StdEncoding.EncodeToString(imageBuffer.Bytes())},
	}, true)
	checkStatus(t, response, http.StatusUnprocessableEntity)
	var problem struct{ Code string }
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil || problem.Code != "ai_model_image_unsupported" {
		t.Fatalf("expected image capability error before sending a model request: HTTP %d %s", response.Code, response.Body.String())
	}
}

func assertAISettingsPublished(t *testing.T, client iamClient, version string) {
	t.Helper()
	active := invokeCapability(t, client, "application.runtime.resolve", map[string]any{
		"input": map[string]any{"applicationId": "system-settings"},
	})
	if active.Status != "succeeded" {
		t.Fatal(active)
	}
	application := active.Data.(map[string]any)
	if applicationVersion, _ := application["applicationVersion"].(string); !strings.HasPrefix(applicationVersion, version+".") {
		t.Fatal(application["applicationVersion"])
	}
	pages := application["protocol"].(map[string]any)["pages"].([]any)
	if len(pages) != 2 {
		t.Fatalf("expected settings and AI configuration pages, got %d", len(pages))
	}
	for _, page := range pages {
		view := page.(map[string]any)
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		switch view["route"] {
		case "/settings":
			if bytes.Contains(raw, []byte(`"aiModels.default"`)) || bytes.Contains(raw, []byte(`"assistant.model"`)) {
				t.Fatal("the general settings form still edits model configuration")
			}
		case "/ai-settings":
			for _, field := range []string{`"ai-model-settings"`, `"settings.password.verify"`} {
				if !bytes.Contains(raw, []byte(field)) {
					t.Errorf("AI configuration page is missing %s", field)
				}
			}
		default:
			t.Fatalf("unexpected page route %v", view["route"])
		}
	}
}

//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"horizon/foundation/db/migrations"
	gen "horizon/foundation/gen/capability"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/settings"
)

func aiConfigChanges(t *testing.T, values map[string]any) []map[string]any {
	t.Helper()
	changes := make([]map[string]any, 0, len(values))
	for key, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		changes = append(changes, map[string]any{"key": key, "value": string(raw), "operation": "set"})
	}
	return changes
}

func saveAIConfig(t *testing.T, client iamClient, revision int64, values map[string]any) {
	t.Helper()
	checkSucceeded(t, client.invoke(t, "ai.config.save", map[string]any{
		"baseRevision": revision, "changes": aiConfigChanges(t, values),
	}, true))
}

func TestAIConfigurationSavesImmediatelyWithIndependentPermissions(t *testing.T) {
	handler, pool, account := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	assertAISettingsPublished(t, root)
	const secret = "test-only-custom-credential"
	read := func(client iamClient) map[string]any {
		t.Helper()
		reply := client.invoke(t, "ai.config.read", map[string]any{}, true)
		if strings.Contains(reply.Body.String(), secret) {
			t.Fatal("AI configuration exposed the model key")
		}
		return decodeData[map[string]any](t, reply)
	}
	reject := func(revision int64, code string, values map[string]any) {
		t.Helper()
		reply := root.invoke(t, "ai.config.save", map[string]any{
			"baseRevision": revision, "changes": aiConfigChanges(t, values),
		}, true)
		result := decodeBody[gen.CapabilityResult](t, reply)
		if result.Status != "failed" || result.ErrorCode != code {
			t.Fatalf("wanted %s, got %s (%s)", code, result.ErrorCode, result.Status)
		}
	}
	if revision := read(root)["revision"]; revision != float64(0) {
		t.Fatalf("unexpected initial revision: %v", revision)
	}
	reject(0, "invalid", map[string]any{"branding.name": "Invalid"})
	reject(0, "ai_model_incomplete.custom-first.models", map[string]any{"aiModels.custom-first.displayName": "Incomplete"})
	for i, profile := range []string{"custom-first", "custom-second"} {
		prefix := "aiModels." + profile + "."
		saveAIConfig(t, root, int64(i), map[string]any{
			prefix + "type": "text", prefix + "displayName": profile,
			prefix + "baseUrl": "https://example.test/v1", prefix + "providerId": "example",
			prefix + "model": profile, prefix + "token": secret,
		})
	}
	reject(1, "conflict", map[string]any{"agent.enabled": false})
	saveAIConfig(t, root, 2, map[string]any{
		"agent.enabled": true, "aiModels.default": "custom-first", "aiModels.route.projectImport": "custom-second",
	})
	config := read(root)
	if config["revision"] != float64(3) {
		t.Fatal("configuration did not advance immediately")
	}
	keys := 0
	for _, entry := range config["entries"].([]any) {
		field := entry.(map[string]any)
		if field["key"] == "aiModels.custom-first.token" || field["key"] == "aiModels.custom-second.token" {
			if field["value"] != "" || field["configured"] != true {
				t.Fatal("configured credential was not redacted")
			}
			keys++
		}
	}
	if keys != 2 {
		t.Fatalf("got %d custom models", keys)
	}

	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	principal, err := iam.PrincipalForTask(t.Context(), "tenant-a", account.ID, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	for use, want := range map[governance.AIModelUse]string{
		governance.ModelWorkspace: "custom-first", governance.ModelProjectImport: "custom-second",
	} {
		model, e := core.ResolveAIModel(t.Context(), principal, use)
		if e != nil || !model.Enabled || model.Name != want || model.Token != secret || model.Protocol != governance.ChatCompletions {
			t.Fatalf("%s did not resolve the saved model: %v", use, e)
		}
	}
	reject(3, "ai_embedding_incomplete", map[string]any{"knowledge.embedding.profile": "bailian"})
	reject(3, "ai_embedding_dashscope_invalid", map[string]any{"aiModels.bailian.baseUrl": "https://example.test/api/v1"})
	notConfigured := root.invoke(t, "knowledge.reindex", map[string]any{}, true)
	reindexResult := decodeBody[gen.CapabilityResult](t, notConfigured)
	if reindexResult.Status != "failed" || reindexResult.ErrorCode != "ai_embedding_incomplete" {
		t.Fatalf("wanted an incomplete embedding configuration error, got %s (%s)", reindexResult.ErrorCode, reindexResult.Status)
	}
	saveAIConfig(t, root, 3, map[string]any{
		"aiModels.bailian.token": secret, "knowledge.embedding.profile": "bailian",
	})
	embedding, err := core.KnowledgeVector(t.Context(), principal)
	if err != nil || embedding.Protocol != governance.DashScopeEmbedding || embedding.Token != secret ||
		embedding.Name != "qwen3-vl-embedding" || embedding.Dimensions != 2560 {
		t.Fatalf("knowledge did not resolve the committed Bailian model: %v", err)
	}
	read(root)

	reject(4, "ai_embedding_dashscope_invalid", map[string]any{"aiModels.bailian.dimensions": 300})
	reject(4, "ai_embedding_model_required", map[string]any{"knowledge.embedding.profile": "custom-first"})
	saveAIConfig(t, root, 4, map[string]any{
		"aiModels.custom-vector.type": "embedding", "aiModels.custom-vector.protocol": "openai-embedding",
		"aiModels.custom-vector.displayName": "Custom vector", "aiModels.custom-vector.providerId": "example",
		"aiModels.custom-vector.baseUrl": "https://example.test/v1", "aiModels.custom-vector.model": "vector-model",
		"aiModels.custom-vector.dimensions": 1024, "aiModels.custom-vector.token": secret,
		"knowledge.embedding.profile": "custom-vector",
	})
	vector, err := core.KnowledgeVector(t.Context(), principal)
	if err != nil || vector.Protocol != governance.OpenAIEmbedding || vector.Name != "vector-model" ||
		vector.Dimensions != 1024 || vector.Token != secret {
		t.Fatalf("custom embedding configuration was not applied: %v", err)
	}

	other, err := identity.Bootstrap(t.Context(), pool, "tenant-b", "other-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	otherClient := loginIAM(t, newRuntimeHandler(pool, "tenant-b"), other.Username)
	otherConfig := read(otherClient)
	if otherConfig["revision"] != float64(0) {
		t.Fatal("AI configuration revision crossed workspace boundary")
	}
	for _, entry := range otherConfig["entries"].([]any) {
		if strings.HasPrefix(entry.(map[string]any)["key"].(string), "aiModels.custom-first.") {
			t.Fatal("custom model crossed workspace boundary")
		}
	}

	workspace := decodeData[settings.SettingsWorkspace](t, root.invoke(t, "settings.workspace.read", map[string]any{"targetKind": "workspace", "targetId": 0}, true))
	for _, entry := range workspace.Entries {
		if governance.IsAISetting(entry.Key) {
			t.Fatalf("system settings still return %s", entry.Key)
		}
	}
	old := root.invoke(t, "settings.draft.create", map[string]any{
		"target": map[string]any{"kind": "workspace", "id": 0}, "baseRevision": workspace.Revision,
		"reason": "Verify system settings isolation", "changes": aiConfigChanges(t, map[string]any{"agent.enabled": false}),
	}, true)
	if result := decodeBody[gen.CapabilityResult](t, old); result.ErrorCode != "invalid" {
		t.Fatalf("system settings accepted an AI write: %s", result.ErrorCode)
	}
}

func TestAIConfigurationMigrationMovesActiveOverridesAndPreservesCredentials(t *testing.T) {
	_, pool, account := iamFixture(t)
	ctx := t.Context()
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 46); err != nil {
		t.Fatal(err)
	}
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	sealed, err := core.Seal("tenant-a", "workspace:0", "aiModels.deepseek.token", `"test-only-legacy-key"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO gov_scopes (tenant_id,scope_key,kind,entity_id)
		VALUES ('tenant-a','workspace:0','workspace',0) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO gov_values (tenant_id,scope_key,config_key,value_json) VALUES
		('tenant-a','workspace:0','aiModels.deepseek.token',to_jsonb($1::text)),
		('tenant-a','workspace:0','agent.enabled','true'),
		('tenant-a','workspace:0','branding.name','"Legacy workspace"')`, sealed); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	principal, err := iam.PrincipalForTask(ctx, "tenant-a", account.ID, account.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	model, err := core.ResolveAIModel(ctx, principal, governance.ModelWorkspace)
	if err != nil || !model.Enabled || model.Token != "test-only-legacy-key" {
		t.Fatalf("migrated credential was not usable: %v", err)
	}
	var general, legacyAI int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE config_key = 'branding.name'),
		count(*) FILTER (WHERE config_key IN ('aiModels.deepseek.token','agent.enabled'))
		FROM gov_values WHERE tenant_id = 'tenant-a'`).Scan(&general, &legacyAI); err != nil || general != 1 || legacyAI != 0 {
		t.Fatalf("unexpected split of persisted settings: system=%d, AI=%d, err=%v", general, legacyAI, err)
	}
	if _, err := provider.DownTo(ctx, 46); err != nil {
		t.Fatal(err)
	}
	var restored string
	if err := pool.QueryRow(ctx, `SELECT value_json #>> '{}' FROM gov_values
		WHERE tenant_id = 'tenant-a' AND scope_key = 'workspace:0' AND config_key = 'aiModels.deepseek.token'`).Scan(&restored); err != nil || restored != sealed {
		t.Fatalf("down migration lost the encrypted override: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
}

func assertAISettingsPublished(t *testing.T, client iamClient) {
	t.Helper()
	for id, version := range map[string]string{"system-settings": "1.0.4", "ai-settings": "1.0.3"} {
		active := invokeCapability(t, client, "application.runtime.resolve", map[string]any{
			"input": map[string]any{"applicationId": id},
		})
		if active.Status != "succeeded" {
			t.Fatal(active)
		}
		application := active.Data.(map[string]any)
		if applicationVersion, _ := application["applicationVersion"].(string); !strings.HasPrefix(applicationVersion, version+".") {
			t.Fatal(application["applicationVersion"])
		}
		pages := application["protocol"].(map[string]any)["pages"].([]any)
		if len(pages) != 1 {
			t.Fatalf("%s: expected one page, got %d", id, len(pages))
		}
		view := pages[0].(map[string]any)
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		if id == "system-settings" {
			if view["route"] != "/settings" {
				t.Fatal(view["route"])
			}
			for _, field := range []string{`"aiModels.default"`, `"assistant.model"`, `"agent.enabled"`, `"knowledge.embedding.model"`} {
				if bytes.Contains(raw, []byte(field)) {
					t.Errorf("system settings still edits %s", field)
				}
			}
			continue
		}
		if view["route"] != "/ai-settings" {
			t.Fatal(view["route"])
		}
		for _, obsolete := range []string{"settings.draft.create", "settings.workspace.read", "workspace-schedules", "aiSettings.tabs.releases", "knowledge.embedding.model", "knowledge.embedding.token"} {
			if bytes.Contains(raw, []byte(obsolete)) {
				t.Fatalf("AI configuration uses legacy system settings: %s", obsolete)
			}
		}
		for _, field := range []string{`"ai-model-settings"`, `"agent.enabled"`, `"knowledge.embedding.profile"`, `"ai.config.save@1.0.0"`} {
			if !bytes.Contains(raw, []byte(field)) {
				t.Errorf("AI configuration page is missing %s", field)
			}
		}
	}
}

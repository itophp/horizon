//go:build integration

package tests

import (
	dict "baseline/services/api/gen/dictionaries"
	filegen "baseline/services/api/gen/files"
	genidentity "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/settings"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/transport"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestGovernancePersistenceAndBoundaries(t *testing.T) {
	_, pool, account := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	options := governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir(), PublicWorkspace: "tenant-a"}
	iam := identity.New(pool, false)
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, options, iam)
	root := loginIAM(t, handler, "root-admin")
	target := &gen.ConfigTarget{Kind: "workspace"}
	workspace := func() gen.SettingsWorkspace {
		w := root.request(t, "GET", "/api/admin/settings", nil, false)
		checkStatus(t, w, 200)
		return decodeBody[gen.SettingsWorkspace](t, w)
	}
	draft := func(changes ...*gen.ChangeEntry) gen.ChangeRecord {
		w := root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: target, BaseRevision: workspace().Revision, Reason: "Integration test change", Changes: changes}, true)
		checkStatus(t, w, 201)
		return decodeBody[gen.ChangeRecord](t, w)
	}
	proof := func(operation string, id, rev int64) string {
		w := root.request(t, "POST", "/api/auth/reauthenticate", &gen.ReauthenticatePayload{Password: "TestOnly!2026Secure", Operation: operation, ResourceID: id, Revision: rev}, true)
		checkStatus(t, w, 200)
		return decodeBody[gen.ReauthProof](t, w).Proof
	}
	publish := func(d gen.ChangeRecord) gen.ChangeRecord {
		token := ""
		if d.Sensitive {
			token = proof("settings.publish", d.ID, d.BaseRevision)
		}
		w := root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: d.BaseRevision, Proof: token}, true)
		checkStatus(t, w, 200)
		return decodeBody[gen.ChangeRecord](t, w)
	}
	t.Run("draft does not change effective values and conflict is atomic", func(t *testing.T) {
		a := draft(&gen.ChangeEntry{Key: "branding.name", Operation: "set", Value: `"Governance"`})
		b := draft(&gen.ChangeEntry{Key: "branding.name", Operation: "set", Value: `"Stale"`})
		before := root.request(t, "GET", "/api/auth/configuration", nil, false)
		checkStatus(t, before, 200)
		if strings.Contains(before.Body.String(), "Governance") {
			t.Fatal("draft leaked into runtime")
		}
		v := publish(a)
		if v.Revision != 1 {
			t.Fatal("missing version")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", b.ID), &gen.PublishPayload{BaseRevision: b.BaseRevision}, true), 409)
		after := root.request(t, "GET", "/api/auth/configuration", nil, false)
		if !strings.Contains(after.Body.String(), "Governance") {
			t.Fatal("published brand not effective")
		}
	})
	t.Run("encrypted secrets and single-use bound reauthentication", func(t *testing.T) {
		d := draft(&gen.ChangeEntry{Key: "smtp.password", Operation: "set", Value: `"private-mail-secret"`})
		if strings.Contains(fmt.Sprintf("%+v", d), "private-mail-secret") {
			t.Fatal("secret returned")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: d.BaseRevision}, true), 403)
		token := proof("settings.publish", d.ID, d.BaseRevision)
		w := root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: d.BaseRevision, Proof: token}, true)
		checkStatus(t, w, 200)
		var raw string
		if err := pool.QueryRow(context.Background(), "SELECT value_json::text FROM gov_values WHERE config_key='smtp.password'").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, "private-mail-secret") || !strings.Contains(raw, "enc:v1:") {
			t.Fatal("unencrypted secret")
		}
		for _, path := range []string{"/api/admin/settings", "/api/admin/settings/export"} {
			response := root.request(t, "GET", path, nil, false)
			checkStatus(t, response, 200)
			if strings.Contains(response.Body.String(), "private-mail-secret") || strings.Contains(response.Body.String(), "enc:v1:") {
				t.Fatal("secret projection leaked")
			}
		}
		v := decodeBody[gen.ChangeRecord](t, w)
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/history/%d/rollback", v.ID), &gen.RollbackPayload{BaseRevision: v.Revision, Proof: token, Reason: "Reject reuse of credential"}, true), 403)
	})
	t.Run("files retain immutable storage version and enforce content type", func(t *testing.T) {
		upload := func(name string, data []byte) filegen.ManagedFile {
			w := root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: name, Purpose: "attachment", Content: data}, true)
			checkStatus(t, w, 201)
			return decodeBody[filegen.ManagedFile](t, w)
		}
		a := upload("before.txt", []byte("before storage change"))
		publish(draft(&gen.ChangeEntry{Key: "storage.directory", Operation: "set", Value: `"new-location"`}))
		b := upload("after.txt", []byte("after storage change"))
		if a.StorageVersion == b.StorageVersion {
			t.Fatal("profile not versioned")
		}
		w := root.request(t, "GET", fmt.Sprintf("/api/admin/files/%d/content", a.ID), nil, false)
		checkStatus(t, w, 200)
		if got := decodeBody[filegen.FileContent](t, w); string(got.Content) != "before storage change" {
			t.Fatal("old file unreadable")
		}
		checkStatus(t, root.request(t, "POST", "/api/admin/files", map[string]string{"name": "attack.png", "purpose": "logo", "content": base64.StdEncoding.EncodeToString([]byte("not a png"))}, true), 400)
		checkStatus(t, root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: target, BaseRevision: workspace().Revision, Reason: "Reject directory traversal", Changes: []*gen.ChangeEntry{{Key: "storage.directory", Operation: "set", Value: `"../../outside"`}}}, true), 400)
	})
	t.Run("dictionary stable codes, import validation and runtime history", func(t *testing.T) {
		doc := &dict.DictionaryDocument{Definition: &dict.DictionaryDefinition{Code: "test.teams", NameEn: "Teams", NameZh: "团队", Category: "test", Kind: "tree", Provider: "", Status: "active", Visibility: "shared", AllowExtensions: false, Target: &dict.ConfigTarget{Kind: "workspace"}}, Items: []*dict.DictionaryItem{{Code: "root", LabelEn: "Root", LabelZh: "根", Status: "active"}, {Code: "child", LabelEn: "Child", LabelZh: "子", ParentCode: "root", Status: "active"}}}
		w := root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: doc, Reason: "Create test dictionary"}, true)
		checkStatus(t, w, 201)
		d := decodeBody[dict.ChangeRecord](t, w)
		w = root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", d.ID), &dict.PublishPayload{}, true)
		checkStatus(t, w, 200)
		v := decodeBody[dict.ChangeRecord](t, w)
		id := v.Document.Definition.ID
		options := root.request(t, "GET", "/api/auth/dictionary-options/test.teams?language=zh-CN", nil, false)
		checkStatus(t, options, 200)
		if !strings.Contains(options.Body.String(), "子") {
			t.Fatal("localized option missing")
		}
		preview := root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/%d/import-preview", id), &dict.ImportPayload{Format: "csv", Content: "code,labelEn,parentCode\na,A,b\nb,B,a\n"}, true)
		checkStatus(t, preview, 200)
		if len(decodeBody[dict.ImportPreview](t, preview).Errors) == 0 {
			t.Fatal("cycle accepted")
		}
		export := root.request(t, "GET", fmt.Sprintf("/api/admin/dictionaries/%d/export?format=csv", id), nil, false)
		checkStatus(t, export, 200)
		exported := decodeBody[dict.ExportResult](t, export)
		preview = root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/%d/import-preview", id), &dict.ImportPayload{Format: "csv", Content: exported.Content}, true)
		checkStatus(t, preview, 200)
		if len(decodeBody[dict.ImportPreview](t, preview).Errors) > 0 {
			t.Fatal("export cannot roundtrip")
		}
		v.Document.Items = v.Document.Items[:1]
		checkStatus(t, root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: v.Document, BaseRevision: 1, Reason: "Attempt to drop stable code"}, true), 409)
	})
	t.Run("ordinary user own preferences and messages do not grant administration", func(t *testing.T) {
		w := root.request(t, "POST", "/api/admin/accounts", &genidentity.CreateAccountPayload{Username: "governance-user", FullName: "User", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, Status: "active", Password: "TestOnly!2026Secure", RoleIds: []int64{}, Overrides: []*genidentity.PermissionGrant{}}, true)
		checkStatus(t, w, 201)
		user := loginIAM(t, handler, "governance-user")
		for _, path := range []string{"/api/admin/settings", "/api/admin/dictionaries", "/api/admin/files", "/api/admin/notifications/deliveries"} {
			checkStatus(t, user.request(t, "GET", path, nil, false), 403)
		}
		checkStatus(t, user.request(t, "PUT", "/api/auth/preferences", &gen.PreferencesPayload{Changes: []*gen.ChangeEntry{{Key: "display.theme", Operation: "set", Value: `"light"`}}}, true), 200)
		checkStatus(t, user.request(t, "PUT", "/api/auth/preferences", &gen.PreferencesPayload{Changes: []*gen.ChangeEntry{{Key: "security.passwordMin", Operation: "set", Value: `8`}}}, true), 400)
		checkStatus(t, user.request(t, "GET", "/api/auth/notifications", nil, false), 200)
		w = user.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Minutes: 30, Reason: "Need access for test case", Grants: []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "OWN", DepartmentIds: []int64{}}}}, true)
		checkStatus(t, w, 200)
		worker := notifications.New(governance.New(pool, iam, options))
		if err := worker.ProcessOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		inbox := root.request(t, "GET", "/api/auth/notifications", nil, false)
		checkStatus(t, inbox, 200)
		if !strings.Contains(inbox.Body.String(), "requested") {
			t.Fatal("request notification absent")
		}
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM gov_notifications").Scan(&count); err != nil || count < 1 {
			t.Fatal("notification not durable", err)
		}
		if err := worker.ProcessOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		var next int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM gov_notifications").Scan(&next); err != nil || next != count {
			t.Fatal("event duplicated", err)
		}
	})
}

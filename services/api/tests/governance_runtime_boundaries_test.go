//go:build integration

package tests

import (
	dict "baseline/services/api/gen/dictionaries"
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
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestDynamicDictionaryUsesSourcePermissionAndUploadUsesPublishedLimit(t *testing.T) {
	_, pool, account := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, governance.Options{StorageRoot: t.TempDir()}, identity.New(pool, false))
	root := loginIAM(t, handler, "root-admin")
	w := root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: "options-only", FullName: "Options only", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, Status: "active", Password: "TestOnly!2026Secure", RoleIds: []int64{}, Overrides: []*iamgen.PermissionGrant{{Permission: "dictionaries:use", Effect: "allow", Scope: "DEPARTMENT", DepartmentIds: []int64{}}}}, true)
	checkStatus(t, w, 201)
	user := loginIAM(t, handler, "options-only")
	document := &dict.DictionaryDocument{Definition: &dict.DictionaryDefinition{Code: "dynamic.accounts", NameEn: "Account choices", Category: "test", Kind: "dynamic", Provider: "accounts", Status: "active", Visibility: "shared", Target: &dict.ConfigTarget{Kind: "workspace"}}, Items: []*dict.DictionaryItem{}}
	w = root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: document, Reason: "Register controlled account provider"}, true)
	checkStatus(t, w, 201)
	draft := decodeBody[dict.ChangeRecord](t, w)
	w = root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", draft.ID), &dict.PublishPayload{}, true)
	checkStatus(t, w, 200)
	published := decodeBody[dict.ChangeRecord](t, w)
	checkStatus(t, user.request(t, "GET", "/api/auth/dictionary-options/dynamic.accounts", nil, false), 403)
	w = root.request(t, "GET", "/api/auth/dictionary-options/dynamic.accounts", nil, false)
	checkStatus(t, w, 200)
	if strings.Contains(w.Body.String(), `"email"`) || strings.Contains(w.Body.String(), `"password"`) {
		t.Fatal("dynamic source exposed entity fields")
	}
	checkStatus(t, root.request(t, "GET", fmt.Sprintf("/api/admin/dictionaries/%d/export", published.Document.Definition.ID), nil, false), 403)
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/%d/import-preview", published.Document.Definition.ID), &dict.ImportPayload{Format: "csv", Content: "code,labelEn\nx,X\n"}, true), 403)
	w = root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, Reason: "Tighten attachment upload limit", Changes: []*gen.ChangeEntry{{Key: "files.maxBytes", Operation: "set", Value: "1024"}}}, true)
	checkStatus(t, w, 201)
	settingsDraft := decodeBody[gen.ChangeRecord](t, w)
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", settingsDraft.ID), &gen.PublishPayload{}, true), 200)
	checkStatus(t, root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "oversize.txt", Purpose: "attachment", Content: bytes.Repeat([]byte("a"), 1025)}, true), 400)
	w = root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "within-limit.txt", Purpose: "attachment", Content: bytes.Repeat([]byte("a"), 1024)}, true)
	checkStatus(t, w, 201)
	checkStatus(t, user.request(t, "PUT", "/api/auth/preferences", &gen.PreferencesPayload{Changes: []*gen.ChangeEntry{{Key: "display.hour24", Operation: "set", Value: "null"}}}, true), 400)
}

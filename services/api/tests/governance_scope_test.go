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
	"context"
	"fmt"
	"testing"
)

func TestGovernanceSharedConfigurationAndResourceIsolation(t *testing.T) {
	_, pool, account := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	options := governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir()}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, options, identity.New(pool, false))
	root := loginIAM(t, handler, "root-admin")
	snapshot := decodeBody[iamgen.AdminSnapshot](t, root.request(t, "GET", "/api/admin/snapshot", nil, false))
	var other int64
	for _, d := range snapshot.Departments {
		if d.ID != account.DepartmentID {
			other = d.ID
		}
	}
	var enterprise int64
	for _, o := range snapshot.Organizations {
		if o.Kind == "enterprise" {
			enterprise = o.ID
		}
	}
	w := root.request(t, "POST", "/api/admin/organizations", &iamgen.CreateOrganizationPayload{Name: "Governance company B", Code: "gov-company-b", Kind: "company", ParentID: enterprise, Status: "active"}, true)
	checkStatus(t, w, 201)
	companyB := decodeBody[iamgen.OrganizationRecord](t, w)
	w = root.request(t, "POST", "/api/admin/departments", &iamgen.CreateDepartmentPayload{Name: "Governance department B", Code: "gov-dept-b", OrganizationID: companyB.ID, Status: "active"}, true)
	checkStatus(t, w, 201)
	other = decodeBody[iamgen.DepartmentRecord](t, w).ID
	grants := []*iamgen.PermissionGrant{}
	for _, code := range []string{"settings:read", "settings:update", "settings:publish", "settings:history", "settings:export", "dictionaries:read", "dictionaries:update", "dictionaries:publish", "dictionaries:use", "dictionaries:history", "dictionaries:import", "dictionaries:export"} {
		grants = append(grants, &iamgen.PermissionGrant{Permission: code, Effect: "allow", Scope: "DEPARTMENT", DepartmentIds: []int64{}})
	}
	grants = append(grants, &iamgen.PermissionGrant{Permission: "files:read", Effect: "allow", Scope: "OWN", DepartmentIds: []int64{}}, &iamgen.PermissionGrant{Permission: "files:delete", Effect: "allow", Scope: "OWN", DepartmentIds: []int64{}})
	w = root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: "scope-governance", FullName: "Scoped governance", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, Status: "active", Password: "TestOnly!2026Secure", RoleIds: []int64{}, Overrides: grants, DepartmentAccess: &iamgen.DepartmentAccess{Memberships: []*iamgen.DepartmentMembership{{DepartmentID: other, OrganizationID: companyB.ID, RoleIds: []int64{}, Overrides: grants}}}}, true)
	checkStatus(t, w, 201)
	user := loginIAM(t, handler, "scope-governance")
	publish := func(kind string, id int64, base int64, changes []*gen.ChangeEntry) gen.ChangeRecord {
		w := root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: kind, ID: id}, BaseRevision: base, Reason: "Scope test configuration", Changes: changes}, true)
		checkStatus(t, w, 201)
		d := decodeBody[gen.ChangeRecord](t, w)
		w = root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: base}, true)
		checkStatus(t, w, 200)
		return decodeBody[gen.ChangeRecord](t, w)
	}
	publish("workspace", 0, 0, []*gen.ChangeEntry{{Key: "display.theme", Operation: "set", Value: `"light"`}, {Key: "display.hour24", Operation: "set", Value: "false"}})
	// Legacy department grants cannot become workspace administration implicitly.
	checkStatus(t, user.request(t, "GET", "/api/admin/settings", nil, false), 403)
	checkStatus(t, root.request(t, "GET", fmt.Sprintf("/api/admin/settings?targetKind=department&targetId=%d", other), nil, false), 400)
	checkStatus(t, root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "department", ID: other}, Reason: "Reject obsolete configuration scope", Changes: []*gen.ChangeEntry{{Key: "display.theme", Operation: "set", Value: `"dark"`}}}, true), 400)
	checkStatus(t, user.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, Reason: "Reject shared owner escalation", Changes: []*gen.ChangeEntry{{Key: "branding.name", Operation: "set", Value: `"Escalation"`}}}, true), 403)
	value := func(client iamClient, key string) gen.ConfigEntry {
		response := client.request(t, "GET", "/api/auth/configuration", nil, false)
		checkStatus(t, response, 200)
		for _, e := range decodeBody[gen.EffectiveConfiguration](t, response).Entries {
			if e.Key == key {
				return *e
			}
		}
		t.Fatal("missing configuration", key)
		return gen.ConfigEntry{}
	}
	if value(user, "display.theme").Value != `"light"` || value(user, "display.hour24").Value != "false" {
		t.Fatal("shared configuration not applied")
	}
	w = user.request(t, "POST", "/api/auth/department", &iamgen.SwitchDepartmentPayload{DepartmentID: other}, true)
	checkStatus(t, w, 200)
	user.csrf = decodeBody[iamgen.SessionProfile](t, w).CsrfToken
	if value(user, "display.theme").Value != `"light"` {
		t.Fatal("department switch changed shared configuration")
	}
	checkStatus(t, user.request(t, "GET", fmt.Sprintf("/api/admin/settings?targetKind=department&targetId=%d", account.DepartmentID), nil, false), 400)
	doc := &dict.DictionaryDocument{Definition: &dict.DictionaryDefinition{Code: "scope.test", NameEn: "Scoped options", NameZh: "范围选项", Category: "test", Kind: "tree", Status: "active", Visibility: "shared", Target: &dict.ConfigTarget{Kind: "workspace"}, AllowExtensions: false}, Items: []*dict.DictionaryItem{{Code: "parent", LabelEn: "Parent", Status: "active"}, {Code: "child", LabelEn: "Child", ParentCode: "parent", Status: "active"}}}
	w = root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: doc, Reason: "Create inherited dictionary"}, true)
	checkStatus(t, w, 201)
	draft := decodeBody[dict.ChangeRecord](t, w)
	w = root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", draft.ID), &dict.PublishPayload{}, true)
	checkStatus(t, w, 200)
	version := decodeBody[dict.ChangeRecord](t, w)
	id := version.Document.Definition.ID
	item := version.Document.Items[0]
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/%d/overrides", id), &dict.OverridePayload{Target: &dict.ConfigTarget{Kind: "department", ID: other}, Reason: "Reject obsolete dictionary overrides", Overrides: []*dict.DictionaryOverride{}, Extensions: []*dict.DictionaryItem{}}, true), 400)
	w = user.request(t, "GET", "/api/auth/dictionary-options/scope.test", nil, false)
	checkStatus(t, w, 200)
	if len(decodeBody[[]dict.DictionaryOption](t, w)) != 2 {
		t.Fatal("shared dictionary unavailable in another company")
	}
	version.Document.Items[0].Status = "inactive"
	w = root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: version.Document, BaseRevision: version.Revision, Reason: "Disable shared parent"}, true)
	checkStatus(t, w, 201)
	update := decodeBody[dict.ChangeRecord](t, w)
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", update.ID), &dict.PublishPayload{BaseRevision: version.Revision}, true), 200)
	w = user.request(t, "GET", "/api/auth/dictionary-options/scope.test", nil, false)
	checkStatus(t, w, 200)
	if len(decodeBody[[]dict.DictionaryOption](t, w)) != 0 {
		t.Fatal("inactive parent descendants selectable")
	}
	w = user.request(t, "GET", fmt.Sprintf("/api/auth/dictionary-options/scope.test?ids=%d", item.ID), nil, false)
	checkStatus(t, w, 200)
	historical := decodeBody[[]dict.DictionaryOption](t, w)
	if len(historical) != 1 || !historical[0].Disabled {
		t.Fatal("historical option not resolved")
	}
	w = root.request(t, "GET", "/api/auth/dictionary-options/scope.test", nil, false)
	checkStatus(t, w, 200)
	if len(decodeBody[[]dict.DictionaryOption](t, w)) != 0 {
		t.Fatal("shared change not applied across departments")
	}
	checkStatus(t, user.request(t, "GET", fmt.Sprintf("/api/admin/dictionaries/%d/export", id), nil, false), 404)
	checkStatus(t, user.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/%d/import-preview", id), &dict.ImportPayload{Format: "csv", Content: "code,labelEn\nnew,New\n"}, true), 404)
	w = root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "private.txt", Purpose: "attachment", Content: []byte("private")}, true)
	checkStatus(t, w, 201)
	file := decodeBody[filegen.ManagedFile](t, w)
	checkStatus(t, user.request(t, "GET", fmt.Sprintf("/api/admin/files/%d/content", file.ID), nil, false), 404)
	checkStatus(t, user.request(t, "DELETE", fmt.Sprintf("/api/admin/files/%d", file.ID), nil, true), 404)
	if _, err = identity.Bootstrap(context.Background(), pool, "tenant-b", "other-root", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	foreign := loginIAM(t, handler, "other-root")
	for _, path := range []string{fmt.Sprintf("/api/admin/files/%d/content", file.ID), fmt.Sprintf("/api/admin/dictionaries/%d", id)} {
		checkStatus(t, foreign.request(t, "GET", path, nil, false), 404)
	}
	checkStatus(t, foreign.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", draft.ID), &dict.PublishPayload{}, true), 404)
}

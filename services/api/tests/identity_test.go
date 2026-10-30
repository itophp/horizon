//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/transport"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type iamClient struct {
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func (c iamClient) request(t *testing.T, method, path string, payload any, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	body := ""
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = string(raw)
	}
	r := httptest.NewRequest(method, "http://console.test"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://console.test")
	r.Header.Set("X-Tenant-ID", "attacker-controlled")
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	if csrf {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)
	return w
}
func checkStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("HTTP got %d, want %d: %s", w.Code, want, w.Body.String())
	}
}
func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func loginIAM(t *testing.T, handler http.Handler, username string) iamClient {
	t.Helper()
	c := iamClient{handler: handler}
	w := c.request(t, "POST", "/api/auth/login", map[string]string{"username": username, "password": "TestOnly!2026Secure"}, false)
	checkStatus(t, w, 200)
	result := decodeBody[genidentity.LoginResult](t, w)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || len(cookies[0].Value) != 64 {
		t.Fatal("unsafe or missing session cookie")
	}
	if strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "Set-Cookie") {
		t.Fatal("credential leaked into response")
	}
	return iamClient{handler: handler, cookie: cookies[0], csrf: result.Profile.CsrfToken}
}
func iamFixture(t *testing.T) (http.Handler, *pgxpool.Pool, *genidentity.AccountRecord) {
	t.Helper()
	source := os.Getenv("TEST_DATABASE_URL")
	if source == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("iam_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	dsn := parsed.String()
	if err = identity.Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	root, err := identity.Bootstrap(ctx, pool, "tenant-a", "root-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := transport.Handler(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, identity.New(pool, false))
	return handler, pool, root
}
func TestPersistentIdentityAuthorization(t *testing.T) {
	handler, pool, rootAccount := iamFixture(t)
	anonymous := iamClient{handler: handler}
	checkStatus(t, anonymous.request(t, "GET", "/api/admin/snapshot", nil, false), 401)
	checkStatus(t, anonymous.request(t, "POST", "/api/auth/login", map[string]string{"username": "root-admin", "password": "incorrect"}, false), 401)
	root := loginIAM(t, handler, "root-admin")
	initial := decodeBody[genidentity.AdminSnapshot](t, root.request(t, "GET", "/api/admin/snapshot", nil, false))
	if len(initial.Accounts) != 1 || len(initial.Organizations) != 2 || len(initial.Roles) != 2 {
		t.Fatal("bootstrap did not persist expected records")
	}
	t.Run("origin and CSRF enforce write boundaries", func(t *testing.T) {
		payload := &genidentity.CreateOrganizationPayload{Name: "Extra", Code: "extra", Kind: "enterprise", Status: "active"}
		checkStatus(t, root.request(t, "POST", "/api/admin/organizations", payload, false), 403)
		r := httptest.NewRequest("POST", "http://console.test/api/admin/organizations", strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://attacker.test")
		r.Header.Set("X-CSRF-Token", root.csrf)
		r.AddCookie(root.cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		checkStatus(t, w, 403)
	})
	company := rootAccount.OrganizationID
	department := rootAccount.DepartmentID
	var engineering genidentity.DepartmentRecord
	var enterprise genidentity.OrganizationRecord
	for _, dept := range initial.Departments {
		if dept.ID != department {
			engineering = *dept
		}
	}
	for _, org := range initial.Organizations {
		if org.Kind == "enterprise" {
			enterprise = *org
		}
	}
	w := root.request(t, "POST", "/api/admin/departments", &genidentity.CreateDepartmentPayload{Name: "Quality", Code: "quality", OrganizationID: company, ParentID: engineering.ID, Status: "active"}, true)
	checkStatus(t, w, 201)
	child := decodeBody[genidentity.DepartmentRecord](t, w)
	w = root.request(t, "POST", "/api/admin/organizations", &genidentity.CreateOrganizationPayload{Name: "Another company", Code: "another-company", Kind: "company", ParentID: enterprise.ID, Status: "active"}, true)
	checkStatus(t, w, 201)
	otherCompany := decodeBody[genidentity.OrganizationRecord](t, w)
	w = root.request(t, "POST", "/api/admin/departments", &genidentity.CreateDepartmentPayload{Name: "Outside", Code: "outside", OrganizationID: otherCompany.ID, Status: "active"}, true)
	checkStatus(t, w, 201)
	outsideDept := decodeBody[genidentity.DepartmentRecord](t, w)
	grants := []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}, {Permission: "accounts:create", Effect: "allow", Scope: "DEPARTMENT_TREE", DepartmentIds: []int64{}}, {Permission: "accounts:update", Effect: "allow", Scope: "DEPARTMENT_TREE", DepartmentIds: []int64{}}, {Permission: "accounts:delete", Effect: "allow", Scope: "DEPARTMENT_TREE", DepartmentIds: []int64{}}}
	roleInput := &genidentity.CreateRolePayload{Name: "Scoped manager", Code: "scoped-manager", Status: "active", Grants: grants}
	w = root.request(t, "POST", "/api/admin/roles", roleInput, true)
	checkStatus(t, w, 201)
	role := decodeBody[genidentity.RoleRecord](t, w)
	createAccount := func(username string, org, dept int64, roles []int64) genidentity.AccountRecord {
		payload := &genidentity.CreateAccountPayload{Username: username, FullName: username, Status: "active", OrganizationID: org, DepartmentID: dept, RoleIds: roles, Overrides: []*genidentity.PermissionGrant{}, Password: "TestOnly!2026Secure"}
		w := root.request(t, "POST", "/api/admin/accounts", payload, true)
		checkStatus(t, w, 201)
		if strings.Contains(w.Body.String(), "passwordHash") {
			t.Fatal("password hash leaked")
		}
		return decodeBody[genidentity.AccountRecord](t, w)
	}
	manager := createAccount("manager", company, engineering.ID, []int64{role.ID})
	engineer := createAccount("engineer", company, engineering.ID, []int64{})
	childAccount := createAccount("quality-member", company, child.ID, []int64{})
	outside := createAccount("outside-member", otherCompany.ID, outsideDept.ID, []int64{})
	managerClient := loginIAM(t, handler, "manager")
	updateManager := func(overrides []*genidentity.PermissionGrant, status, password string) {
		payload := &genidentity.UpdateAccountPayload{ID: manager.ID, Username: manager.Username, FullName: manager.FullName, Status: status, OrganizationID: company, DepartmentID: engineering.ID, RoleIds: []int64{role.ID}, Overrides: overrides, Password: password}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", manager.ID), payload, true), 200)
	}
	t.Run("all six scopes are evaluated in SQL after account overrides", func(t *testing.T) {
		for _, tc := range []struct {
			scope       string
			departments []int64
			want        []int64
		}{{"ALL", []int64{}, []int64{rootAccount.ID, manager.ID, engineer.ID, childAccount.ID, outside.ID}}, {"ORGANIZATION", []int64{}, []int64{rootAccount.ID, manager.ID, engineer.ID, childAccount.ID}}, {"DEPARTMENT", []int64{}, []int64{manager.ID, engineer.ID}}, {"DEPARTMENT_TREE", []int64{}, []int64{manager.ID, engineer.ID, childAccount.ID}}, {"OWN", []int64{}, []int64{manager.ID}}, {"CUSTOM", []int64{child.ID}, []int64{childAccount.ID}}} {
			t.Run(tc.scope, func(t *testing.T) {
				updateManager([]*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: tc.scope, DepartmentIds: tc.departments}}, "active", "")
				response := managerClient.request(t, "GET", "/api/admin/snapshot", nil, false)
				checkStatus(t, response, 200)
				snapshot := decodeBody[genidentity.AdminSnapshot](t, response)
				ids := map[int64]bool{}
				for _, a := range snapshot.Accounts {
					ids[a.ID] = true
				}
				if len(ids) != len(tc.want) {
					t.Fatalf("scope returned unexpected rows: %v", ids)
				}
				for _, id := range tc.want {
					if !ids[id] {
						t.Fatalf("scope omitted authorized ID %d", id)
					}
				}
			})
		}
	})
	t.Run("direct IDs writes and grant changes cannot bypass scope", func(t *testing.T) {
		updateManager([]*genidentity.PermissionGrant{{Permission: "accounts:delete", Effect: "deny", Scope: "ALL", DepartmentIds: []int64{}}}, "active", "")
		checkStatus(t, managerClient.request(t, "DELETE", fmt.Sprintf("/api/admin/accounts/%d", engineer.ID), nil, true), 403)
		payload := &genidentity.UpdateAccountPayload{ID: outside.ID, Username: outside.Username, FullName: "Forbidden", Status: "active", OrganizationID: outside.OrganizationID, DepartmentID: outside.DepartmentID, RoleIds: []int64{}, Overrides: []*genidentity.PermissionGrant{}}
		checkStatus(t, managerClient.request(t, "PUT", fmt.Sprintf("/api/admin/accounts/%d", outside.ID), payload, true), 404)
		create := &genidentity.CreateAccountPayload{Username: "escalate", FullName: "Escalate", Status: "active", OrganizationID: company, DepartmentID: engineering.ID, RoleIds: []int64{initial.Roles[0].ID}, Overrides: []*genidentity.PermissionGrant{}, Password: "TestOnly!2026Secure"}
		checkStatus(t, managerClient.request(t, "POST", "/api/admin/accounts", create, true), 403)
	})
	t.Run("hierarchy invariants protect the root and reject cycles", func(t *testing.T) {
		checkStatus(t, root.request(t, "DELETE", fmt.Sprintf("/api/admin/accounts/%d", rootAccount.ID), nil, true), 409)
		checkStatus(t, root.request(t, "DELETE", fmt.Sprintf("/api/admin/organizations/%d", company), nil, true), 409)
		payload := &genidentity.UpdateDepartmentPayload{ID: engineering.ID, Name: engineering.Name, Code: engineering.Code, OrganizationID: company, ParentID: child.ID, Status: "active"}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/departments/%d", engineering.ID), payload, true), 400)
	})
	t.Run("role revocation and account disabling take effect immediately", func(t *testing.T) {
		update := &genidentity.UpdateRolePayload{ID: role.ID, Name: role.Name, Code: role.Code, Status: "inactive", Grants: grants}
		checkStatus(t, root.request(t, "PUT", fmt.Sprintf("/api/admin/roles/%d", role.ID), update, true), 200)
		response := managerClient.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, response, 200)
		if len(decodeBody[genidentity.AdminSnapshot](t, response).Accounts) != 0 {
			t.Fatal("inactive role kept authorizing")
		}
		updateManager([]*genidentity.PermissionGrant{}, "inactive", "")
		checkStatus(t, managerClient.request(t, "GET", "/api/admin/snapshot", nil, false), 401)
	})
	t.Run("tenant identifiers cannot be forged", func(t *testing.T) {
		if _, err := identity.Bootstrap(context.Background(), pool, "tenant-b", "other-admin", "TestOnly!2026Secure"); err != nil {
			t.Fatal(err)
		}
		other := loginIAM(t, handler, "other-admin")
		response := other.request(t, "GET", "/api/admin/snapshot", nil, false)
		checkStatus(t, response, 200)
		snapshot := decodeBody[genidentity.AdminSnapshot](t, response)
		if len(snapshot.Accounts) != 1 || snapshot.Accounts[0].ID == rootAccount.ID {
			t.Fatal("tenant data leaked")
		}
		checkStatus(t, other.request(t, "DELETE", fmt.Sprintf("/api/admin/departments/%d", child.ID), nil, true), 404)
		payload := &genidentity.UpdateRolePayload{ID: role.ID, Name: "Forbidden", Code: role.Code, Status: "active", Grants: grants}
		checkStatus(t, other.request(t, "PUT", fmt.Sprintf("/api/admin/roles/%d", role.ID), payload, true), 404)
	})
	t.Run("password reset and logout revoke persisted sessions", func(t *testing.T) {
		updateManager([]*genidentity.PermissionGrant{}, "active", "")
		current := loginIAM(t, handler, "manager")
		updateManager([]*genidentity.PermissionGrant{}, "active", "A-NewPassword!2026")
		checkStatus(t, current.request(t, "GET", "/api/admin/snapshot", nil, false), 401)
		checkStatus(t, root.request(t, "POST", "/api/auth/logout", nil, true), 200)
		checkStatus(t, root.request(t, "GET", "/api/auth/me", nil, false), 401)
	})
}

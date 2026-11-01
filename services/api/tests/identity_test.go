//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/governance"
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

// invoke runs one capability through the gateway. Every module test uses it, so
// the console and the tests exercise the same request shape.
func (c iamClient) invoke(t *testing.T, name string, input any, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{
		"input":          input,
		"idempotencyKey": fmt.Sprintf("test-%d-%s", time.Now().UnixNano(), name),
	}
	return c.request(t, "POST", "/api/capabilities/"+name+"/invocations", body, csrf)
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
	// Tables stay isolated in the fixture schema. public is kept on the path so
	// shared extension objects (for example the pg_trgm operator class used by
	// knowledge retrieval) remain resolvable.
	query.Set("search_path", schema+",public")
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
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{
		EncryptionKey:   []byte("01234567890123456789012345678901"),
		StorageRoot:     t.TempDir(),
		PublicWorkspace: "tenant-a",
	})
	handler := transport.HandlerWithCore(app.System{}, core, iam)
	return handler, pool, root
}
func TestPersistentIdentityAuthorization(t *testing.T) {
	handler, _, rootAccount := iamFixture(t)
	anonymous := iamClient{handler: handler}
	checkStatus(t, anonymous.request(t, "GET", "/api/admin/snapshot", nil, false), 401)
	checkStatus(t, anonymous.request(t, "POST", "/api/auth/login", map[string]string{"username": "root-admin", "password": "incorrect"}, false), 401)
	root := loginIAM(t, handler, "root-admin")
	initial := decodeBody[genidentity.AdminSnapshot](t, root.request(t, "GET", "/api/admin/snapshot", nil, false))
	if len(initial.Accounts) != 1 || len(initial.Organizations) != 2 || len(initial.Roles) != 2 {
		t.Fatal("bootstrap did not persist expected records")
	}

	t.Run("origin and CSRF enforce write boundaries", func(t *testing.T) {
		input := map[string]any{"name": "Extra", "code": "extra", "kind": "enterprise", "parentId": 0, "status": "active", "description": ""}
		checkStatus(t, root.invoke(t, "identity.organization.create", input, false), 403)
		r := httptest.NewRequest("POST", "http://console.test/api/capabilities/identity.organization.create/invocations", strings.NewReader(`{"input":{}}`))
		r.Header.Set("Origin", "https://attacker.test")
		r.Header.Set("X-CSRF-Token", root.csrf)
		r.AddCookie(root.cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		checkStatus(t, w, 403)
	})

	var engineering genidentity.DepartmentRecord
	var enterprise genidentity.OrganizationRecord
	for _, dept := range initial.Departments {
		if dept.ID != rootAccount.DepartmentID {
			engineering = *dept
		}
	}
	for _, org := range initial.Organizations {
		if org.Kind == "enterprise" {
			enterprise = *org
		}
	}

	response := root.invoke(t, "identity.department.create", map[string]any{
		"name": "Quality", "code": "quality", "organizationId": rootAccount.OrganizationID,
		"parentId": engineering.ID, "manager": "", "status": "active", "description": "",
	}, true)
	checkSucceeded(t, response)
	_ = decodeData[genidentity.DepartmentRecord](t, response)

	response = root.invoke(t, "identity.organization.create", map[string]any{
		"name": "Another company", "code": "another-company", "kind": "company",
		"parentId": enterprise.ID, "status": "active", "description": "",
	}, true)
	checkSucceeded(t, response)
	another := decodeData[genidentity.OrganizationRecord](t, response)

	response = root.invoke(t, "identity.department.create", map[string]any{
		"name": "Outside", "code": "outside", "organizationId": another.ID,
		"parentId": 0, "manager": "", "status": "active", "description": "",
	}, true)
	checkSucceeded(t, response)
	outside := decodeData[genidentity.DepartmentRecord](t, response)

	grants := []map[string]any{{"permission": "accounts:read", "effect": "allow", "scope": "ALL"}}
	response = root.invoke(t, "identity.role.create", map[string]any{
		"name": "Scoped manager", "code": "scoped-manager", "description": "", "status": "active", "grants": grants,
	}, true)
	checkSucceeded(t, response)
	role := decodeData[genidentity.RoleRecord](t, response)

	response = root.invoke(t, "identity.account.create", map[string]any{
		"username": "scoped-manager", "fullName": "Scoped manager", "email": "", "status": "active",
		"organizationId": another.ID, "departmentId": outside.ID, "roleIds": []int64{role.ID},
		"overrides": []map[string]any{}, "password": "TestOnly!2026Secure",
	}, true)
	checkSucceeded(t, response)
	manager := loginIAM(t, handler, "scoped-manager")
	checkStatus(t, manager.invoke(t, "identity.account.create", map[string]any{
		"username": "sneaky", "fullName": "Sneaky", "status": "active",
		"organizationId": another.ID, "departmentId": outside.ID, "roleIds": []int64{},
		"overrides": []map[string]any{}, "password": "TestOnly!2026Secure",
	}, true), 403)
	checkStatus(t, manager.request(t, "GET", "/api/admin/snapshot", nil, false), 200)
}

// decodeData unwraps a capability result and decodes its data projection.
func decodeData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "succeeded" {
		t.Fatalf("capability failed: %s", w.Body.String())
	}
	var out T
	if err := json.Unmarshal(envelope.Data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// checkSucceeded asserts the business outcome, not just the transport status.
func checkSucceeded(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("HTTP got %d: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Status    string `json:"status"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "succeeded" {
		t.Fatalf("capability status=%s code=%s body=%s", envelope.Status, envelope.ErrorCode, w.Body.String())
	}
}

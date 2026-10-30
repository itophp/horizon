package transport

import (
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type denyAuthorizer struct{}

func (denyAuthorizer) Allow(auth.Principal, string, string) (bool, error) {
	return false, errors.New("policy engine unavailable")
}
func TestSessionGuardCoversEveryAuthenticatedSurface(t *testing.T) {
	for path, guarded := range map[string]bool{
		"/api/admin/settings":     true,
		"/api/auth/configuration": true,
		"/api/assistant/status":   true,
		"/api/assistant/chat":     true,
		"/api/health":             false,
		"/api/demo/records":       false,
		"/api/assistant":          false,
	} {
		if got := guardedPath(path); got != guarded {
			t.Fatalf("%s guarded = %v, want %v", path, got, guarded)
		}
	}
}

func TestHTTP(t *testing.T) {
	e, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{Token: "test-token"}, e)
	for _, tc := range []struct {
		path, token string
		status      int
	}{{"/api/health", "", 200}, {"/api/echo?name=Ada", "", 200}, {"/api/echo?name=", "", 400}, {"/api/echo?name=" + strings.Repeat("x", 81), "", 400}, {"/api/demo/records", "", 401}, {"/api/demo/records", "bad", 401}, {"/api/demo/records", "test-token", 503}} {
		t.Run(tc.path+tc.token, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			r.Header.Set("X-Tenant-ID", "attacker")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body.String(), tc.status)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request id")
			}
			if strings.Contains(w.Body.String(), "internal_note") {
				t.Fatal("sensitive field leaked")
			}
		})
	}
	h = Handler(app.System{}, app.Demo{}, auth.DevelopmentToken{Token: "test-token"}, denyAuthorizer{})
	r := httptest.NewRequest("GET", "/api/demo/records", nil)
	r.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("policy failure allowed access")
	}
	// Context is not inferred from arbitrary values.
	if _, ok := auth.FromContext(context.Background()); ok {
		t.Fatal("untrusted principal")
	}
}

package transport

import (
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/governance"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionGuardCoversEveryAuthenticatedSurface(t *testing.T) {
	for path, guarded := range map[string]bool{
		"/api/capabilities":               true,
		"/api/capabilities/x/invocations": true,
		"/api/capabilities/approvals":     true,
		"/api/assistant/status":           true,
		"/api/assistant/chat":             true,
		"/api/ai/status":                  true,
		"/api/admin/snapshot":             true,
		"/api/auth/me":                    true,
		"/api/auth/temporary-access":      true,
		"/api/health":                     false,
		"/api/public/branding":            false,
		"/api/assistant":                  false,
	} {
		if got := guardedPath(path); got != guarded {
			t.Fatalf("%s guarded = %v, want %v", path, got, guarded)
		}
	}
}

func TestHTTPSystemBoundaries(t *testing.T) {
	h := Handler(app.System{}, governance.Options{})
	for _, tc := range []struct {
		name, path string
		status     int
	}{
		{"health", "/api/health", 200},
		{"echo", "/api/echo?name=Ada", 200},
		{"empty name", "/api/echo?name=", 400},
		{"long name", "/api/echo?name=" + strings.Repeat("x", 81), 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
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
}

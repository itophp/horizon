package transport

import (
	"baseline/services/api/gen/demo"
	demohttp "baseline/services/api/gen/http/demo/server"
	systemhttp "baseline/services/api/gen/http/system/server"
	"baseline/services/api/gen/system"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	goahttp "goa.design/goa/v3/http"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func Handler(systemService system.Service, demoService demo.Service, authenticate auth.Authenticator, authorize authorization.Authorizer, identities ...*identity.Service) http.Handler {
	return HandlerWithGovernance(systemService, demoService, authenticate, authorize, governance.Options{}, identities...)
}
func HandlerWithGovernance(systemService system.Service, demoService demo.Service, authenticate auth.Authenticator, authorize authorization.Authorizer, options governance.Options, identities ...*identity.Service) http.Handler {
	return handlerWithCore(systemService, demoService, authenticate, authorize, options, nil, identities...)
}
func HandlerWithCore(systemService system.Service, demoService demo.Service, authenticate auth.Authenticator, authorize authorization.Authorizer, core *governance.Core, identities ...*identity.Service) http.Handler {
	return handlerWithCore(systemService, demoService, authenticate, authorize, core.Options, core, identities...)
}
func handlerWithCore(systemService system.Service, demoService demo.Service, authenticate auth.Authenticator, authorize authorization.Authorizer, options governance.Options, core *governance.Core, identities ...*identity.Service) http.Handler {
	mux := goahttp.NewMuxer()
	errors := func(_ context.Context, _ http.ResponseWriter, err error) {
		slog.Error("transport failure", "error", err)
	}
	systemhttp.Mount(mux, systemhttp.New(system.NewEndpoints(systemService), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, errors, nil))
	demohttp.Mount(mux, demohttp.New(demo.NewEndpoints(demoService), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, errors, nil))
	var identityGuard func(http.ResponseWriter, *http.Request) (*http.Request, bool)
	if len(identities) > 0 {
		identityGuard = mountIdentity(mux, identities[0], errors)
		if core == nil {
			core = governance.New(identities[0].Database(), identities[0], options)
		}
		mountGovernance(mux, core, errors)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			http.Error(w, "internal", 500)
			return
		}
		id := hex.EncodeToString(raw[:])
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		// Paths and credentials are deliberately excluded from logs. No permissive CORS; Vite proxies same-origin /api.
		defer func() {
			slog.Info("request completed", "request_id", id, "method", r.Method, "duration_ms", time.Since(started).Milliseconds())
		}()
		if identityGuard != nil {
			next, ok := identityGuard(w, r)
			if !ok {
				return
			}
			r = next
		}
		if strings.HasPrefix(r.URL.Path, "/api/demo") {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || authenticate == nil {
				deny(w, 401, "unauthenticated")
				return
			}
			p, err := authenticate.Authenticate(r.Context(), token)
			if err != nil || p.UserID == "" || p.TenantID == "" {
				deny(w, 401, "unauthenticated")
				return
			}
			actions := map[string]string{"GET": "read", "PATCH": "update", "DELETE": "delete"}
			action, ok := actions[r.Method]
			if !ok || authorize == nil {
				deny(w, 403, "forbidden")
				return
			}
			allowed, err := authorize.Allow(p, "demo-record", action)
			if err != nil || !allowed {
				deny(w, 403, "forbidden")
				return
			}
			slog.Info("authorization allowed", "request_id", id, "resource", "demo-record", "action", action)
			r = r.WithContext(auth.WithPrincipal(r.Context(), p))
		}
		limit := int64(1 << 20)
		if r.Method == "POST" && r.URL.Path == "/api/admin/files" {
			limit = 29 << 20
		}
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/admin/dictionaries") {
			limit = 21 << 20
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		mux.ServeHTTP(w, r)
	})
}
func deny(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"code": code}); err != nil {
		slog.Error("error response write failed", "error", err)
	}
}

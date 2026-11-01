package transport

import (
	systemhttp "baseline/services/api/gen/http/system/server"
	"baseline/services/api/gen/system"
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

// Handler builds the HTTP surface used by tests: system health, the identity
// session, the public branding projection and the capability platform.
func Handler(systemService system.Service, options governance.Options, identities ...*identity.Service) http.Handler {
	return HandlerWithGovernance(systemService, options, identities...)
}

// HandlerWithGovernance keeps the historical name used by callers that pass
// module options instead of a prebuilt core.
func HandlerWithGovernance(systemService system.Service, options governance.Options, identities ...*identity.Service) http.Handler {
	handler, _ := handlerWithCore(systemService, options, nil, identities...)
	return handler
}

// HandlerWithCore accepts an already configured core, which is what long lived
// processes and multi-tenant tests use.
func HandlerWithCore(systemService system.Service, core *governance.Core, identities ...*identity.Service) http.Handler {
	handler, _ := handlerWithCore(systemService, core.Options, core, identities...)
	return handler
}

// HandlerWithRuntime returns the HTTP handler and the background starter (outbox
// relay and AI trigger engine). Tests keep using Handler, which never spawns
// workers.
func HandlerWithRuntime(systemService system.Service, core *governance.Core, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	return handlerWithCore(systemService, core.Options, core, identities...)
}

func handlerWithCore(systemService system.Service, options governance.Options, core *governance.Core, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	mux := goahttp.NewMuxer()
	errors := func(_ context.Context, _ http.ResponseWriter, err error) {
		slog.Error("transport failure", "error", err)
	}
	systemhttp.Mount(mux, systemhttp.New(system.NewEndpoints(systemService), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, errors, nil))
	var identityGuard func(http.ResponseWriter, *http.Request) (*http.Request, bool)
	start := func(context.Context) {}
	if len(identities) > 0 {
		identityGuard = mountIdentity(mux, identities[0], errors)
		if core == nil || core.Pool == nil {
			core = governance.New(identities[0].Database(), identities[0], options)
		}
		start = mountGovernance(mux, core, errors)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		// Paths and credentials are deliberately excluded from logs. No permissive CORS; the console proxies same-origin /api.
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
		// A capability invocation may carry a base64 attachment; every other
		// endpoint keeps the small default body.
		limit := int64(1 << 20)
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/capabilities/") && strings.HasSuffix(r.URL.Path, "/invocations") {
			// A capability input may carry a base64 attachment (file.upload).
			limit = 29 << 20
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		mux.ServeHTTP(w, r)
	})
	return handler, start
}

func deny(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"code": code}); err != nil {
		slog.Error("error response write failed", "error", err)
	}
}

package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	goahttp "goa.design/goa/v3/http"
	systemhttp "horizon/foundation/gen/http/system/server"
	"horizon/foundation/gen/system"
	"horizon/foundation/kernel/plugin"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	memorysystem "horizon/foundation/system/memory"
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
	handler, _ := handlerWithCore(systemService, options, nil, nil, identities...)
	return handler
}

// HandlerWithCore accepts an already configured core, which is what long lived
// processes and multi-tenant tests use.
func HandlerWithCore(systemService system.Service, core *governance.Core, identities ...*identity.Service) http.Handler {
	handler, _ := handlerWithCore(systemService, core.Options, core, nil, identities...)
	return handler
}

// HandlerWithRuntime returns the HTTP handler and the background starter (outbox
// relay and AI trigger engine). Tests keep using Handler, which never spawns
// workers.
func HandlerWithRuntime(systemService system.Service, core *governance.Core, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	return handlerWithCore(systemService, core.Options, core, nil, identities...)
}

// HandlerWithPlugins uses the already registered runtime plugin instances.
func HandlerWithPlugins(systemService system.Service, core *governance.Core, plugins *plugin.Manager, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	return handlerWithCore(systemService, core.Options, core, plugins, identities...)
}

func handlerWithCore(systemService system.Service, options governance.Options, core *governance.Core, plugins *plugin.Manager, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	mux := goahttp.NewMuxer()
	errors := func(_ context.Context, _ http.ResponseWriter, err error) {
		slog.Error("transport failure", "error", err)
	}
	systemhttp.Mount(mux, systemhttp.New(system.NewEndpoints(systemService), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, errors, nil))
	var identityGuard func(http.ResponseWriter, *http.Request) (*http.Request, bool)
	start := func(context.Context) {}
	var observer *memorysystem.Observer
	if len(identities) > 0 {
		identityGuard = mountIdentity(mux, identities[0], errors)
		if core == nil || core.Pool == nil {
			core = governance.New(identities[0].Database(), identities[0], options)
		}
		observer = memorysystem.NewObserver(core.Pool)
		start = mountGovernance(mux, core, plugins, observer, errors)
		startGovernance := start
		start = func(ctx context.Context) { startGovernance(ctx); go observer.Run(ctx) }
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			http.Error(w, "internal", 500)
			return
		}
		id := hex.EncodeToString(raw[:])
		r = r.WithContext(context.WithValue(r.Context(), requestCorrelationKey{}, "req_"+id))
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/api/ai/runs/stream" {
			w.Header().Set("X-Accel-Buffering", "no")
		}
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
		if p, err := identity.Current(r.Context()); err == nil && observer != nil {
			recorded := &statusWriter{ResponseWriter: w}
			w = recorded
			defer func() {
				failure := recover()
				observeHTTP(observer, p, r, id, started, recorded, failure != nil)
				if failure != nil {
					panic(failure)
				}
			}()
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

package transport

import (
	identityhttp "baseline/services/api/gen/http/identity/server"
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/identity"
	"context"
	"errors"
	goahttp "goa.design/goa/v3/http"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type loginBucket struct {
	window time.Time
	count  int
}
type loginLimiter struct {
	mu      sync.Mutex
	buckets map[string]loginBucket
}

func (l *loginLimiter) allow(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[host]
	if now.Sub(bucket.window) >= time.Minute {
		bucket = loginBucket{window: now}
	}
	bucket.count++
	if len(l.buckets) >= 4096 {
		for key, value := range l.buckets {
			if now.Sub(value.window) > time.Minute {
				delete(l.buckets, key)
			}
		}
		if len(l.buckets) >= 4096 {
			return false
		}
	}
	l.buckets[host] = bucket
	return bucket.count <= 20
}
func sameOrigin(r *http.Request, secure bool) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	scheme := "http"
	if secure || r.TLS != nil {
		scheme = "https"
	}
	return err == nil && origin.Scheme == scheme && strings.EqualFold(origin.Host, r.Host) && origin.User == nil && (origin.Path == "" || origin.Path == "/") && origin.RawQuery == "" && origin.Fragment == ""
}
func identityStatus(err error) int {
	var problem *genidentity.IAMProblem
	if errors.As(err, &problem) {
		switch problem.Code {
		case "unauthenticated":
			return 401
		case "forbidden":
			return 403
		case "unavailable":
			return 503
		}
	}
	return 500
}
func mountIdentity(mux goahttp.Muxer, service *identity.Service, onError func(context.Context, http.ResponseWriter, error)) func(http.ResponseWriter, *http.Request) (*http.Request, bool) {
	identityhttp.Mount(mux, identityhttp.New(genidentity.NewEndpoints(service), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	limiter := &loginLimiter{buckets: map[string]loginBucket{}}
	return func(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
		if !strings.HasPrefix(r.URL.Path, "/api/admin/") && !strings.HasPrefix(r.URL.Path, "/api/auth/") {
			return r, true
		}
		unsafe := r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
		if unsafe && !sameOrigin(r, service.SecureCookies) {
			deny(w, 403, "forbidden")
			return r, false
		}
		if r.URL.Path == "/api/auth/login" {
			if !limiter.allow(r.RemoteAddr) {
				deny(w, 429, "rate_limited")
				return r, false
			}
			return r, true
		}
		cookie, err := r.Cookie(identity.CookieName)
		if err != nil {
			deny(w, 401, "unauthenticated")
			return r, false
		}
		p, err := service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			status := identityStatus(err)
			code := "internal"
			if status == 401 {
				code = "unauthenticated"
			} else if status == 503 {
				code = "unavailable"
			}
			deny(w, status, code)
			return r, false
		}
		if unsafe && !identity.CSRFMatches(p, r.Header.Get("X-CSRF-Token")) {
			deny(w, 403, "forbidden")
			return r, false
		}
		return r.WithContext(identity.WithPrincipal(r.Context(), p)), true
	}
}

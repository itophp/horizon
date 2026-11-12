package transport

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	canonical "horizon/foundation/kernel/memory"
	"horizon/foundation/system/identity"
	memorysystem "horizon/foundation/system/memory"
)

type requestCorrelationKey struct{}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) WriteHeader(status int) {
	if status >= 200 && w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}
func (w *statusWriter) FlushError() error {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *statusWriter) Flush() {
	if err := w.FlushError(); err != nil {
		slog.Warn("response flush failed", "code", "transport_error")
	}
}
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func observationSource(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/capabilities"):
		return "capabilities"
	case strings.HasPrefix(path, "/api/ai/"):
		return "ai"
	case strings.HasPrefix(path, "/api/assistant/"):
		return "assistant"
	case strings.HasPrefix(path, "/api/admin/"):
		return "admin"
	case strings.HasPrefix(path, "/api/auth/"):
		return "auth"
	default:
		return "other"
	}
}

func observeHTTP(observer *memorysystem.Observer, p *identity.Principal, r *http.Request, requestID string, started time.Time, w *statusWriter, panicked bool) {
	status := w.status
	if status == 0 {
		status = 200
	}
	if panicked {
		status = 500
	}
	statusClass := strconv.Itoa(status/100) + "xx"
	if status < 100 || status > 599 {
		statusClass = "5xx"
	}
	method := r.Method
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		method = "OTHER"
	}
	duration := "gte1s"
	if time.Since(started) < 100*time.Millisecond {
		duration = "lt100ms"
	} else if time.Since(started) < time.Second {
		duration = "lt1s"
	}
	base := canonical.Envelope{EventID: "http:" + requestID, EventType: "activity.http.completed", TenantID: p.TenantID,
		OccurredAt: started, Actor: canonical.Actor{ID: strconv.FormatInt(p.Account.ID, 10), Type: "user"}, DepartmentID: p.Account.DepartmentID,
		Source: canonical.Source{Type: "http", ID: observationSource(r.URL.Path)}, ActionName: method, CorrelationID: "req_" + requestID, Sensitivity: "internal",
		Payload: map[string]string{"method": method, "status": statusClass, "duration": duration}}
	observer.Submit(base)
	if status >= 500 {
		base.EventID = "exception:" + requestID
		base.EventType = "system.exception.detected"
		kind := "http_5xx"
		if panicked {
			kind = "panic"
		}
		base.Payload = map[string]string{"kind": kind}
		observer.Submit(base)
	}
}

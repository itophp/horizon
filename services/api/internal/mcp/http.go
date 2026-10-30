package mcp

import (
	gd "baseline/services/api/gen/dictionaries"
	gn "baseline/services/api/gen/notifications"
	"baseline/services/api/internal/dictionaries"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type toolInput struct {
	Code     string `json:"code,omitempty"`
	Language string `json:"language,omitempty"`
	Search   string `json:"search,omitempty"`
	ID       int64  `json:"id,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func (s *Service) Handler(core *governance.Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			host = r.RemoteAddr
		}
		if !s.limits.allow("ip:"+host, 600, 100) {
			http.Error(w, "rate_limited", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if s.identity != nil && s.identity.SecureCookies {
			expected, e := url.Parse(core.Options.PublicURL)
			if e != nil || expected.Scheme != "https" || !strings.EqualFold(expected.Host, r.Host) {
				http.Error(w, "forbidden", 403)
				return
			}
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || (s.identity != nil && s.identity.SecureCookies && u.Scheme != "https") || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
				http.Error(w, "forbidden", 403)
				return
			}
		}
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(raw) != 69 || !strings.HasPrefix(raw, "bmcp_") || s.pool == nil {
			http.Error(w, "unauthenticated", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		q := dbgen.New(s.pool)
		row, err := q.MCPAuthenticateToken(ctx, hash(raw))
		if err != nil || subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(hash(raw))) != 1 {
			http.Error(w, "unauthenticated", 401)
			return
		}
		if !s.limits.allow("token:"+strconv.FormatInt(row.ID, 10), 120, 30) || !s.limits.allow("account:"+row.TenantID+":"+strconv.FormatInt(row.AccountID, 10), 240, 60) {
			http.Error(w, "rate_limited", 429)
			return
		}
		p, err := s.identity.PrincipalForTask(ctx, row.TenantID, row.AccountID, row.DepartmentID)
		if err != nil {
			http.Error(w, "unauthenticated", 401)
			return
		}
		allowed, err := q.MCPTokenTools(ctx, dbgen.MCPTokenToolsParams{TenantID: row.TenantID, AccountID: row.AccountID, ID: row.ID})
		if err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		allowed = slices.DeleteFunc(allowed, func(name string) bool {
			return !slices.Contains(Tools, name) || (name == "dictionary_options" && !p.Can("dictionaries:use"))
		})
		if err = q.MCPTouchToken(ctx, row.ID); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		server := sdk.NewServer(&sdk.Implementation{Name: "baseline-business", Version: "1.0.0"}, &sdk.ServerOptions{Capabilities: &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{ListChanged: false}}})
		for _, name := range allowed {
			registerReadTool(server, name, func(callctx context.Context, in toolInput) (*sdk.CallToolResult, interface{}, error) {
				started := time.Now()
				result, e := s.call(identity.WithPrincipal(callctx, p), core, name, in, p, allowed)
				var data []byte
				if e == nil {
					data, e = json.Marshal(result)
					if e == nil && len(data) > 512*1024 {
						e = governance.Problem("result_too_large")
					}
				}
				code := "ok"
				if e != nil {
					code = "operation_failed"
					var coded interface{ GoaErrorName() string }
					if errors.As(e, &coded) {
						code = coded.GoaErrorName()
					} else {
						var known *governance.Error
						if errors.As(e, &known) {
							code = known.Code
						}
					}
				}

				auditCtx, auditCancel := context.WithTimeout(context.WithoutCancel(callctx), 2*time.Second)
				auditErr := q.MCPInsertAudit(auditCtx, dbgen.MCPInsertAuditParams{TenantID: row.TenantID, AccountID: row.AccountID, TokenID: row.ID, DepartmentID: row.DepartmentID, Tool: name, ResultCode: code, DurationMs: time.Since(started).Milliseconds(), RequestID: w.Header().Get("X-Request-ID")})
				auditCancel()
				if auditErr != nil {
					return nil, nil, errors.New("audit_unavailable")
				}
				if e != nil {
					return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: code}}}, nil, nil
				}

				return &sdk.CallToolResult{StructuredContent: map[string]interface{}{"data": result}, Content: []sdk.Content{&sdk.TextContent{Text: string(data)}}}, nil, nil
			})
		}
		handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true})
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		handler.ServeHTTP(w, r)
	})
}

var descriptions = map[string]string{
	"context_get": "Read the current account ID and bound department ID.", "capabilities_list": "List tools allowed for this credential and current permissions.", "dictionary_options": "Read authorized dictionary options by code and language.", "notifications_list": "Read your notifications, with offset and limit (maximum 50).", "notifications_get": "Read one of your notifications by ID.",
}

func (s *Service) call(ctx context.Context, core *governance.Core, name string, in toolInput, p *identity.Principal, allowed []string) (interface{}, error) {
	if in.Offset < 0 || in.Offset > 10000 || in.Limit < 0 || in.Limit > 50 || len([]rune(in.Search)) > 100 || len([]rune(in.Code)) > 80 {
		return nil, governance.Problem("invalid")
	}
	switch name {
	case "context_get":
		return map[string]int64{"accountId": p.Account.ID, "departmentId": p.Account.DepartmentID}, nil
	case "capabilities_list":
		return allowed, nil
	case "dictionary_options":
		if in.Code == "" || (in.Language != "en" && in.Language != "zh-CN") {
			return nil, governance.Problem("invalid")
		}
		items, err := dictionaries.New(core).Options(ctx, &gd.OptionsPayload{Code: in.Code, Language: in.Language, Search: in.Search})
		if err != nil {
			return nil, err
		}
		limit := in.Limit
		if limit == 0 {
			limit = 20
		}
		start := min(in.Offset, len(items))
		return items[start:min(start+limit, len(items))], nil
	case "notifications_get":
		if in.ID < 1 {
			return nil, governance.Problem("invalid")
		}
		return notifications.New(core).Message(ctx, &gn.MessagePayload{ID: in.ID})
	case "notifications_list":
		limit := in.Limit
		if limit == 0 {
			limit = 20
		}
		return notifications.New(core).InboxPage(ctx, in.Offset, limit)
	}
	return nil, governance.Problem("forbidden")
}

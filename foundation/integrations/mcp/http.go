package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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
		if s.gateway == nil {
			http.Error(w, "unavailable", 503)
			return
		}
		principalCtx := identity.WithPrincipal(ctx, p)
		definitions, err := s.gateway.AutomatedDefinitions(principalCtx)
		if err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		if err = q.MCPTouchToken(ctx, row.ID); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		server := sdk.NewServer(&sdk.Implementation{Name: "baseline-business", Version: "1.0.0"}, &sdk.ServerOptions{Capabilities: &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{ListChanged: false}}})
		for _, definition := range definitions {
			d := definition
			description := d.Description
			if d.Idempotency.Required {
				description += " For safe retries, set a stable _meta.idempotencyKey for the same operation."
			}
			server.AddTool(&sdk.Tool{Name: d.ToolName(), Description: description, InputSchema: d.Input,
				Annotations: &sdk.ToolAnnotations{ReadOnlyHint: d.ReadOnly}}, func(callctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				started := time.Now()
				input := contracts.Input{}
				var err error
				if len(req.Params.Arguments) > 0 {
					err = json.Unmarshal(req.Params.Arguments, &input)
				}
				if err == nil && input == nil {
					err = contracts.Problem("invalid")
				}
				key := ""
				if err == nil && d.Idempotency.Required {
					if value, exists := req.Params.Meta["idempotencyKey"]; exists {
						supplied, ok := value.(string)
						if !ok || strings.TrimSpace(supplied) == "" || len(supplied) > 128 {
							err = contracts.Problem("invalid")
						} else {
							digest := sha256.Sum256([]byte(supplied))
							key = fmt.Sprintf("mcp:%d:%d:%x", p.Account.ID, p.Account.DepartmentID, digest)
						}
					} else {
						var raw [16]byte
						_, err = rand.Read(raw[:])
						key = "mcp:" + hex.EncodeToString(raw[:])
					}
				}
				var result *contracts.Result
				if err == nil {
					result, err = s.gateway.Invoke(identity.WithPrincipal(callctx, p), gateway.Request{Name: d.Name, Version: d.Version, Input: input, IdempotencyKey: key, Origin: contracts.OriginAgent})
				}
				code := "ok"
				if err != nil {
					code = contracts.Code(err)
					if code == "" {
						code = "operation_failed"
					}
				}
				if err == nil && result.Status != contracts.StatusSucceeded {
					code = string(result.Status)
					if result.ErrorCode != "" {
						code = result.ErrorCode
					}
				}
				auditCtx, cancel := context.WithTimeout(context.WithoutCancel(callctx), 2*time.Second)
				auditErr := q.MCPInsertAudit(auditCtx, dbgen.MCPInsertAuditParams{TenantID: row.TenantID, AccountID: row.AccountID, TokenID: row.ID, DepartmentID: row.DepartmentID, Tool: d.ToolName(), ResultCode: code, DurationMs: time.Since(started).Milliseconds(), RequestID: w.Header().Get("X-Request-ID")})
				cancel()
				if auditErr != nil {
					return nil, errors.New("audit_unavailable")
				}
				if err != nil {
					return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: code}}}, nil
				}
				data, err := json.Marshal(result)
				if err != nil || len(data) > 512*1024 {
					return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "result_too_large"}}}, nil
				}
				return &sdk.CallToolResult{StructuredContent: result, Content: []sdk.Content{&sdk.TextContent{Text: string(data)}}}, nil
			})
		}
		handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true})
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		handler.ServeHTTP(w, r)
	})
}

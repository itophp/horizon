package mcp

import (
	gen "baseline/services/api/gen/mcp_management"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"goa.design/goa/v3/security"
	"slices"
	"strings"
)

var Tools = []string{"context_get", "capabilities_list", "dictionary_options", "notifications_list", "notifications_get"}

type Service struct {
	pool     *pgxpool.Pool
	identity *identity.Service
	limits   requestLimits
}

func New(pool *pgxpool.Pool, iam *identity.Service) *Service {
	return &Service{pool: pool, identity: iam}
}
func problem(code string) error { return &gen.IAMProblem{Code: code} }
func mapped(err *error) {
	if *err != nil {
		var p *gen.IAMProblem
		if errors.As(*err, &p) {
			return
		}
		if errors.Is(*err, pgx.ErrNoRows) {
			*err = problem("not_found")
			return
		}
		*err = problem(governance.Code(*err))
	}
}
func (s *Service) APIKeyAuth(ctx context.Context, _ string, _ *security.APIKeyScheme) (context.Context, error) {
	_, e := identity.Current(ctx)
	return ctx, e
}
func hash(raw string) string { sum := sha256.Sum256([]byte(raw)); return hex.EncodeToString(sum[:]) }
func (s *Service) Create(ctx context.Context, in *gen.CreatePayload) (out *gen.MCPTokenCreated, err error) {
	return s.issue(ctx, in, 0, in.Password)
}
func (s *Service) issue(ctx context.Context, in *gen.CreatePayload, rotateID int64, password string) (out *gen.MCPTokenCreated, err error) {
	defer mapped(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, problem("unavailable")
	}
	if err = s.identity.VerifyCurrentPassword(ctx, p, password); err != nil {
		return nil, err
	}
	departmentID := int64(0)
	if rotateID != 0 {
		departmentID, err = dbgen.New(s.pool).MCPRotationDepartment(ctx, dbgen.MCPRotationDepartmentParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: rotateID})
		if err != nil {
			return nil, err
		}
	} else {
		departmentID = in.DepartmentID
	}
	boundPrincipal, err := s.identity.PrincipalForTask(ctx, p.TenantID, p.Account.ID, departmentID)
	if err != nil {
		return nil, problem("forbidden")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	locked, err := q.MCPLockOwner(ctx, dbgen.MCPLockOwnerParams{TenantID: p.TenantID, ID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	if !identity.VerifyPassword(password, locked.PasswordHash) {
		return nil, problem("forbidden")
	}
	count, err := q.MCPActiveCount(ctx, dbgen.MCPActiveCountParams{TenantID: p.TenantID, AccountID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	if rotateID == 0 && count >= 20 {
		return nil, problem("invalid")
	}
	var expiry pgtype.Timestamptz
	if rotateID != 0 {
		old, e := q.MCPTokenForRotation(ctx, dbgen.MCPTokenForRotationParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: rotateID})
		if e != nil {
			return nil, e
		}
		tools, e := q.MCPTokenTools(ctx, dbgen.MCPTokenToolsParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: rotateID})
		if e != nil {
			return nil, e
		}
		in = &gen.CreatePayload{Name: old.Name, DepartmentID: old.DepartmentID, Days: 30, Tools: tools}
		expiry = old.ExpiresAt
		if _, e = q.MCPRevokeToken(ctx, dbgen.MCPRevokeTokenParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: rotateID}); e != nil {
			return nil, e
		}
	}
	if strings.TrimSpace(in.Name) == "" || in.Days < 1 || in.Days > 90 || len(in.Tools) == 0 {
		return nil, problem("invalid")
	}

	seen := map[string]bool{}
	for _, tool := range in.Tools {
		if !slices.Contains(Tools, tool) || seen[tool] {
			return nil, problem("invalid")
		}
		if tool == "dictionary_options" && !boundPrincipal.Can("dictionaries:use") {
			return nil, problem("forbidden")
		}
		seen[tool] = true
	}
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return nil, err
	}
	secret := "bmcp_" + hex.EncodeToString(raw[:])

	row, err := q.MCPCreateToken(ctx, dbgen.MCPCreateTokenParams{TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: in.DepartmentID, Name: in.Name, TokenHash: hash(secret), TokenPrefix: secret[:13], PreservedExpiry: expiry, ValidDays: int32(in.Days)})
	if err != nil {
		return nil, err
	}
	for _, tool := range in.Tools {
		if err = q.MCPAddTokenTool(ctx, dbgen.MCPAddTokenToolParams{TokenID: row.ID, Tool: tool}); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &gen.MCPTokenCreated{Secret: secret, Token: &gen.MCPToken{ID: row.ID, DepartmentID: row.DepartmentID, Name: row.Name, Prefix: row.TokenPrefix, Tools: in.Tools, CreatedAt: governance.Time(row.CreatedAt), ExpiresAt: governance.Time(row.ExpiresAt)}}, nil
}
func (s *Service) List(ctx context.Context, _ *gen.ListPayload) (out []*gen.MCPToken, err error) {
	defer mapped(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, problem("unavailable")
	}
	q := dbgen.New(s.pool)
	rows, err := q.MCPListTokens(ctx, dbgen.MCPListTokensParams{TenantID: p.TenantID, AccountID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	out = []*gen.MCPToken{}
	for _, r := range rows {
		tools, e := q.MCPTokenTools(ctx, dbgen.MCPTokenToolsParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: r.ID})
		if e != nil {
			return nil, e
		}
		out = append(out, &gen.MCPToken{ID: r.ID, DepartmentID: r.DepartmentID, Name: r.Name, Prefix: r.TokenPrefix, Tools: tools, CreatedAt: governance.Time(r.CreatedAt), ExpiresAt: governance.Time(r.ExpiresAt), RevokedAt: governance.Time(r.RevokedAt), LastUsedAt: governance.Time(r.LastUsedAt), RevokedBy: r.RevokedBy})
	}
	return out, nil
}
func (s *Service) Revoke(ctx context.Context, in *gen.RevokePayload) (err error) {
	defer mapped(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	if s.pool == nil {
		return problem("unavailable")
	}
	if err = s.identity.VerifyCurrentPassword(ctx, p, in.Password); err != nil {
		return err
	}
	_, err = dbgen.New(s.pool).MCPRevokeToken(ctx, dbgen.MCPRevokeTokenParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: in.ID})
	return err
}
func (s *Service) Audit(ctx context.Context, _ *gen.AuditPayload) (out []*gen.MCPAudit, err error) {
	defer mapped(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if s.pool == nil {
		return nil, problem("unavailable")
	}
	rows, err := dbgen.New(s.pool).MCPListAudits(ctx, dbgen.MCPListAuditsParams{TenantID: p.TenantID, AccountID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	out = []*gen.MCPAudit{}
	for _, r := range rows {
		out = append(out, &gen.MCPAudit{ID: r.ID, TokenID: r.TokenID, DepartmentID: r.DepartmentID, Tool: r.Tool, ResultCode: r.ResultCode, DurationMs: r.DurationMs, RequestID: r.RequestID, CreatedAt: governance.Time(r.CreatedAt)})
	}
	return out, nil
}

func (s *Service) Rotate(ctx context.Context, in *gen.RotatePayload) (*gen.MCPTokenCreated, error) {
	return s.issue(ctx, nil, in.ID, in.Password)
}
func (s *Service) Capabilities(ctx context.Context, in *gen.CapabilitiesPayload) (out []string, err error) {
	defer mapped(&err)
	p, e := identity.Current(ctx)
	if e != nil {
		return nil, e
	}
	departmentID := p.Account.DepartmentID
	if in.DepartmentID != nil {
		departmentID = *in.DepartmentID
	}
	p, e = s.identity.PrincipalForTask(ctx, p.TenantID, p.Account.ID, departmentID)
	if e != nil {
		return nil, e
	}
	out = append([]string{}, Tools...)
	if !p.Can("dictionaries:use") {
		out = slices.DeleteFunc(out, func(v string) bool { return v == "dictionary_options" })
	}
	return out, nil
}

// AdminRevoke is deliberately separate from personal token management. It grants
// no read access to another account's credential and cannot mint one for them.
func (s *Service) AdminRevoke(ctx context.Context, in *gen.AdminRevokePayload) (err error) {
	defer mapped(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	if s.pool == nil {
		return problem("unavailable")
	}
	_, scope, err := s.identity.Authorize(ctx, "mcp:revoke", true)
	if err != nil {
		return err
	}
	if !scope.All {
		return problem("forbidden")
	}
	if err = s.identity.VerifyCurrentPassword(ctx, p, in.Password); err != nil {
		return err
	}
	_, err = dbgen.New(s.pool).MCPAdminRevokeToken(ctx, dbgen.MCPAdminRevokeTokenParams{TenantID: p.TenantID, ActorID: pgtype.Int8{Int64: p.Account.ID, Valid: true}, ID: in.ID})
	return err
}

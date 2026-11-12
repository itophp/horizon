package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"goa.design/goa/v3/security"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
	"strings"
)

type Service struct {
	pool     *pgxpool.Pool
	identity *identity.Service
	limits   requestLimits
	gateway  *gateway.Gateway
}

func New(pool *pgxpool.Pool, iam *identity.Service) *Service {
	return &Service{pool: pool, identity: iam}
}
func problem(code string) error { return governance.Problem(code) }
func mapped(err *error) {
	if *err != nil {
		var coded *governance.Error
		if errors.As(*err, &coded) {
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
func (s *Service) Create(ctx context.Context, in *CreatePayload) (out *MCPTokenCreated, err error) {
	return s.issue(ctx, in, 0, in.Password)
}
func (s *Service) issue(ctx context.Context, in *CreatePayload, rotateID int64, password string) (out *MCPTokenCreated, err error) {
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
	_, err = s.identity.PrincipalForTask(ctx, p.TenantID, p.Account.ID, departmentID)
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
		in = &CreatePayload{Name: old.Name, DepartmentID: old.DepartmentID, Days: 30}

		expiry = old.ExpiresAt
		if _, e = q.MCPRevokeToken(ctx, dbgen.MCPRevokeTokenParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: rotateID}); e != nil {
			return nil, e
		}
	}
	if strings.TrimSpace(in.Name) == "" || in.Days < 1 || in.Days > 90 {
		return nil, problem("invalid")
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
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &MCPTokenCreated{Secret: secret, Token: &MCPToken{ID: row.ID, DepartmentID: row.DepartmentID, Name: row.Name, Prefix: row.TokenPrefix, Tools: []string{}, CreatedAt: governance.Time(row.CreatedAt), ExpiresAt: governance.Time(row.ExpiresAt)}}, nil
}
func (s *Service) List(ctx context.Context, _ *ListPayload) (out []*MCPToken, err error) {
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
	out = []*MCPToken{}
	for _, r := range rows {

		out = append(out, &MCPToken{ID: r.ID, DepartmentID: r.DepartmentID, Name: r.Name, Prefix: r.TokenPrefix, Tools: []string{}, CreatedAt: governance.Time(r.CreatedAt), ExpiresAt: governance.Time(r.ExpiresAt), RevokedAt: governance.Time(r.RevokedAt), LastUsedAt: governance.Time(r.LastUsedAt), RevokedBy: r.RevokedBy})
	}
	return out, nil
}
func (s *Service) Revoke(ctx context.Context, in *RevokePayload) (err error) {
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
func (s *Service) Audit(ctx context.Context, _ *AuditPayload) (out []*MCPAudit, err error) {
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
	out = []*MCPAudit{}
	for _, r := range rows {
		out = append(out, &MCPAudit{ID: r.ID, TokenID: r.TokenID, DepartmentID: r.DepartmentID, Tool: r.Tool, ResultCode: r.ResultCode, DurationMs: r.DurationMs, RequestID: r.RequestID, CreatedAt: governance.Time(r.CreatedAt)})
	}
	return out, nil
}

func (s *Service) Rotate(ctx context.Context, in *RotatePayload) (*MCPTokenCreated, error) {
	return s.issue(ctx, nil, in.ID, in.Password)
}
func (s *Service) Capabilities(ctx context.Context, in *CapabilitiesPayload) (out []string, err error) {
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
	if s.gateway == nil {
		return nil, problem("unavailable")
	}
	definitions, e := s.gateway.AutomatedDefinitions(identity.WithPrincipal(ctx, p))
	if e != nil {
		return nil, e
	}
	out = []string{}
	for _, d := range definitions {
		out = append(out, d.ToolName())
	}
	return out, nil
}

// AdminRevoke is deliberately separate from personal token management. It grants
// no read access to another account's credential and cannot mint one for them.
func (s *Service) AdminRevoke(ctx context.Context, in *AdminRevokePayload) (err error) {
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

func (s *Service) AttachGateway(g *gateway.Gateway) { s.gateway = g }

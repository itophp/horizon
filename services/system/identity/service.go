package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"goa.design/goa/v3/security"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const CookieName = "baseline_session"

type Service struct {
	repo          *Repository
	SecureCookies bool
	Policies      PolicyProvider
}

func New(pool *pgxpool.Pool, secure bool) *Service {
	if pool == nil {
		return &Service{SecureCookies: secure}
	}
	return &Service{repo: newRepository(pool), SecureCookies: secure}
}
func problem(code string) error { return &genidentity.IAMProblem{Code: code} }
func safe(err error) error {
	if err == nil {
		return nil
	}
	var api *genidentity.IAMProblem
	if errors.As(err, &api) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return problem("not_found")
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		if pgerr.Code == "23505" || pgerr.Code == "23503" {
			return problem("conflict")
		}
		if pgerr.Code == "23514" {
			return problem("invalid")
		}
	}
	slog.Error("identity repository operation failed", "error_type", fmt.Sprintf("%T", err))
	return problem("internal")
}
func (s *Service) available() error {
	if s.repo == nil {
		return problem("unavailable")
	}
	return nil
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func (s *Service) cookie(value string, expired bool) string {
	return s.policyCookie(value, expired, 480)
}
func (s *Service) policyCookie(value string, expired bool, minutes int) string {
	max := minutes * 60
	if expired {
		max = -1
	}
	return (&http.Cookie{Name: CookieName, Value: value, Path: "/", HttpOnly: true, Secure: s.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: max}).String()
}
func (s *Service) APIKeyAuth(ctx context.Context, _ string, _ *security.APIKeyScheme) (context.Context, error) {
	_, err := Current(ctx)
	return ctx, err
}
func (s *Service) Authenticate(ctx context.Context, raw string) (*Principal, error) {
	if err := s.available(); err != nil {
		return nil, err
	}
	if len(raw) != 64 {
		return nil, problem("unauthenticated")
	}
	session, err := s.repo.session(ctx, tokenHash(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, problem("unauthenticated")
	}
	if err != nil {
		return nil, safe(err)
	}
	account, err := s.repo.account(ctx, Scope{TenantID: session.TenantID, All: true}, session.AccountID)
	if err != nil {
		return nil, safe(err)
	}
	account, err = s.departmentAccount(ctx, session.TenantID, account, session.ActiveDepartmentID)
	if err != nil {
		return nil, err
	}
	timing, err := s.repo.q.GovSessionTiming(ctx, tokenHash(raw))
	if err != nil {
		return nil, safe(err)
	}
	check := &Principal{Account: account, TenantID: session.TenantID}
	if err = s.sessionPolicy(ctx, check, timing.CreatedAt.Time, timing.ExpiresAt.Time, timing.LastSeenAt.Time); err != nil {
		var expired *genidentity.IAMProblem
		if errors.As(err, &expired) && expired.Code == "unauthenticated" {
			if e := s.repo.deleteSession(ctx, tokenHash(raw)); e != nil {
				return nil, safe(e)
			}
		}
		return nil, err
	}
	if err = s.repo.q.GovTouchSession(ctx, tokenHash(raw)); err != nil {
		return nil, safe(err)
	}
	grants, err := s.departmentGrants(ctx, session.TenantID, account)
	if err != nil {
		return nil, safe(err)
	}
	grants = MergeGrants(grants, account.Overrides)
	p, err := NewPrincipal(account, session.TenantID, grants)
	if err != nil {
		return nil, problem("internal")
	}
	if err = s.loadTemporary(ctx, p); err != nil {
		return nil, err
	}
	p.CSRF = session.CsrfToken
	p.TokenHash = tokenHash(raw)
	return p, nil
}
func CSRFMatches(p *Principal, value string) bool {
	return value != "" && subtle.ConstantTimeCompare([]byte(p.CSRF), []byte(value)) == 1
}
func profile(p *Principal) *genidentity.SessionProfile {
	return &genidentity.SessionProfile{Account: p.Account, CsrfToken: p.CSRF, Permissions: p.Effective(), TenantID: p.TenantID, ActiveDepartmentID: p.Account.DepartmentID}
}
func (s *Service) Login(ctx context.Context, payload *genidentity.LoginPayload) (*genidentity.LoginResult, error) {
	if err := s.available(); err != nil {
		return nil, err
	}
	username := strings.ToLower(strings.TrimSpace(payload.Username))
	key := tokenHash(username)
	account, err := s.repo.loginAccount(ctx, username)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, safe(err)
	}
	policy := DefaultSecurityPolicy()
	if err == nil {
		policy, err = s.policy(ctx, account.TenantID)
		if err != nil {
			return nil, safe(err)
		}
	}
	throttle, throttleErr := s.repo.q.IAMLoginThrottle(ctx, key)
	if throttleErr != nil && !errors.Is(throttleErr, pgx.ErrNoRows) {
		return nil, safe(throttleErr)
	}
	if throttleErr == nil && throttle.Failures >= int32(policy.LockAttempts) && throttle.LockedUntil.Time.After(time.Now()) {
		dummyPasswordWork(payload.Password)
		return nil, problem("unauthenticated")
	}
	valid := false
	if err == nil {
		valid = VerifyPassword(payload.Password, account.PasswordHash) && account.Status == "active"
	} else {
		dummyPasswordWork(payload.Password)
	}
	if !valid {
		if err = s.repo.q.GovLoginFailure(ctx, dbgen.GovLoginFailureParams{AttemptKey: key, MaxFailures: int32(policy.LockAttempts), LockMinutes: int32(policy.LockMinutes)}); err != nil {
			return nil, safe(err)
		}
		return nil, problem("unauthenticated")
	}
	stored, err := s.repo.account(ctx, Scope{TenantID: account.TenantID, All: true}, account.ID)
	if err != nil {
		return nil, safe(err)
	}
	var selected *genidentity.AccountRecord
	for _, m := range stored.Memberships {
		candidate, placementErr := s.departmentAccount(ctx, account.TenantID, stored, m.DepartmentID)
		if placementErr == nil {
			selected = candidate
			break
		}
	}
	if selected == nil {
		return nil, problem("unauthenticated")
	}
	token, err := randomToken()
	if err != nil {
		return nil, problem("internal")
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, problem("internal")
	}
	if err = s.repo.createSession(ctx, account.TenantID, account.ID, tokenHash(token), csrf, policy.SessionMinutes); err != nil {
		return nil, safe(err)
	}
	if selected.DepartmentID != stored.DepartmentID {
		if _, err = s.repo.q.IAMSwitchDepartment(ctx, dbgen.IAMSwitchDepartmentParams{TokenHash: tokenHash(token), ActiveDepartmentID: pgtype.Int8{Int64: selected.DepartmentID, Valid: true}, CsrfToken: csrf}); err != nil {
			return nil, safe(err)
		}
	}
	if err = s.repo.loginSuccess(ctx, account.TenantID, account.ID, key); err != nil {
		return nil, safe(err)
	}
	current, err := s.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	return &genidentity.LoginResult{Profile: profile(current), Cookie: s.policyCookie(token, false, policy.SessionMinutes)}, nil
}
func (s *Service) Me(ctx context.Context, _ *genidentity.MePayload) (*genidentity.SessionProfile, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	return profile(p), nil
}
func (s *Service) Logout(ctx context.Context, _ *genidentity.LogoutPayload) (*genidentity.LogoutResult, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.repo.deleteSession(ctx, p.TokenHash); err != nil {
		return nil, safe(err)
	}
	return &genidentity.LogoutResult{Cookie: s.cookie("", true)}, nil
}
func (s *Service) Snapshot(ctx context.Context, _ *genidentity.SnapshotPayload) (*genidentity.AdminSnapshot, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	out := &genidentity.AdminSnapshot{Accounts: []*genidentity.AccountRecord{}, Organizations: []*genidentity.OrganizationRecord{}, Departments: []*genidentity.DepartmentRecord{}, Roles: []*genidentity.RoleRecord{}, Permissions: append([]string{}, Permissions...), Catalog: PermissionCatalog()}
	if p.Can("accounts:read") {
		scope, err := s.scope(ctx, p, "accounts:read")
		if err != nil {
			return nil, err
		}
		out.Accounts, err = s.repo.accounts(ctx, scope)
		if err != nil {
			return nil, safe(err)
		}
	}
	if p.Can("departments:read") {
		scope, err := s.scope(ctx, p, "departments:read")
		if err != nil {
			return nil, err
		}
		out.Departments, err = s.repo.departments(ctx, scope)
		if err != nil {
			return nil, safe(err)
		}
	}
	if p.Can("organizations:read") {
		scope, err := s.scope(ctx, p, "organizations:read")
		if err != nil {
			return nil, err
		}
		if !scope.All {
			scope.OrgIDs = append(scope.OrgIDs, p.Account.OrganizationID)
		}
		out.Organizations, err = s.repo.organizations(ctx, scope)
		if err != nil {
			return nil, safe(err)
		}
	}
	if p.Can("roles:read") {
		out.Roles, err = s.repo.roles(ctx, tenantScope(p))
		if err != nil {
			return nil, safe(err)
		}
	}
	return out, nil
}
func (s *Service) validateGrants(ctx context.Context, p *Principal, grants []*genidentity.PermissionGrant, role bool) error {
	seen := map[string]bool{}
	for _, g := range grants {
		if g == nil || !knownPermission(g.Permission) || seen[g.Permission] || (role && g.Effect != "allow") {
			return problem("invalid")
		}
		seen[g.Permission] = true
		if (g.Permission == "access:manage" || g.Permission == "temporary:approve") && g.Scope != "ALL" {
			return problem("invalid")
		}
		if g.Scope == "CUSTOM" && len(g.DepartmentIds) == 0 {
			return problem("invalid")
		}
		for _, id := range g.DepartmentIds {
			if _, err := s.repo.department(ctx, tenantScope(p), id); err != nil {
				return problem("invalid")
			}
		}
	}
	return nil
}
func sameJSON(left, right any) bool {
	a, err := json.Marshal(left)
	if err != nil {
		return false
	}
	b, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(a) == string(b)
}
func (s *Service) authorized(ctx context.Context, permission string) (*Principal, Scope, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, Scope{}, err
	}
	scope, err := s.scope(ctx, p, permission)
	return p, scope, err
}
func nowString(t time.Time) string { return t.UTC().Format(time.RFC3339) }

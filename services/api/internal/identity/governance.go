package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type SecurityPolicy struct {
	PasswordMin, LockAttempts, LockMinutes, SessionMinutes, IdleMinutes, ReasonMin, MaximumMinutes int
	Upper, Lower, Digit, Symbol                                                                    bool
	AllowedMinutes                                                                                 []int
}
type PolicyProvider interface {
	SecurityPolicy(context.Context, string) (SecurityPolicy, error)
}

func DefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{PasswordMin: 12, LockAttempts: 5, LockMinutes: 15, SessionMinutes: 480, AllowedMinutes: []int{30, 60, 120, 240, 480, 1440, 4320, 10080}, ReasonMin: 5, MaximumMinutes: 10080}
}
func (s *Service) policy(ctx context.Context, tenant string) (SecurityPolicy, error) {
	if s.Policies == nil {
		return DefaultSecurityPolicy(), nil
	}
	return s.Policies.SecurityPolicy(ctx, tenant)
}
func (s *Service) Authorize(ctx context.Context, permission string, permanent bool) (*Principal, Scope, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, Scope{}, err
	}
	if permanent {
		p, err = p.permanent()
		if err != nil {
			return nil, Scope{}, err
		}
	}
	scope, err := s.scope(ctx, p, permission)
	return p, scope, safe(err)
}
func (s *Service) ScopeFor(ctx context.Context, p *Principal, permission string, permanent bool) (Scope, error) {
	if permanent {
		base, err := p.permanent()
		if err != nil {
			return Scope{}, err
		}
		p = base
	}
	return s.scope(ctx, p, permission)
}
func (s *Service) PrincipalForTask(ctx context.Context, tenant string, id, dept int64) (*Principal, error) {
	account, err := s.repo.account(ctx, Scope{TenantID: tenant, All: true}, id)
	if err != nil || account.Status != "active" {
		return nil, problem("forbidden")
	}
	account, err = s.departmentAccount(ctx, tenant, account, dept)
	if err != nil {
		return nil, err
	}
	grants, err := s.departmentGrants(ctx, tenant, account)
	if err != nil {
		return nil, err
	}
	return NewPrincipal(account, tenant, MergeGrants(grants, account.Overrides))
}
func (s *Service) VerifyCurrentPassword(ctx context.Context, p *Principal, password string) error {
	key := tokenHash("reauth:" + p.TenantID + ":" + p.TokenHash)
	blocked, err := s.repo.loginBlocked(ctx, key)
	if err != nil {
		return safe(err)
	}
	if blocked {
		dummyPasswordWork(password)
		return problem("forbidden")
	}
	row, err := s.repo.q.GovAccountPassword(ctx, dbgen.GovAccountPasswordParams{TenantID: p.TenantID, ID: p.Account.ID})
	if err != nil {
		return problem("forbidden")
	}
	if !VerifyPassword(password, row) {
		if err = s.repo.loginFailure(ctx, key); err != nil {
			return safe(err)
		}
		return problem("forbidden")
	}
	return safe(s.repo.q.IAMLoginClear(ctx, key))
}
func (s *Service) validatePolicyPassword(ctx context.Context, p *Principal, raw string) error {
	policy, err := s.policy(ctx, p.TenantID)
	if err != nil {
		return safe(err)
	}
	if utf8.RuneCountInString(raw) < policy.PasswordMin {
		return problem("invalid")
	}
	var up, low, digit, symbol bool
	for _, r := range raw {
		up = up || unicode.IsUpper(r)
		low = low || unicode.IsLower(r)
		digit = digit || unicode.IsDigit(r)
		symbol = symbol || (!unicode.IsLetter(r) && !unicode.IsDigit(r))
	}
	if policy.Upper && !up || policy.Lower && !low || policy.Digit && !digit || policy.Symbol && !symbol {
		return problem("invalid")
	}
	return nil
}
func (s *Service) sessionPolicy(ctx context.Context, p *Principal, created, expires, last time.Time) error {
	policy, err := s.policy(ctx, p.TenantID)
	if err != nil {
		return safe(err)
	}
	now := time.Now()
	if now.After(expires) || now.After(created.Add(time.Duration(policy.SessionMinutes)*time.Minute)) || (policy.IdleMinutes > 0 && now.After(last.Add(time.Duration(policy.IdleMinutes)*time.Minute))) {
		return problem("unauthenticated")
	}
	return nil
}

// Preserve existing seed role privileges as the catalog grows.
func seedViewerPermission(code string) bool {
	return code == "accounts:read" || code == "organizations:read" || code == "departments:read" || code == "roles:read"
}

type ReferenceOption struct {
	ID                      int64
	Code, Label, ParentCode string
}

// ReferenceOptions projects the existing authorized snapshot, preserving each original resource's semantics.
func (s *Service) ReferenceOptions(ctx context.Context, source, search string, ids ...int64) ([]ReferenceOption, error) {
	p, err := Current(ctx)
	if err != nil {
		return nil, err
	}
	if !p.Can(source + ":read") {
		return nil, problem("forbidden")
	}
	snapshot, err := s.Snapshot(ctx, &genidentity.SnapshotPayload{})
	if err != nil {
		return nil, err
	}
	out := []ReferenceOption{}
	add := func(id int64, code, label, parent string) {
		if (len(ids) == 0 || contains(ids, id)) && strings.Contains(strings.ToLower(label+" "+code), strings.ToLower(search)) {
			out = append(out, ReferenceOption{id, code, label, parent})
		}
	}
	switch source {
	case "organizations":
		for _, o := range snapshot.Organizations {
			if o.Status == "active" {
				add(o.ID, o.Code, o.Name, "")
			}
		}
	case "departments":
		byID := map[int64]string{}
		for _, d := range snapshot.Departments {
			byID[d.ID] = d.Code
		}
		for _, d := range snapshot.Departments {
			if d.Status == "active" {
				add(d.ID, d.Code, d.Name, byID[d.ParentID])
			}
		}
	case "roles":
		for _, r := range snapshot.Roles {
			if r.Status == "active" {
				add(r.ID, r.Code, r.Name, "")
			}
		}
	case "accounts":
		for _, a := range snapshot.Accounts {
			if a.Status == "active" {
				add(a.ID, a.Username, a.FullName, "")
			}
		}
	default:
		return nil, problem("invalid")
	}
	if len(out) > 200 {
		out = out[:200]
	}
	return out, nil
}

func (s *Service) Database() *pgxpool.Pool { return s.repo.pool }

// WithReadQueries binds authorization and reference reads to an existing transaction.
// The returned service is only for read use cases, never mounted as an HTTP service.
func (s *Service) WithReadQueries(q *dbgen.Queries) *Service {
	copy := *s
	if s.repo != nil {
		repository := *s.repo
		repository.q = q
		copy.repo = &repository
	}
	return &copy
}

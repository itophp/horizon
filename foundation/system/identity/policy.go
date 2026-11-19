package identity

import (
	"context"
	"errors"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	genidentity "horizon/foundation/gen/identity"
	"log/slog"
	"strconv"
)

var Permissions = func() []string {
	out := []string{}
	for _, definition := range PermissionCatalog() {
		out = append(out, definition.Code)
	}
	return out
}()

const policyModel = `[request_definition]
r = sub, permission
[policy_definition]
p = sub, permission, eft
[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))
[matchers]
m = r.sub == p.sub && r.permission == p.permission
`

type Principal struct {
	Account                   *genidentity.AccountRecord
	TenantID, CSRF, TokenHash string
	Grants                    []*genidentity.PermissionGrant
	PermanentGrants           []*genidentity.PermissionGrant
	TemporaryScopes           map[string][]Scope
	enforcer                  *casbin.Enforcer
	permissionCodes           []string
	system                    bool
}
type principalKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func Current(ctx context.Context) (*Principal, error) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	if !ok || p.Account == nil || p.TenantID == "" {
		return nil, problem("unauthenticated")
	}
	return p, nil
}
func NewPrincipal(account *genidentity.AccountRecord, tenant string, grants []*genidentity.PermissionGrant) (*Principal, error) {
	return newPrincipal(account, tenant, grants, Permissions)
}

// NewPrincipal constructs a principal using this service's immutable plugin catalog.
func (s *Service) NewPrincipal(account *genidentity.AccountRecord, tenant string, grants []*genidentity.PermissionGrant) (*Principal, error) {
	return newPrincipal(account, tenant, grants, s.permissionCodes())
}

func newPrincipal(account *genidentity.AccountRecord, tenant string, grants []*genidentity.PermissionGrant, codes []string) (*Principal, error) {
	m, err := model.NewModelFromString(policyModel)
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	p := &Principal{Account: account, TenantID: tenant, Grants: append([]*genidentity.PermissionGrant{}, grants...), PermanentGrants: grants, TemporaryScopes: map[string][]Scope{}, enforcer: e, permissionCodes: append([]string(nil), codes...), system: account != nil && account.ID == 0}
	for _, g := range grants {
		if g == nil {
			return nil, errors.New("invalid stored policy")
		}
		if _, err = e.AddPolicy(strconv.FormatInt(account.ID, 10), g.Permission, g.Effect); err != nil {
			return nil, err
		}
	}
	return p, nil
}
func (p *Principal) Can(permission string) bool {
	// In-process worker principals (account zero) receive their grants from
	// calling code instead of the editable catalog, so every granted
	// permission is known without catalog membership.
	known := p.system
	for _, code := range p.permissionCodes {
		if code == permission {
			known = true
			break
		}
	}
	if !known {
		return false
	}
	if p.Account.IsAdmin {
		return true
	}
	allowed, err := p.enforcer.Enforce(strconv.FormatInt(p.Account.ID, 10), permission)
	if err != nil {
		slog.Error("authorization evaluation failed", "error", err)
		return false
	}
	return allowed
}
func (p *Principal) Effective() []string {
	out := []string{}
	for _, permission := range p.permissionCodes {
		if p.Can(permission) {
			out = append(out, permission)
		}
	}
	// Account zero is reserved for in-process workers, never an authenticated
	// human account. Their effective set is exactly the granted permissions.
	if p.system {
		for _, grant := range p.Grants {
			if grant != nil && grant.Effect == "allow" {
				out = append(out, grant.Permission)
			}
		}
	}
	return out
}

type Scope struct {
	TenantID        string
	All             bool
	OwnID           int64
	OrgIDs, DeptIDs []int64
}

func tenantScope(p *Principal) Scope {
	return Scope{TenantID: p.TenantID, All: true, OrgIDs: []int64{}, DeptIDs: []int64{}}
}
func (s *Service) scope(ctx context.Context, p *Principal, permission string) (Scope, error) {
	scope := Scope{TenantID: p.TenantID, OrgIDs: []int64{}, DeptIDs: []int64{}}
	if !p.Can(permission) {
		return scope, problem("forbidden")
	}
	if p.Account.IsAdmin {
		scope.All = true
		return scope, nil
	}
	for _, g := range p.PermanentGrants {
		if g.Permission != permission || g.Effect != "allow" {
			continue
		}
		switch g.Scope {
		case "ALL":
			scope.All = true
		case "ORGANIZATION":
			scope.OrgIDs = append(scope.OrgIDs, p.Account.OrganizationID)
		case "DEPARTMENT":
			scope.DeptIDs = append(scope.DeptIDs, p.Account.DepartmentID)
		case "DEPARTMENT_TREE":
			ids, err := s.repo.departmentTree(ctx, p.TenantID, p.Account.DepartmentID)
			if err != nil {
				return scope, err
			}
			scope.DeptIDs = append(scope.DeptIDs, ids...)
		case "OWN":
			scope.OwnID = p.Account.ID
		case "CUSTOM":
			scope.DeptIDs = append(scope.DeptIDs, g.DepartmentIds...)
		default:
			return scope, problem("forbidden")
		}
	}
	for _, temporary := range p.TemporaryScopes[permission] {
		scope.All = scope.All || temporary.All
		scope.OrgIDs = append(scope.OrgIDs, temporary.OrgIDs...)
		scope.DeptIDs = append(scope.DeptIDs, temporary.DeptIDs...)
		if temporary.OwnID != 0 {
			scope.OwnID = temporary.OwnID
		}
	}
	return scope, nil
}

// MergeGrants lets an explicit account rule replace the inherited rule and data scope for the same operation.
func MergeGrants(roles, overrides []*genidentity.PermissionGrant) []*genidentity.PermissionGrant {
	keys := map[string]bool{}
	for _, g := range overrides {
		if g != nil {
			keys[g.Permission] = true
		}
	}
	out := []*genidentity.PermissionGrant{}
	for _, g := range roles {
		if g == nil || !keys[g.Permission] {
			out = append(out, g)
		}
	}
	return append(out, overrides...)
}

// CanPermanent excludes temporary grants for installation and governance operations.
func (p *Principal) CanPermanent(permission string) bool {
	base, err := p.permanent()
	return err == nil && base.Can(permission)
}

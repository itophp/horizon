package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// authorizationVersion contains no CSRF nonce, credential, name or policy text.
// The hash changes when effective grants/scopes change even if permission codes do not.
func authorizationVersion(p *Principal) string {
	parts := []string{fmt.Sprintf("principal:%q:%d:%d:%d:%t", p.TenantID, p.Account.ID, p.Account.DepartmentID, p.Account.OrganizationID, p.Account.IsAdmin)}
	for _, permission := range p.Effective() {
		parts = append(parts, fmt.Sprintf("permission:%q", permission))
	}
	for _, grant := range p.Grants {
		ids := append([]int64{}, grant.DepartmentIds...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		parts = append(parts, fmt.Sprintf("grant:%q:%q:%q:%v", grant.Permission, grant.Effect, grant.Scope, ids))
	}
	for permission, scopes := range p.TemporaryScopes {
		for _, scope := range scopes {
			orgs := append([]int64{}, scope.OrgIDs...)
			depts := append([]int64{}, scope.DeptIDs...)
			sort.Slice(orgs, func(i, j int) bool { return orgs[i] < orgs[j] })
			sort.Slice(depts, func(i, j int) bool { return depts[i] < depts[j] })
			parts = append(parts, fmt.Sprintf("temporary:%q:%q:%t:%d:%v:%v", permission, scope.TenantID, scope.All, scope.OwnID, orgs, depts))
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(fmt.Sprint(parts)))
	return hex.EncodeToString(sum[:])
}

// AuthorizationVersion is a non-sensitive freshness stamp, not a permission grant.
func AuthorizationVersion(p *Principal) string { return authorizationVersion(p) }

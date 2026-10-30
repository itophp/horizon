package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"sort"
)

func PermissionCatalog() []*genidentity.PermissionDefinition {
	out := []*genidentity.PermissionDefinition{}
	for _, resource := range []string{"accounts", "organizations", "departments", "roles"} {
		for _, action := range []string{"read", "create", "update", "delete"} {
			out = append(out, &genidentity.PermissionDefinition{Code: resource + ":" + action, Module: "identity", Function: resource, Action: action, TemporaryEligible: resource != "roles" || action == "read", ScopeConfigurable: resource != "roles"})
		}
	}
	out = append(out,
		&genidentity.PermissionDefinition{Code: "access:manage", Module: "access", Function: "policies", Action: "manage"},
		&genidentity.PermissionDefinition{Code: "temporary:approve", Module: "access", Function: "temporary", Action: "approve"})
	for resource, actions := range map[string][]string{"settings": {"read", "update", "publish", "history", "rollback", "export"}, "dictionaries": {"read", "create", "update", "delete", "publish", "import", "export", "history", "rollback", "use"}, "files": {"read", "create", "delete"}, "notifications": {"read", "retry", "test", "send"}} {
		for _, action := range actions {
			out = append(out, &genidentity.PermissionDefinition{Code: resource + ":" + action, Module: "system", Function: resource, Action: action, ScopeConfigurable: resource != "settings" && (resource != "dictionaries" || action == "use"), TemporaryEligible: resource == "files" && action == "read" || resource == "dictionaries" && action == "use"})
		}
	}
	sort.SliceStable(out[18:], func(i, j int) bool { return out[18+i].Code < out[18+j].Code })
	return out
}
func permissionDefinition(code string) *genidentity.PermissionDefinition {
	for _, definition := range PermissionCatalog() {
		if definition.Code == code {
			return definition
		}
	}
	return nil
}
func (p *Principal) permanent() (*Principal, error) {
	return NewPrincipal(p.Account, p.TenantID, p.PermanentGrants)
}

package identity

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/plugin"
	"fmt"
	"reflect"
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
		&genidentity.PermissionDefinition{Code: "temporary:approve", Module: "access", Function: "temporary", Action: "approve"},
		&genidentity.PermissionDefinition{Code: "capability:approve", Module: "system", Function: "capability", Action: "approve", ScopeConfigurable: false, TemporaryEligible: false},
		&genidentity.PermissionDefinition{Code: "mcp:revoke", Module: "system", Function: "mcp", Action: "revoke", ScopeConfigurable: false, TemporaryEligible: false})
	out = append(out,
		&genidentity.PermissionDefinition{Code: "example:read", Module: "business", Function: "example", Action: "read", ScopeConfigurable: false, TemporaryEligible: false},
		&genidentity.PermissionDefinition{Code: "example:create", Module: "business", Function: "example", Action: "create", ScopeConfigurable: false, TemporaryEligible: false},
		&genidentity.PermissionDefinition{Code: "example:archive", Module: "business", Function: "example", Action: "archive", ScopeConfigurable: false, TemporaryEligible: false})
	for resource, actions := range map[string][]string{"memory": {"read", "write", "collect", "replay", "manage"}, "applications": {"read", "design", "manage"}, "drive": {"use", "create"}, "articles": {"read", "write", "review"}, "projects": {"read", "write", "admin"}, "orchestration": {"read", "run", "cancel"}, "settings": {"read", "update", "publish", "history", "rollback", "export"}, "dictionaries": {"read", "create", "update", "delete", "publish", "import", "export", "history", "rollback", "use"}, "files": {"read", "create", "delete"}, "notifications": {"read", "retry", "test", "send"}, "assistant": {"use"}, "knowledge": {"read", "create", "delete"}} {
		for _, action := range actions {
			out = append(out, &genidentity.PermissionDefinition{Code: resource + ":" + action, Module: "system", Function: resource, Action: action, ScopeConfigurable: resource != "memory" && resource != "applications" && resource != "drive" && resource != "articles" && resource != "orchestration" && resource != "settings" && (resource != "dictionaries" || action == "use"), TemporaryEligible: resource == "files" && action == "read" || resource == "dictionaries" && action == "use"})
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
	return newPrincipal(p.Account, p.TenantID, p.PermanentGrants, p.permissionCodes)
}

// ConfigurePluginPermissions builds an instance-local catalog before serving requests.
// A declaration matching an existing catalog entry is a reference, never a grant.
func (s *Service) ConfigurePluginPermissions(definitions []plugin.Permission) error {
	catalog := PermissionCatalog()
	byCode := make(map[string]*genidentity.PermissionDefinition, len(catalog))
	for _, definition := range catalog {
		byCode[definition.Code] = definition
	}
	seen := map[string]bool{}
	for _, d := range definitions {
		if d.Code == "" || d.Module == "" || d.Function == "" || d.Action == "" || d.Code != d.Function+":"+d.Action || seen[d.Code] {
			return fmt.Errorf("invalid plugin permission %q", d.Code)
		}
		seen[d.Code] = true
		if existing := byCode[d.Code]; existing != nil {
			if existing.Module != d.Module || existing.Function != d.Function || existing.Action != d.Action || existing.ScopeConfigurable != d.ScopeConfigurable || existing.TemporaryEligible != d.TemporaryEligible {
				return fmt.Errorf("plugin permission %q conflicts with identity catalog", d.Code)
			}
			continue
		}
		definition := &genidentity.PermissionDefinition{Code: d.Code, Module: d.Module, Function: d.Function, Action: d.Action, ScopeConfigurable: d.ScopeConfigurable, TemporaryEligible: d.TemporaryEligible}
		catalog = append(catalog, definition)
		byCode[d.Code] = definition
	}
	sort.SliceStable(catalog[18:], func(i, j int) bool { return catalog[18+i].Code < catalog[18+j].Code })
	if s.pluginCatalog != nil {
		if !reflect.DeepEqual(s.pluginCatalog, catalog) {
			return fmt.Errorf("plugin permissions already configured")
		}
		return nil
	}
	s.pluginCatalog = catalog
	return nil
}
func (s *Service) permissionCatalog() []*genidentity.PermissionDefinition {
	if s.pluginCatalog == nil {
		return PermissionCatalog()
	}
	result := make([]*genidentity.PermissionDefinition, 0, len(s.pluginCatalog))
	for _, definition := range s.pluginCatalog {
		copy := *definition
		result = append(result, &copy)
	}
	return result
}
func (s *Service) permissionDefinition(code string) *genidentity.PermissionDefinition {
	for _, definition := range s.permissionCatalog() {
		if definition.Code == code {
			return definition
		}
	}
	return nil
}
func (s *Service) permissionCodes() []string {
	catalog := s.permissionCatalog()
	out := make([]string, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, d.Code)
	}
	return out
}

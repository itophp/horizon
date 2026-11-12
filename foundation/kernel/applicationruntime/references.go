package applicationruntime

import (
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
	"regexp"
	"strings"
)

// AuthenticatedPermission is available to Runtime applications whose individual
// providers and actions enforce their own permissions.
const AuthenticatedPermission = "authenticated"

var routePattern = regexp.MustCompile(`^/[a-zA-Z0-9_:/-]*$`)
var appVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// CoreFieldComponents is shared by form contracts and the authorized design catalog.
func CoreFieldComponents() []string {
	return []string{"text", "textarea", "string", "integer", "number", "boolean", "enum", "date", "reference", "timestamp", "money", "time", "datetime", "multiselect", "file", "subtable", "switch", "markdown"}
}

func validateReferences(app ApplicationV2, definitions map[string]contracts.Definition) error {
	if app.Protocol == nil {
		return fmt.Errorf("schema_version_unsupported")
	}
	if !appVersionPattern.MatchString(app.Version) {
		return fmt.Errorf("schema_invalid: application version")
	}
	knownPermissions := map[string]bool{}
	if strings.HasPrefix(app.Source, "builtin:") {
		knownPermissions[AuthenticatedPermission] = true
	}
	for _, definition := range definitions {
		if definition.Permission != "" {
			knownPermissions[definition.Permission] = true
		}
	}
	if !knownPermissions[app.Permission] || !knownPermissions[app.Protocol.Permission] {
		return fmt.Errorf("schema_invalid: unknown application permission")
	}
	for _, provider := range app.Protocol.Providers {
		systemSchema, systemPermission := systemProviderInput(provider.Adapter)
		if provider.Adapter != "capability" && systemSchema == nil {
			return fmt.Errorf("schema_invalid: provider adapter")
		}
		if provider.Pagination != "" && provider.Pagination != "none" && provider.Pagination != "server" {
			return fmt.Errorf("schema_invalid: pagination")
		}
		if provider.PollInterval < 0 || provider.PollInterval > 0 && provider.PollInterval < 1000 {
			return fmt.Errorf("schema_invalid: polling interval")
		}
		if systemSchema != nil {
			if provider.Permission != systemPermission {
				return fmt.Errorf("schema_invalid: system provider permission")
			}
		} else if provider.Permission == "" || !knownPermissions[provider.Permission] {
			return fmt.Errorf("schema_invalid: provider permission")
		}
	}
	for _, page := range app.Protocol.Pages {
		if page.Route == "" || !routePattern.MatchString(page.Route) || strings.HasPrefix(page.Route, "//") || strings.Contains(page.Route, "..") {
			return fmt.Errorf("schema_invalid: route")
		}
		if page.Permission == "" || !knownPermissions[page.Permission] {
			return fmt.Errorf("schema_invalid: unknown page permission")
		}
	}
	return nil
}

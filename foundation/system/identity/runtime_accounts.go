package identity

import ar "horizon/foundation/kernel/applicationruntime"

func AccountsRuntimeApplication() ar.ApplicationV2 {
	accounts := ar.Provider{
		ID: "protocol.accounts", Capability: ar.Ref("identity.accounts.admin.read"),
		KeyField: "id", LabelField: "fullName", Pagination: "none", Tags: []string{"identity"},
	}
	organizations := ar.Provider{
		ID: "protocol.organization.options", Capability: ar.Ref("identity.organizations.read"),
		ResultPath: "items", KeyField: "id", LabelField: "name", Pagination: "none", Tags: []string{"identity"},
	}
	departments := ar.Provider{
		ID: "protocol.department.options", Capability: ar.Ref("identity.departments.read"),
		ResultPath: "items", KeyField: "id", LabelField: "name", Pagination: "none", Tags: []string{"identity"},
	}
	roles := ar.Provider{
		ID: "protocol.role.options", Capability: ar.Ref("identity.roles.read"),
		ResultPath: "items", KeyField: "id", LabelField: "name", Pagination: "none", Tags: []string{"identity"},
	}
	form := accountRuntimeForm()
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "accounts", Version: "2.3.0", Source: "system.identity",
		RuntimeCompatibility: "3.0", LabelKey: "admin:accounts.title", Permission: "accounts:read", Route: "/accounts",
		Pages: []ar.PageV3{{
			SchemaVersion: "3.0", ID: "accounts.console", Route: "/accounts",
			TitleKey: "admin:accounts.title", SubtitleKey: "admin:accounts.subtitle", Permission: "accounts:read",
			Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
				ID: "accounts-root", Kind: "stack", Props: ar.LayoutProps{Gap: "20px", Width: "full"},
				Data: []ar.DataSpec{{ID: "directory", Provider: accounts.ID}},
				Children: []ar.Node{
					{Type: "layout", Layout: &ar.LayoutSpec{
						ID: "accounts-heading", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"},
						Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{
							ID: "heading", Kind: "page-heading", Version: "1.0",
							Props: map[string]any{"titleKey": "admin:accounts.title", "subtitleKey": "admin:accounts.subtitle", "actions": []map[string]any{{"labelKey": "admin:accounts.create", "icon": "plus", "permission": "accounts:create", "dialog": "create"}}},
						}}},
					}},
					{Type: "layout", Layout: &ar.LayoutSpec{
						ID: "accounts-panel", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"},
						Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{
							ID: "directory", Kind: "table-panel", Version: "1.0", Data: "$directory",
							Props: map[string]any{
								"rowsPath": "items", "keyField": "id", "searchParam": "q", "selectable": true,
								"searchFields": []string{"fullName", "username", "email"},
								"references":   map[string]string{"organizations": "organizations", "departments": "departments", "roles": "roles"},
								"tabs": map[string]any{"param": "status", "field": "status", "options": []map[string]any{
									{"value": "all", "labelKey": "admin:actions.all"},
									{"value": "active", "labelKey": "admin:status.active"},
									{"value": "inactive", "labelKey": "admin:status.inactive"},
								}},
								"select": map[string]any{"param": "status", "field": "status", "options": []map[string]any{
									{"value": "all", "labelKey": "admin:status.all"},
									{"value": "active", "labelKey": "admin:status.active"},
									{"value": "inactive", "labelKey": "admin:status.inactive"},
								}},
								"columns": []map[string]any{
									{"name": "fullName", "labelKey": "admin:fields.fullName", "display": "account", "titleField": "fullName", "subtitleField": "email", "fallbackField": "username", "flagField": "isAdmin"},
									{"name": "username", "labelKey": "admin:fields.username", "display": "mono"},
									{"name": "organizationId", "labelKey": "admin:fields.organization", "display": "lookup", "reference": "organizations", "fallbackField": "organizationId"},
									{"name": "memberships", "labelKey": "admin:fields.department", "display": "badges", "itemsField": "memberships", "itemTextField": "departmentName", "itemFallbackField": "departmentId", "reference": "departments"},
									{"name": "roleIds", "labelKey": "admin:fields.role", "display": "relatedBadges", "flagField": "isAdmin", "flagLabelKey": "admin:accounts.admin", "itemsField": "memberships", "itemTextField": "departmentName", "itemFallbackField": "departmentId", "reference": "departments", "relationField": "roleIds", "relationReference": "roles", "emptyKey": "admin:accounts.noRoles"},
									{"name": "status", "labelKey": "admin:fields.status", "display": "status"},
									{"name": "createdAt", "labelKey": "admin:fields.createdAt", "display": "date"},
								},
								"rowActions": []map[string]any{
									{"labelKey": "admin:actions.details", "icon": "eye", "mode": "view"},
									{"labelKey": "admin:actions.edit", "icon": "pencil", "mode": "edit", "permission": "accounts:update"},
									{"labelKey": "admin:actions.manageAccess", "icon": "key", "mode": "access", "permission": "accounts:update", "requirePermissions": []string{"access:manage"}, "notFlagUnlessSelf": true},
									{"labelKey": "admin:actions.delete", "icon": "trash", "mode": "delete", "permission": "accounts:delete", "notFlagField": "isAdmin", "notSelf": true},
								},
								"createInHeading": true,
								"forms":           map[string]any{"create": form, "edit": form, "view": form, "access": form},
								"dialogTitles": map[string]any{
									"create": map[string]any{"titleKey": "admin:accounts.create", "descriptionKey": "admin:accounts.description", "wide": true},
									"edit":   map[string]any{"titleKey": "admin:accounts.edit", "descriptionKey": "admin:accounts.description", "wide": true},
									"view":   map[string]any{"titleKey": "admin:accounts.details", "descriptionKey": "admin:accounts.description", "wide": true},
									"access": map[string]any{"titleKey": "admin:actions.manageAccess", "descriptionKey": "admin:accounts.description", "wide": true},
								},
								"delete":           map[string]any{"capability": "identity.account.delete", "version": "1.0.0", "idField": "id"},
								"deleteLabelField": "fullName",
							},
						}}},
					}},
				},
			}},
		}},
		Providers:  []ar.Provider{accounts, organizations, departments, roles},
		Navigation: []ar.Navigation{{PageID: "accounts.console", LabelKey: "admin:nav.accounts", Icon: "users"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: "2.3.0", MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}

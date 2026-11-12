package identity

import ar "horizon/foundation/kernel/applicationruntime"

func PermissionsRuntimeApplication() ar.ApplicationV2 {
	roles := ar.Provider{
		ID: "protocol.roles", Capability: ar.Ref("identity.roles.read"),
		KeyField: "id", LabelField: "name", Pagination: "none", Tags: []string{"identity"},
	}
	accounts := ar.Provider{
		ID: "protocol.permission.accounts", Capability: ar.Ref("identity.accounts.admin.read"),
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
	roleOptions := ar.Provider{
		ID: "protocol.role.options", Capability: ar.Ref("identity.roles.read"),
		ResultPath: "items", KeyField: "id", LabelField: "name", Pagination: "none", Tags: []string{"identity"},
	}
	roleForm := roleRuntimeForm()
	roleAccessForm := roleAccessRuntimeForm()
	accountForm := accountAccessRuntimeForm()
	rolePanel := map[string]any{
		"id": "role", "labelKey": "admin:permissions.roleTab", "icon": "shield", "permission": "roles:read",
		"type": "master-detail", "rowsPath": "roles.items", "keyField": "id", "selectedParam": "id", "titleKey": "admin:permissions.selectRole", "itemIcon": "shield",
		"titleField": "name", "subtitleField": "description", "form": roleAccessForm, "mode": "edit",
	}
	accountPanel := map[string]any{
		"id": "account", "labelKey": "admin:permissions.accountTab", "icon": "users", "permission": "accounts:read",
		"type": "master-detail", "rowsPath": "_accounts.items", "keyField": "id", "selectedParam": "id", "titleKey": "admin:permissions.selectAccount", "itemIcon": "users",
		"titleField": "fullName", "subtitleField": "username", "form": accountForm, "mode": "edit",
	}
	rolesPanel := map[string]any{
		"id": "roles", "labelKey": "admin:permissions.rolesTab", "icon": "key", "permission": "roles:read",
		"type": "cards", "rowsPath": "roles.items", "searchFields": []string{"name", "code", "description"},
		"columns": []map[string]any{
			{"name": "name", "labelKey": "admin:fields.name", "display": "icon", "icon": "shield", "subtitleField": "code"},
			{"name": "grants", "labelKey": "admin:permissions.dataScope", "display": "scope"},
			{"name": "memberCount", "labelKey": "admin:fields.members", "display": "countText", "field": "memberCount", "templateKey": "admin:roles.members"},
			{"name": "status", "labelKey": "admin:fields.status", "display": "status"},
			{"name": "createdAt", "labelKey": "admin:fields.createdAt", "display": "date"},
		},
		"totalKey": "admin:roles.total", "detailHref": "/permissions?type=role&id=:id",
		"createKey": "admin:roles.create", "createPermission": "roles:create",
		"forms": map[string]any{"create": roleForm, "edit": roleForm, "view": roleForm},
		"dialogTitles": map[string]any{
			"create": map[string]any{"titleKey": "admin:roles.create", "descriptionKey": "admin:roles.description", "wide": true},
			"edit":   map[string]any{"titleKey": "admin:roles.edit", "descriptionKey": "admin:roles.description", "wide": true},
			"view":   map[string]any{"titleKey": "admin:roles.title", "descriptionKey": "admin:roles.description", "wide": true},
		},
		"delete": map[string]any{"capability": "identity.role.delete", "version": "1.0.0"}, "deleteLabelField": "name",
		"rowActions": []map[string]any{
			{"labelKey": "admin:actions.details", "icon": "eye", "mode": "view"},
			{"labelKey": "admin:actions.manageAccess", "icon": "pencil", "mode": "edit", "permission": "roles:update", "requirePermissions": []string{"access:manage"}},
			{"labelKey": "admin:actions.delete", "icon": "trash", "mode": "delete", "permission": "roles:delete", "requirePermissions": []string{"access:manage"}},
		},
	}
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "permissions", Version: "2.3.0", Source: "system.identity",
		RuntimeCompatibility: "3.0", LabelKey: "admin:permissions.title", Permission: "roles:read", Route: "/permissions",
		Pages: []ar.PageV3{{
			SchemaVersion: "3.0", ID: "permissions.console", Route: "/permissions", TitleKey: "admin:permissions.title",
			SubtitleKey: "admin:permissions.subtitle", Permission: "roles:read",
			Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
				ID: "permissions-root", Kind: "stack", Props: ar.LayoutProps{Gap: "20px", Width: "full"},
				Data: []ar.DataSpec{{ID: "roles", Provider: roles.ID}, {ID: "_accounts", Provider: accounts.ID}},
				Children: []ar.Node{
					{Type: "layout", Layout: &ar.LayoutSpec{ID: "heading", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{
						{Type: "component", Component: &ar.ComponentSpec{ID: "heading", Kind: "page-heading", Version: "1.0", Props: map[string]any{"titleKey": "admin:permissions.title", "subtitleKey": "admin:permissions.subtitle"}}},
					}}},
					{Type: "layout", Layout: &ar.LayoutSpec{ID: "tabs", Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"}, Children: []ar.Node{
						{Type: "component", Component: &ar.ComponentSpec{ID: "views", Kind: "page-tabs", Version: "1.0", Props: map[string]any{
							"param": "type", "panels": []map[string]any{rolePanel, accountPanel, rolesPanel},
						}}},
					}}},
				},
			}},
		}},
		Providers:  []ar.Provider{roles, accounts, organizations, departments, roleOptions},
		Navigation: []ar.Navigation{{PageID: "permissions.console", LabelKey: "admin:nav.permissions", Icon: "shield"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: "2.3.0", MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}

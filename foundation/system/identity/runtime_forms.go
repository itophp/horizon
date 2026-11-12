package identity

import ar "horizon/foundation/kernel/applicationruntime"

func protocolField(name, labelKey, typ string) ar.FieldV2 {
	return ar.FieldV2{Name: name, LabelKey: labelKey, Type: typ, Component: typ, ComponentVersion: "2.0"}
}

func protocolReference(name, labelKey, provider string) ar.FieldV2 {
	field := protocolField(name, labelKey, "reference")
	field.Reference = provider
	field.SearchParam = "keyword"
	return field
}

func protocolEnum(name, labelKey, prefix string, values ...string) ar.FieldV2 {
	field := protocolField(name, labelKey, "enum")
	field.Enum = values
	field.ValueLabelPrefix = prefix
	if len(values) > 0 {
		field.Default = ar.Literal(`"` + values[0] + `"`)
	}
	return field
}

func protocolMultiReference(name, labelKey, provider string) ar.FieldV2 {
	field := protocolReference(name, labelKey, provider)
	field.Multiple = true
	field.Type = "array"
	field.Component = "multiselect"
	return field
}

func accountRuntimeForm() ar.FormV2 {
	basics := []ar.FieldV2{
		protocolField("fullName", "admin:fields.fullName", "string"),
		protocolField("username", "admin:fields.username", "string"),
		protocolField("email", "admin:fields.email", "string"),
		protocolEnum("status", "admin:fields.status", "admin:status.", "active", "inactive"),
		protocolReference("organizationId", "admin:fields.organization", "protocol.organization.options"),
		protocolReference("departmentId", "admin:fields.department", "protocol.department.options"),
	}
	roleIds := protocolMultiReference("roleIds", "admin:fields.role", "protocol.role.options")
	roleIds.Hidden = true
	overrides := protocolField("overrides", "admin:permissions.grants", "array")
	overrides.Hidden = true
	departmentAccess := protocolField("departmentAccess", "admin:memberships.title", "array")
	departmentAccess.Component = "membership-editor"
	departmentAccess.Group = "access"
	createOnly := protocolField("password", "admin:accounts.newPassword", "string")
	createOnly.Required = true
	createOnly.Group = "basics"
	for i := range basics {
		basics[i].Required = true
		basics[i].Group = "basics"
	}
	basics[2].Required = false
	basics[3].DisabledWhen = ar.Condition(
		"eq",
		ar.Path("record.isAdmin"),
		ar.Literal(`true`),
	)
	fields := append(basics, createOnly)
	fields = append(fields, roleIds, overrides, departmentAccess)
	form := ar.FormV2{
		ID: "account.form", LabelKey: "admin:accounts.title", Tags: []string{"identity"},
		Groups: []ar.FormGroup{
			{ID: "basics", LabelKey: "admin:accounts.basics", Type: "tabs"},
			{ID: "access", LabelKey: "admin:memberships.tab", Type: "tabs"},
		},
		ReferenceFilters: map[string][]ar.ReferenceFilter{
			"organizationId": {
				{OptionField: "kind", Value: "company"},
				{OptionField: "status", Value: "active"},
			},
			"departmentId": {
				{OptionField: "organizationId", FormField: "organizationId"},
				{OptionField: "status", Value: "active"},
			},
		},
		ReferencePaths: map[string]string{
			"protocol.organization.options": "organizations",
			"protocol.department.options":   "departments",
			"protocol.role.options":         "roles",
		},
	}
	form.Create = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.account.create"), Fields: fields}
	form.Edit = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.account.update"), Fields: append([]ar.FieldV2{protocolField("id", "admin:fields.name", "integer")}, fields...)}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	return ar.Canonical(form)
}

func organizationRuntimeForm() ar.FormV2 {
	fields := []ar.FieldV2{
		protocolField("name", "admin:fields.name", "string"),
		protocolField("code", "admin:fields.code", "string"),
		protocolEnum("kind", "admin:organizations.kind", "admin:organizations.", "enterprise", "company"),
		protocolReference("parentId", "admin:organizations.parent", "protocol.parent.options"),
		protocolEnum("status", "admin:fields.status", "admin:status.", "active", "inactive"),
		{LabelKey: "admin:fields.description", Name: "description", Type: "string", Component: "textarea", ComponentVersion: "2.0"},
	}
	form := ar.FormV2{ID: "organization.form", LabelKey: "admin:organizations.title", Tags: []string{"identity"}}
	form.Create = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.organization.create"), Fields: fields}
	editFields := append([]ar.FieldV2{protocolField("id", "admin:fields.name", "integer")}, fields...)
	for i := range editFields {
		if editFields[i].Name == "kind" {
			editFields[i].ReadOnly = true
		}
	}
	form.Edit = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.organization.update"), Fields: editFields}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	form.ReferenceFilters = map[string][]ar.ReferenceFilter{
		"parentId": {
			{OptionField: "kind", Value: "enterprise"},
			{OptionField: "status", Value: "active"},
		},
	}
	form.ReferencePaths = map[string]string{
		"protocol.parent.options": "organizations.items",
	}
	return ar.Canonical(form)
}

func departmentRuntimeForm() ar.FormV2 {
	fields := []ar.FieldV2{
		protocolField("name", "admin:fields.name", "string"),
		protocolField("code", "admin:fields.code", "string"),
		protocolReference("organizationId", "admin:fields.organization", "protocol.organization.options"),
		protocolReference("parentId", "admin:fields.parent", "protocol.department.options"),
		protocolField("manager", "admin:departments.manager", "string"),
		protocolEnum("status", "admin:fields.status", "admin:status.", "active", "inactive"),
		{LabelKey: "admin:fields.description", Name: "description", Type: "string", Component: "textarea", ComponentVersion: "2.0"},
	}
	form := ar.FormV2{ID: "department.form", LabelKey: "admin:departments.title", Tags: []string{"identity"}}
	form.Create = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.department.create"), Fields: fields}
	form.Edit = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.department.update"), Fields: append([]ar.FieldV2{protocolField("id", "admin:fields.name", "integer")}, fields...)}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	form.ReferenceFilters = map[string][]ar.ReferenceFilter{
		"organizationId": {
			{OptionField: "kind", Value: "company"},
			{OptionField: "status", Value: "active"},
		},
		"parentId": {
			{OptionField: "organizationId", FormField: "organizationId"},
			{OptionField: "status", Value: "active"},
		},
	}
	form.ReferencePaths = map[string]string{
		"protocol.organization.options": "organizations.items",
		"protocol.department.options":   "_departments.items",
	}
	return ar.Canonical(form)
}

func grantFields() []ar.FieldV2 {
	fields := []ar.FieldV2{
		protocolField("permission", "admin:permissions.permission", "string"),
		protocolEnum("effect", "admin:permissions.effect", "admin:permissions.", "allow", "deny"),
		protocolEnum("scope", "admin:permissions.dataScope", "admin:permissions.", "ALL", "ORGANIZATION", "DEPARTMENT", "DEPARTMENT_TREE", "OWN", "CUSTOM"),
	}
	for i := range fields {
		fields[i].Required = true
	}
	departments := protocolMultiReference("departmentIds", "admin:permissions.customDepartments", "protocol.department.options")
	return append(fields, departments)
}

func roleRuntimeForm() ar.FormV2 {
	grants := protocolField("grants", "admin:permissions.grants", "array")
	grants.Component = "permission-matrix"
	grants.Children = grantFields()
	fields := []ar.FieldV2{
		protocolField("name", "admin:fields.name", "string"),
		protocolField("code", "admin:fields.code", "string"),
		{LabelKey: "admin:fields.description", Name: "description", Type: "string", Component: "textarea", ComponentVersion: "2.0"},
		protocolEnum("status", "admin:fields.status", "admin:status.", "active", "inactive"),
		grants,
	}
	form := ar.FormV2{ID: "role.form", LabelKey: "admin:roles.title", Tags: []string{"identity"}}
	form.Create = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.role.create"), Fields: fields}
	form.Edit = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.role.update"), Fields: append([]ar.FieldV2{protocolField("id", "admin:fields.name", "integer")}, fields...)}
	for _, field := range fields {
		form.Create.Input = append(form.Create.Input, ar.Bind(field.Name, "form."+field.Name))
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	form.ReferencePaths = map[string]string{
		"protocol.department.options": "departments",
		"catalog":                     "roles.catalog",
		"departments":                 "roles.departments",
	}
	return ar.Canonical(form)
}

func accountAccessRuntimeForm() ar.FormV2 {
	hidden := []ar.FieldV2{
		protocolField("id", "admin:fields.name", "integer"),
		protocolField("username", "admin:fields.username", "string"),
		protocolField("fullName", "admin:fields.fullName", "string"),
		protocolField("email", "admin:fields.email", "string"),
		protocolEnum("status", "admin:fields.status", "admin:status.", "active", "inactive"),
		protocolReference("organizationId", "admin:fields.organization", "protocol.organization.options"),
		protocolReference("departmentId", "admin:fields.department", "protocol.department.options"),
		protocolMultiReference("roleIds", "admin:fields.role", "protocol.role.options"),
		protocolField("overrides", "admin:permissions.grants", "array"),
	}
	for i := range hidden {
		hidden[i].Hidden = true
		hidden[i].ReadOnly = true
	}
	departmentAccess := protocolField("departmentAccess", "admin:memberships.title", "array")
	departmentAccess.Component = "membership-editor"
	fields := append(hidden, departmentAccess)
	form := ar.FormV2{
		ID: "account.access.form", LabelKey: "admin:memberships.title", Tags: []string{"identity"},
		ReferencePaths: map[string]string{
			"protocol.organization.options": "_accounts.organizations",
			"protocol.department.options":   "_accounts.departments",
			"protocol.role.options":         "_accounts.roles",
		},
	}
	form.Edit = ar.Command{ActionType: "capability", Capability: ar.Ref("identity.account.update"), Fields: fields}
	for _, field := range fields {
		form.Edit.Input = append(form.Edit.Input, ar.Bind(field.Name, "form."+field.Name))
	}
	form.Edit.Input = append(form.Edit.Input, ar.Bind("id", "record.id"))
	return ar.Canonical(form)
}

func roleAccessRuntimeForm() ar.FormV2 {
	roleForm := roleRuntimeForm()
	fields := make([]ar.FieldV2, 0, len(roleForm.Edit.Fields))
	for _, field := range roleForm.Edit.Fields {
		if field.Name != "grants" {
			field.Hidden = true
			field.ReadOnly = true
		}
		fields = append(fields, field)
	}
	form := ar.FormV2{
		ID: "role.access.form", LabelKey: "admin:permissions.matrix", Tags: []string{"identity"},
		ReferencePaths: map[string]string{
			"catalog":     "roles.catalog",
			"departments": "roles.departments",
		},
	}
	form.Edit = ar.Command{
		ActionType: "capability", Capability: ar.Ref("identity.role.update"),
		Input: roleForm.Edit.Input, Fields: fields,
	}
	return ar.Canonical(form)
}

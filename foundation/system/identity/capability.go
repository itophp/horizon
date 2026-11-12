package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
)

// Identity administration is a capability domain like any other: the console
// button, an agent tool and an external adapter all pass through the gateway, so
// a grant change is authorized, audited and announced exactly once.
const identityVersion = "1.0.0"

// Entries is the identity administration catalog.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: analyticsDefinition(), Handler: contracts.Handler{Execute: projected(s.analyticsRead)}},
		{Definition: rolesReadDefinition(), Handler: contracts.Handler{Execute: projected(s.rolesRead)}},
		{Definition: accountsReadDefinition(), Handler: contracts.Handler{Execute: projected(s.accountsRead)}},
		{Definition: organizationsReadDefinition(), Handler: contracts.Handler{Execute: projected(s.organizationsRead)}},
		{Definition: departmentsReadDefinition(), Handler: contracts.Handler{Execute: projected(s.departmentsRead)}},
		{Definition: accountsAdminReadDefinition(), Handler: contracts.Handler{Execute: projected(s.accountsAdminRead)}},
		{Definition: accountCreateDefinition(), Handler: contracts.Handler{Execute: projected(s.createAccount)}},
		{Definition: accountUpdateDefinition(), Handler: contracts.Handler{Execute: projected(s.updateAccount)}},
		{Definition: accountDeleteDefinition(), Handler: contracts.Handler{Execute: projected(s.deleteAccount)}},
		{Definition: organizationCreateDefinition(), Handler: contracts.Handler{Execute: projected(s.createOrganization)}},
		{Definition: organizationUpdateDefinition(), Handler: contracts.Handler{Execute: projected(s.updateOrganization)}},
		{Definition: organizationDeleteDefinition(), Handler: contracts.Handler{Execute: projected(s.deleteOrganization)}},
		{Definition: departmentCreateDefinition(), Handler: contracts.Handler{Execute: projected(s.createDepartment)}},
		{Definition: departmentUpdateDefinition(), Handler: contracts.Handler{Execute: projected(s.updateDepartment)}},
		{Definition: departmentDeleteDefinition(), Handler: contracts.Handler{Execute: projected(s.deleteDepartment)}},
		{Definition: roleCreateDefinition(), Handler: contracts.Handler{Execute: projected(s.createRole)}},
		{Definition: roleUpdateDefinition(), Handler: contracts.Handler{Execute: projected(s.updateRole)}},
		{Definition: roleDeleteDefinition(), Handler: contracts.Handler{Execute: projected(s.deleteRole)}},
	}
}

// projected rewrites the Go field names of a snapshot record to the camelCase
// JSON the console and the old REST projection used. The identity record types
// come from the session snapshot contract, which has no JSON tags.
func projected(execute contracts.Executor) contracts.Executor {
	return func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
		out, err := execute(ctx, in)
		if err != nil {
			return out, err
		}
		out.Data = camelData(out.Data)
		return out, nil
	}
}

func camelData(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return value
	}
	return camelize(decoded)
}

func camelize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[lowerCamel(key)] = camelize(item)
		}
		return out
	case []any:
		for index := range typed {
			typed[index] = camelize(typed[index])
		}
		return typed
	}
	return value
}

// lowerCamel turns a Go field name into the JSON name Goa used: ID becomes id,
// OrganizationID becomes organizationId, RoleIds stays roleIds.
func lowerCamel(name string) string {
	parts := splitWords(name)
	if len(parts) == 0 {
		return name
	}
	out := strings.ToLower(parts[0])
	for _, part := range parts[1:] {
		if part == strings.ToUpper(part) {
			out += part[:1] + strings.ToLower(part[1:])
			continue
		}
		out += strings.ToUpper(part[:1]) + part[1:]
	}
	return out
}

// splitWords splits a Go identifier into words, keeping acronyms together.
func splitWords(name string) []string {
	words := []string{}
	current := []rune{}
	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}
	runes := []rune(name)
	for index, r := range runes {
		if r < 'A' || r > 'Z' {
			current = append(current, r)
			continue
		}
		previousLower := index > 0 && runes[index-1] >= 'a' && runes[index-1] <= 'z'
		nextLower := index+1 < len(runes) && runes[index+1] >= 'a' && runes[index+1] <= 'z'
		if len(current) > 0 && (previousLower || nextLower) {
			flush()
		}
		current = append(current, r)
	}
	flush()
	return words
}

func identityWrite(name, title, description, entity, permission string, events []string, input *contracts.Schema, loggable ...string) contracts.Definition {
	risk := contracts.RiskHigh
	approval := contracts.ApprovalConfirm
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: identityVersion, Title: title, Description: description,
		Domain: "identity", Entity: entity, Permission: permission,
		Risk: risk, Approval: approval,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: events, Executor: "go:" + name, Tags: []string{"identity", "write"},
		Input: input, Output: contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}),
		Loggable: loggable,
	}
}

// Runtime providers must bind read-only capabilities. The console snapshot
// endpoint is a transport projection, so these two capabilities expose the same
// authorized sections to the Gateway without weakening either permission gate.
func snapshotReadDefinition(name, title, description, entity, permission, section string, item *contracts.Schema) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: identityVersion, Title: title, Description: description,
		Domain: "identity", Entity: entity, Entities: []string{entity, "department"},
		Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		ReadOnly: true, TransientResult: true,
		Executor: "go:" + name, Tags: []string{"identity", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"items":       {Type: "array", Items: item},
			"total":       {Type: "integer"},
			"departments": {Type: "array", Items: contracts.FreeObject()},
			"catalog":     {Type: "array", Items: contracts.FreeObject()},
		}, "items", "total", "departments", "catalog"),
	}
}

var roleReadFields = []string{"id", "name", "code", "description", "status", "memberCount", "grants", "createdAt"}
var accountReadFields = []string{"id", "username", "fullName", "email", "status", "isAdmin", "organizationId", "departmentId", "roleIds", "overrides"}

func rolesReadDefinition() contracts.Definition {
	return snapshotReadDefinition("identity.roles.read", "Read roles",
		"Read authorized roles with departments and the permission catalog for grant editing.",
		"role", "roles:read", "roles", contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
			"description": {Type: "string"}, "status": {Type: "string"}, "memberCount": {Type: "integer"},
			"createdAt": {Type: "string"}, "grants": {Type: "array", Items: contracts.FreeObject()},
		}, "id", "name", "code", "status", "grants"))
}

func organizationsReadDefinition() contracts.Definition {
	return readDefinition("identity.organizations.read", "Read organizations",
		"Read the authorized organization structure for administration and Runtime providers.",
		"organization", "organizations:read", contracts.Object(map[string]*contracts.Schema{
			"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
				"kind": {Type: "string"}, "parentId": {Type: "integer"}, "status": {Type: "string"},
				"description": {Type: "string"}, "createdAt": {Type: "string"},
			}, "id", "name", "code", "kind", "parentId", "status")},
			"total": {Type: "integer"},
		}, "items", "total"))
}

func departmentsReadDefinition() contracts.Definition {
	return readDefinition("identity.departments.read", "Read departments",
		"Read authorized departments with their companies and account membership summaries.",
		"department", "departments:read", contracts.Object(map[string]*contracts.Schema{
			"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
				"organizationId": {Type: "integer"}, "parentId": {Type: "integer"},
				"manager": {Type: "string"}, "status": {Type: "string"}, "description": {Type: "string"},
				"createdAt": {Type: "string"},
			}, "id", "name", "code", "organizationId", "parentId", "status")},
			"organizations": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "kind": {Type: "string"},
			}, "id", "name", "kind")},
			"accounts": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"},
				"memberships": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
					"departmentId": {Type: "integer"},
				}, "departmentId")},
			}, "id", "memberships")},
			"total": {Type: "integer"},
		}, "items", "organizations", "total"))
}

func accountsAdminReadDefinition() contracts.Definition {
	return readDefinition("identity.accounts.admin.read", "Read account directory",
		"Read authorized accounts with memberships, structure options, roles and the permission catalog for administration.",
		"account", "accounts:read", contracts.Object(map[string]*contracts.Schema{
			"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "username": {Type: "string"}, "fullName": {Type: "string"},
				"email": {Type: "string"}, "status": {Type: "string"}, "isAdmin": {Type: "boolean"}, "createdAt": {Type: "string"},
				"organizationId": {Type: "integer"}, "departmentId": {Type: "integer"},
				"roleIds":   {Type: "array", Items: contracts.Int(0, 1<<40)},
				"overrides": {Type: "array", Items: contracts.FreeObject()},
				"memberships": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
					"organizationId": {Type: "integer"}, "departmentId": {Type: "integer"},
					"departmentName": {Type: "string"}, "roleIds": {Type: "array", Items: contracts.Int(0, 1<<40)},
					"overrides": {Type: "array", Items: contracts.FreeObject()},
				}, "organizationId", "departmentId", "roleIds", "overrides")},
			}, "id", "username", "fullName", "status", "memberships")},
			"organizations": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
				"kind": {Type: "string"}, "status": {Type: "string"},
			}, "id", "name", "kind", "status")},
			"departments": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
				"organizationId": {Type: "integer"}, "parentId": {Type: "integer"},
				"manager": {Type: "string"}, "status": {Type: "string"},
			}, "id", "name", "organizationId", "status")},
			"roles": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
				"id": {Type: "integer"}, "name": {Type: "string"}, "code": {Type: "string"},
				"status": {Type: "string"}, "grants": {Type: "array", Items: contracts.FreeObject()},
			}, "id", "name", "grants")},
			"catalog": {Type: "array", Items: contracts.FreeObject()},
			"total":   {Type: "integer"},
		}, "items", "organizations", "departments", "roles", "total"))
}

var accountAdminFields = []string{"id", "username", "fullName", "email", "status", "isAdmin", "createdAt", "organizationId", "departmentId", "roleIds", "overrides", "memberships"}
var membershipFields = []string{"organizationId", "departmentId", "departmentName", "roleIds", "overrides"}

func projectAccounts(items []any) []any {
	out := make([]any, 0, len(items))
	for _, item := range projectList(items, accountAdminFields) {
		account, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if memberships, ok := account["memberships"].([]any); ok {
			account["memberships"] = projectList(memberships, membershipFields)
		}
		out = append(out, account)
	}
	return out
}

func (s *Service) accountsAdminRead(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	snapshot, err := s.Snapshot(ctx, &genidentity.SnapshotPayload{})
	if err != nil {
		return contracts.Output{}, safe(err)
	}
	data, _ := camelData(snapshot).(map[string]any)
	accounts, _ := data["accounts"].([]any)
	organizations, _ := data["organizations"].([]any)
	departments, _ := data["departments"].([]any)
	roles, _ := data["roles"].([]any)
	projectedAccounts := projectAccounts(accounts)
	return contracts.Output{
		Data: map[string]any{
			"items":         projectedAccounts,
			"organizations": projectList(organizations, []string{"id", "name", "code", "kind", "status"}),
			"departments":   projectList(departments, []string{"id", "name", "code", "organizationId", "parentId", "manager", "status"}),
			"roles":         projectList(roles, []string{"id", "name", "code", "status", "grants"}),
			"catalog":       data["catalog"],
			"total":         len(projectedAccounts),
		},
		Summary: fmt.Sprintf("accounts=%d", len(projectedAccounts)),
	}, nil
}

func readDefinition(name, title, description, entity, permission string, output *contracts.Schema) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: identityVersion, Title: title, Description: description,
		Domain: "identity", Entity: entity, Entities: []string{entity},
		Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		ReadOnly: true, TransientResult: true,
		Executor: "go:" + name, Tags: []string{"identity", "read"},
		Input:  contracts.Object(map[string]*contracts.Schema{}),
		Output: output,
	}
}

var organizationReadFields = []string{"id", "name", "code", "kind", "parentId", "status", "description", "createdAt"}
var departmentReadFields = []string{"id", "name", "code", "organizationId", "parentId", "manager", "status", "description", "createdAt"}

func projectFields(item any, fields []string) any {
	record, ok := item.(map[string]any)
	if !ok {
		return nil
	}
	row := map[string]any{}
	for _, field := range fields {
		if value, exists := record[field]; exists {
			row[field] = value
		}
	}
	return row
}

func projectList(items []any, fields []string) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		if row := projectFields(item, fields); row != nil {
			out = append(out, row)
		}
	}
	return out
}

func (s *Service) organizationsRead(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	snapshot, err := s.Snapshot(ctx, &genidentity.SnapshotPayload{})
	if err != nil {
		return contracts.Output{}, safe(err)
	}
	data, _ := camelData(snapshot).(map[string]any)
	items, _ := data["organizations"].([]any)
	projectedItems := projectList(items, organizationReadFields)
	return contracts.Output{
		Data:    map[string]any{"items": projectedItems, "total": len(projectedItems)},
		Summary: fmt.Sprintf("organizations=%d", len(projectedItems)),
	}, nil
}

func (s *Service) departmentsRead(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	snapshot, err := s.Snapshot(ctx, &genidentity.SnapshotPayload{})
	if err != nil {
		return contracts.Output{}, safe(err)
	}
	data, _ := camelData(snapshot).(map[string]any)
	departments, _ := data["departments"].([]any)
	organizations, _ := data["organizations"].([]any)
	accounts := []any{}
	if p, err := Current(ctx); err == nil && p.Can("accounts:read") {
		if raw, ok := data["accounts"].([]any); ok {
			for _, item := range projectList(raw, []string{"id", "memberships"}) {
				account, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if memberships, ok := account["memberships"].([]any); ok {
					projected := make([]any, 0, len(memberships))
					for _, membership := range memberships {
						if row := projectFields(membership, []string{"departmentId"}); row != nil {
							projected = append(projected, row)
						}
					}
					account["memberships"] = projected
				}
				accounts = append(accounts, account)
			}
		}
	}
	projectedDepartments := projectList(departments, departmentReadFields)
	projectedOrganizations := projectList(organizations, []string{"id", "name", "kind"})
	return contracts.Output{
		Data: map[string]any{
			"items":         projectedDepartments,
			"organizations": projectedOrganizations,
			"accounts":      accounts,
			"total":         len(projectedDepartments),
		},
		Summary: fmt.Sprintf("departments=%d", len(projectedDepartments)),
	}, nil
}

func accountsReadDefinition() contracts.Definition {
	return snapshotReadDefinition("identity.accounts.read", "Read accounts",
		"Read authorized accounts with departments and the permission catalog for override editing.",
		"account", "accounts:read", "accounts", contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "username": {Type: "string"}, "fullName": {Type: "string"},
			"email": {Type: "string"}, "status": {Type: "string"}, "isAdmin": {Type: "boolean"},
			"organizationId": {Type: "integer"}, "departmentId": {Type: "integer"},
			"roleIds":   {Type: "array", Items: contracts.Int(0, 1<<40)},
			"overrides": {Type: "array", Items: contracts.FreeObject()},
		}, "id", "username", "fullName", "status", "overrides"))
}

func (s *Service) snapshotSection(ctx context.Context, section string, fields []string) (contracts.Output, error) {
	snapshot, err := s.Snapshot(ctx, &genidentity.SnapshotPayload{})
	if err != nil {
		return contracts.Output{}, safe(err)
	}
	data, _ := camelData(snapshot).(map[string]any)
	items, _ := data[section].([]any)
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	projected := make([]any, 0, len(items))
	for _, item := range items {
		if record, ok := item.(map[string]any); ok {
			row := map[string]any{}
			for field, value := range record {
				if allowed[field] {
					row[field] = value
				}
			}
			projected = append(projected, row)
		}
	}
	return contracts.Output{
		Data: map[string]any{
			"items":       projected,
			"total":       len(projected),
			"departments": data["departments"],
			"catalog":     data["catalog"],
		},
		Summary: fmt.Sprintf("%s=%d", section, len(projected)),
	}, nil
}

func (s *Service) rolesRead(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	return s.snapshotSection(ctx, "roles", roleReadFields)
}

func (s *Service) accountsRead(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
	return s.snapshotSection(ctx, "accounts", accountReadFields)
}

func grantSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"permission":      contracts.Str(1, 80),
		"effect":          contracts.Enum("allow", "deny"),
		"scope":           contracts.Str(1, 40),
		"departmentIds":   contracts.Array(contracts.Int(1, 1<<40), 0, 200),
		"organizationIds": contracts.Array(contracts.Int(1, 1<<40), 0, 200),
	}, "permission", "effect", "scope")
}

func accountSchema(requireID bool) *contracts.Schema {
	properties := map[string]*contracts.Schema{
		"username":       contracts.Str(3, 80),
		"fullName":       contracts.Str(1, 100),
		"email":          contracts.Str(0, 160),
		"status":         contracts.Enum("active", "inactive"),
		"organizationId": contracts.Int(1, 1<<40),
		"departmentId":   contracts.Int(1, 1<<40),
		"roleIds":        contracts.Array(contracts.Int(1, 1<<40), 0, 50),
		"overrides":      contracts.Array(grantSchema(), 0, 100),
		"password":       contracts.Str(0, 128),
		"departmentAccess": contracts.Object(map[string]*contracts.Schema{
			"memberships": contracts.Array(contracts.FreeObject(), 0, 49),
		}, "memberships"),
	}
	required := []string{"username", "fullName", "status", "organizationId", "departmentId", "roleIds", "overrides"}
	if requireID {
		properties["id"] = contracts.Int(1, 1<<40)
		required = append(required, "id")
	}
	return contracts.Object(properties, required...)
}

func organizationSchema(requireID bool) *contracts.Schema {
	properties := map[string]*contracts.Schema{
		"name": contracts.Str(1, 100), "code": contracts.Str(2, 40),
		"kind": contracts.Enum("enterprise", "company"), "parentId": contracts.Int(0, 1<<40),
		"status": contracts.Enum("active", "inactive"), "description": contracts.Str(0, 500),
	}
	required := []string{"name", "code", "kind", "parentId", "status", "description"}
	if requireID {
		properties["id"] = contracts.Int(1, 1<<40)
		required = append(required, "id")
	}
	return contracts.Object(properties, required...)
}

func departmentSchema(requireID bool) *contracts.Schema {
	properties := map[string]*contracts.Schema{
		"name": contracts.Str(1, 100), "code": contracts.Str(2, 40),
		"organizationId": contracts.Int(1, 1<<40), "parentId": contracts.Int(0, 1<<40),
		"manager": contracts.Str(0, 100), "status": contracts.Enum("active", "inactive"),
		"description": contracts.Str(0, 500),
	}
	required := []string{"name", "code", "organizationId", "parentId", "manager", "status", "description"}
	if requireID {
		properties["id"] = contracts.Int(1, 1<<40)
		required = append(required, "id")
	}
	return contracts.Object(properties, required...)
}

func roleSchema(requireID bool) *contracts.Schema {
	properties := map[string]*contracts.Schema{
		"name": contracts.Str(1, 100), "code": contracts.Str(2, 40),
		"description": contracts.Str(0, 500), "status": contracts.Enum("active", "inactive"),
		"grants": contracts.Array(grantSchema(), 0, 100),
	}
	required := []string{"name", "code", "description", "status", "grants"}
	if requireID {
		properties["id"] = contracts.Int(1, 1<<40)
		required = append(required, "id")
	}
	return contracts.Object(properties, required...)
}

func deleteSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<40)}, "id")
}

func accountCreateDefinition() contracts.Definition {
	definition := identityWrite("identity.account.create", "Create account", "Create one account with a primary department, roles and explicit grants.",
		"account", "accounts:create", []string{"identity.account.created"}, accountSchema(false), "username", "fullName", "organizationId", "departmentId", "roleIds")
	// One call also writes the role assignments and the department membership.
	definition.Entities = []string{"account", "role", "department"}
	return definition
}

func accountUpdateDefinition() contracts.Definition {
	return identityWrite("identity.account.update", "Update account", "Update one account, its placement, roles and explicit grants.",
		"account", "accounts:update", []string{"identity.account.updated"}, accountSchema(true), "id", "username", "fullName", "status", "departmentId", "roleIds")
}

func accountDeleteDefinition() contracts.Definition {
	return identityWrite("identity.account.delete", "Delete account", "Delete one account. High risk: automation must obtain human confirmation.",
		"account", "accounts:delete", []string{"identity.account.deleted"}, deleteSchema(), "id")
}

func organizationCreateDefinition() contracts.Definition {
	return identityWrite("identity.organization.create", "Create organization", "Create one enterprise or company node of the organization tree.",
		"organization", "organizations:create", []string{"identity.organization.created"}, organizationSchema(false), "name", "code", "kind", "parentId")
}

func organizationUpdateDefinition() contracts.Definition {
	return identityWrite("identity.organization.update", "Update organization", "Update one organization node.",
		"organization", "organizations:update", []string{"identity.organization.updated"}, organizationSchema(true), "id", "name", "code", "status")
}

func organizationDeleteDefinition() contracts.Definition {
	return identityWrite("identity.organization.delete", "Delete organization", "Delete one organization node and its empty children.",
		"organization", "organizations:delete", []string{"identity.organization.deleted"}, deleteSchema(), "id")
}

func departmentCreateDefinition() contracts.Definition {
	return identityWrite("identity.department.create", "Create department", "Create one department inside a company.",
		"department", "departments:create", []string{"identity.department.created"}, departmentSchema(false), "name", "code", "organizationId", "parentId")
}

func departmentUpdateDefinition() contracts.Definition {
	return identityWrite("identity.department.update", "Update department", "Update one department, its parent and manager.",
		"department", "departments:update", []string{"identity.department.updated"}, departmentSchema(true), "id", "name", "code", "parentId", "manager")
}

func departmentDeleteDefinition() contracts.Definition {
	return identityWrite("identity.department.delete", "Delete department", "Delete one department when it holds no accounts.",
		"department", "departments:delete", []string{"identity.department.deleted"}, deleteSchema(), "id")
}

func roleCreateDefinition() contracts.Definition {
	return identityWrite("identity.role.create", "Create role", "Create one role and its permission grants.",
		"role", "roles:create", []string{"identity.role.created"}, roleSchema(false), "name", "code")
}

func roleUpdateDefinition() contracts.Definition {
	return identityWrite("identity.role.update", "Update role", "Replace the grants of one role. High risk: the change moves real authority.",
		"role", "roles:update", []string{"identity.role.updated"}, roleSchema(true), "id", "name", "code")
}

func roleDeleteDefinition() contracts.Definition {
	return identityWrite("identity.role.delete", "Delete role", "Delete one role that no account still holds.",
		"role", "roles:delete", []string{"identity.role.deleted"}, deleteSchema(), "id")
}

func (s *Service) createAccount(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AccountInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.CreateAccount(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("account=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.account.created", AggregateType: "account", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"accountId": out.ID, "username": out.Username},
	}}}, nil
}

func (s *Service) updateAccount(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AccountInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.UpdateAccount(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("account=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.account.updated", AggregateType: "account", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"accountId": out.ID, "username": out.Username, "status": out.Status},
	}}}, nil
}

func (s *Service) deleteAccount(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DeleteInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.DeleteAccount(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("account=%d", payload.ID), Events: []contracts.Event{{
		Type: "identity.account.deleted", AggregateType: "account", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"accountId": payload.ID},
	}}}, nil
}

func (s *Service) createOrganization(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := OrganizationInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.CreateOrganization(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("organization=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.organization.created", AggregateType: "organization", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"organizationId": out.ID, "name": out.Name, "kind": out.Kind},
	}}}, nil
}

func (s *Service) updateOrganization(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := OrganizationInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.UpdateOrganization(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("organization=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.organization.updated", AggregateType: "organization", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"organizationId": out.ID, "name": out.Name, "status": out.Status},
	}}}, nil
}

func (s *Service) deleteOrganization(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DeleteInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.DeleteOrganization(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("organization=%d", payload.ID), Events: []contracts.Event{{
		Type: "identity.organization.deleted", AggregateType: "organization", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"organizationId": payload.ID},
	}}}, nil
}

func (s *Service) createDepartment(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DepartmentInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.CreateDepartment(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("department=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.department.created", AggregateType: "department", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"departmentId": out.ID, "name": out.Name, "organizationId": out.OrganizationID},
	}}}, nil
}

func (s *Service) updateDepartment(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DepartmentInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.UpdateDepartment(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("department=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.department.updated", AggregateType: "department", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"departmentId": out.ID, "name": out.Name, "status": out.Status},
	}}}, nil
}

func (s *Service) deleteDepartment(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DeleteInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.DeleteDepartment(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("department=%d", payload.ID), Events: []contracts.Event{{
		Type: "identity.department.deleted", AggregateType: "department", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"departmentId": payload.ID},
	}}}, nil
}

func (s *Service) createRole(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RoleInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.CreateRole(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("role=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.role.created", AggregateType: "role", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"roleId": out.ID, "name": out.Name, "grants": len(out.Grants)},
	}}}, nil
}

func (s *Service) updateRole(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RoleInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.UpdateRole(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("role=%d", out.ID), Events: []contracts.Event{{
		Type: "identity.role.updated", AggregateType: "role", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"roleId": out.ID, "name": out.Name, "grants": len(out.Grants)},
	}}}, nil
}

func (s *Service) deleteRole(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DeleteInput{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.DeleteRole(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("role=%d", payload.ID), Events: []contracts.Event{{
		Type: "identity.role.deleted", AggregateType: "role", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"roleId": payload.ID},
	}}}, nil
}

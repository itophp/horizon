package design

import . "goa.design/goa/v3/dsl"

var SessionCookie = APIKeySecurity("browser_session", func() { Description("HttpOnly session cookie; writes additionally require Origin and X-CSRF-Token") })
var IAMProblem = Type("IAMProblem", func() { Attribute("code", String, func() { Meta("struct:error:name") }); Required("code") })
var Grant = Type("PermissionGrant", func() {
	Attribute("permission", String, func() { MinLength(1); MaxLength(80) })
	Attribute("effect", String, func() { Enum("allow", "deny") })
	Attribute("scope", String, func() { Enum("ALL", "ORGANIZATION", "DEPARTMENT", "DEPARTMENT_TREE", "OWN", "CUSTOM") })
	Attribute("departmentIds", ArrayOf(Int64), func() { MaxLength(200) })
	Required("permission", "effect", "scope", "departmentIds")
})
var Org = Type("OrganizationRecord", func() {
	Attribute("id", Int64)
	Attribute("name", String)
	Attribute("code", String)
	Attribute("kind", String)
	Attribute("parentId", Int64)
	Attribute("status", String)
	Attribute("description", String)
	Attribute("createdAt", String)
	Required("id", "name", "code", "kind", "parentId", "status", "description", "createdAt")
})
var Dept = Type("DepartmentRecord", func() {
	Attribute("id", Int64)
	Attribute("name", String)
	Attribute("code", String)
	Attribute("organizationId", Int64)
	Attribute("parentId", Int64)
	Attribute("manager", String)
	Attribute("status", String)
	Attribute("description", String)
	Attribute("createdAt", String)
	Required("id", "name", "code", "organizationId", "parentId", "manager", "status", "description", "createdAt")
})
var Role = Type("RoleRecord", func() {
	Attribute("id", Int64)
	Attribute("name", String)
	Attribute("code", String)
	Attribute("description", String)
	Attribute("status", String)
	Attribute("grants", ArrayOf(Grant))
	Attribute("memberCount", Int64)
	Attribute("createdAt", String)
	Required("id", "name", "code", "description", "status", "grants", "memberCount", "createdAt")
})
var DepartmentMembership = Type("DepartmentMembership", func() {
	Attribute("organizationId", Int64, func() { Minimum(1) })
	Attribute("departmentId", Int64, func() { Minimum(1) })
	Attribute("departmentName", String)
	Attribute("roleIds", ArrayOf(Int64), func() { MaxLength(50) })
	Attribute("overrides", ArrayOf(Grant), func() { MaxLength(100) })
	Required("organizationId", "departmentId", "roleIds", "overrides")
})
var DepartmentAccess = Type("DepartmentAccess", func() {
	Description("Explicitly replace additional memberships; the primary department remains in account fields. Omission preserves existing additional memberships.")
	Attribute("memberships", ArrayOf(DepartmentMembership), func() { MaxLength(49) })
	Required("memberships")
})
var Account = Type("AccountRecord", func() {
	Attribute("id", Int64)
	Attribute("username", String)
	Attribute("fullName", String)
	Attribute("email", String)
	Attribute("status", String)
	Attribute("organizationId", Int64)
	Attribute("departmentId", Int64)
	Attribute("roleIds", ArrayOf(Int64))
	Attribute("overrides", ArrayOf(Grant))
	Attribute("memberships", ArrayOf(DepartmentMembership))
	Attribute("isAdmin", Boolean)
	Attribute("createdAt", String)
	Attribute("lastLogin", String)
	Required("id", "username", "fullName", "email", "status", "organizationId", "departmentId", "roleIds", "overrides", "isAdmin", "createdAt", "lastLogin", "memberships")
})
var SessionProfile = Type("SessionProfile", func() {
	Attribute("account", Account)
	Attribute("csrfToken", String)
	Attribute("permissions", ArrayOf(String))
	Attribute("tenantId", String)
	Attribute("activeDepartmentId", Int64)
	Required("account", "csrfToken", "permissions", "tenantId", "activeDepartmentId")
})
var LoginResult = Type("LoginResult", func() {
	Attribute("profile", SessionProfile)
	Attribute("cookie", String)
	Required("profile", "cookie")
})
var LogoutResult = Type("LogoutResult", func() { Attribute("cookie", String); Required("cookie") })
var Snapshot = Type("AdminSnapshot", func() {
	Attribute("accounts", ArrayOf(Account))
	Attribute("organizations", ArrayOf(Org))
	Attribute("departments", ArrayOf(Dept))
	Attribute("roles", ArrayOf(Role))
	Attribute("permissions", ArrayOf(String))
	Attribute("catalog", ArrayOf(PermissionDefinition))
	Required("accounts", "organizations", "departments", "roles", "permissions", "catalog")
})

func cookiePayload() { APIKey("browser_session", "cookie", String); Required("cookie") }
func orgFields() {
	Attribute("name", String, func() { MinLength(1); MaxLength(100) })
	Attribute("code", String, func() { Pattern("^[A-Za-z0-9_-]{2,40}$") })
	Attribute("kind", String, func() { Enum("enterprise", "company") })
	Attribute("parentId", Int64, func() { Minimum(0) })
	Attribute("status", String, func() { Enum("active", "inactive") })
	Attribute("description", String, func() { MaxLength(500) })
	Required("name", "code", "kind", "parentId", "status", "description")
}
func deptFields() {
	Attribute("name", String, func() { MinLength(1); MaxLength(100) })
	Attribute("code", String, func() { Pattern("^[A-Za-z0-9_-]{2,40}$") })
	Attribute("organizationId", Int64, func() { Minimum(1) })
	Attribute("parentId", Int64, func() { Minimum(0) })
	Attribute("manager", String, func() { MaxLength(100) })
	Attribute("status", String, func() { Enum("active", "inactive") })
	Attribute("description", String, func() { MaxLength(500) })
	Required("name", "code", "organizationId", "parentId", "manager", "status", "description")
}
func roleFields() {
	Attribute("name", String, func() { MinLength(1); MaxLength(100) })
	Attribute("code", String, func() { Pattern("^[A-Za-z0-9_-]{2,40}$") })
	Attribute("description", String, func() { MaxLength(500) })
	Attribute("status", String, func() { Enum("active", "inactive") })
	Attribute("grants", ArrayOf(Grant), func() { MaxLength(100) })
	Required("name", "code", "description", "status", "grants")
}
func accountFields() {
	Attribute("departmentAccess", DepartmentAccess)
	Attribute("username", String, func() { Pattern("^[A-Za-z0-9_.@-]{3,80}$") })
	Attribute("fullName", String, func() { MinLength(1); MaxLength(100) })
	Attribute("email", String, func() { MaxLength(160) })
	Attribute("status", String, func() { Enum("active", "inactive") })
	Attribute("organizationId", Int64, func() { Minimum(1) })
	Attribute("departmentId", Int64, func() { Minimum(1) })
	Attribute("roleIds", ArrayOf(Int64), func() { MaxLength(50) })
	Attribute("overrides", ArrayOf(Grant), func() { MaxLength(100) })
	Attribute("password", String, func() { MaxLength(128) })
	Required("username", "fullName", "email", "status", "organizationId", "departmentId", "roleIds", "overrides", "password")
}

var _ = Service("identity", func() {
	temporaryMethods()
	Method("switch_department", func() {
		Payload(func() {
			cookiePayload()
			Attribute("departmentId", Int64, func() { Minimum(1) })
			Required("departmentId")
		})
		Result(SessionProfile)
		Error("unauthenticated", IAMProblem)
		Error("forbidden", IAMProblem)
		HTTP(func() {
			Header("cookie:Cookie")
			POST("/api/auth/department")
			Response(StatusOK)
			Response("unauthenticated", StatusUnauthorized)
			Response("forbidden", StatusForbidden)
		})
	})
	Description("Persistent account, organization, department and access administration")
	Security(SessionCookie)
	Error("unavailable", IAMProblem)
	Error("internal", IAMProblem)
	HTTP(func() {
		Response("unavailable", StatusServiceUnavailable)
		Response("internal", StatusInternalServerError)
	})
	Method("login", func() {
		NoSecurity()
		Error("rate_limited", IAMProblem)
		Error("unauthenticated", IAMProblem)
		Error("invalid", IAMProblem)
		Payload(func() {
			Attribute("username", String, func() { MinLength(3); MaxLength(80) })
			Attribute("password", String, func() { MinLength(1); MaxLength(128) })
			Required("username", "password")
		})
		Result(LoginResult)
		HTTP(func() {
			POST("/api/auth/login")
			Response("rate_limited", StatusTooManyRequests)
			Response(StatusOK, func() { Header("cookie:Set-Cookie") })
			Response("unauthenticated", StatusUnauthorized)
			Response("invalid", StatusBadRequest)
		})
	})
	Method("me", func() {
		Error("unauthenticated", IAMProblem)
		Payload(cookiePayload)
		Result(SessionProfile)
		HTTP(func() {
			Header("cookie:Cookie")
			GET("/api/auth/me")
			Response(StatusOK)
			Response("unauthenticated", StatusUnauthorized)
		})
	})
	Method("logout", func() {
		Error("unauthenticated", IAMProblem)
		Payload(cookiePayload)
		Result(LogoutResult)
		HTTP(func() {
			Header("cookie:Cookie")
			POST("/api/auth/logout")
			Response(StatusOK, func() { Header("cookie:Set-Cookie") })
			Response("unauthenticated", StatusUnauthorized)
		})
	})
	Method("snapshot", func() {
		Error("unauthenticated", IAMProblem)
		Payload(cookiePayload)
		Result(Snapshot)
		HTTP(func() {
			Header("cookie:Cookie")
			GET("/api/admin/snapshot")
			Response(StatusOK)
			Response("unauthenticated", StatusUnauthorized)
		})
	})
	for _, resource := range []struct {
		name, path string
		fields     func()
		result     any
	}{{"account", "accounts", accountFields, Account}, {"organization", "organizations", orgFields, Org}, {"department", "departments", deptFields, Dept}, {"role", "roles", roleFields, Role}} {
		Method("create_"+resource.name, func() {
			for _, code := range []string{"unauthenticated", "forbidden", "invalid", "conflict"} {
				Error(code, IAMProblem)
			}
			Payload(func() { cookiePayload(); resource.fields() })
			Result(resource.result)
			HTTP(func() {
				Header("cookie:Cookie")
				POST("/api/admin/" + resource.path)
				Response(StatusCreated)
				Response("unauthenticated", StatusUnauthorized)
				Response("forbidden", StatusForbidden)
				Response("invalid", StatusBadRequest)
				Response("conflict", StatusConflict)
			})
		})
		Method("update_"+resource.name, func() {
			for _, code := range []string{"unauthenticated", "forbidden", "invalid", "conflict", "not_found"} {
				Error(code, IAMProblem)
			}
			Payload(func() {
				cookiePayload()
				Attribute("id", Int64, func() { Minimum(1) })
				Required("id")
				resource.fields()
			})
			Result(resource.result)
			HTTP(func() {
				Header("cookie:Cookie")
				PUT("/api/admin/" + resource.path + "/{id}")
				Response(StatusOK)
				Response("unauthenticated", StatusUnauthorized)
				Response("forbidden", StatusForbidden)
				Response("invalid", StatusBadRequest)
				Response("conflict", StatusConflict)
				Response("not_found", StatusNotFound)
			})
		})
		Method("delete_"+resource.name, func() {
			for _, code := range []string{"unauthenticated", "forbidden", "conflict", "not_found"} {
				Error(code, IAMProblem)
			}
			Payload(func() { cookiePayload(); Attribute("id", Int64, func() { Minimum(1) }); Required("id") })
			HTTP(func() {
				Header("cookie:Cookie")
				DELETE("/api/admin/" + resource.path + "/{id}")
				Response(StatusNoContent)
				Response("unauthenticated", StatusUnauthorized)
				Response("forbidden", StatusForbidden)
				Response("conflict", StatusConflict)
				Response("not_found", StatusNotFound)
			})
		})
	}
})

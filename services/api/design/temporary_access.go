package design

import . "goa.design/goa/v3/dsl"

var PermissionDefinition = Type("PermissionDefinition", func() {
	Attribute("code", String)
	Attribute("module", String)
	Attribute("function", String)
	Attribute("action", String)
	Attribute("temporaryEligible", Boolean)
	Attribute("scopeConfigurable", Boolean)
	Required("code", "module", "function", "action", "temporaryEligible", "scopeConfigurable")
})
var ScopeDepartmentChoice = Type("ScopeDepartmentChoice", func() {
	Attribute("id", Int64)
	Attribute("name", String)
	Attribute("organizationId", Int64)
	Attribute("organizationName", String)
	Required("id", "name", "organizationId", "organizationName")
})
var TemporaryAccessRecord = Type("TemporaryAccessRecord", func() {
	Attribute("id", Int64)
	Attribute("applicantId", Int64)
	Attribute("applicantName", String)
	Attribute("departmentId", Int64)
	Attribute("departmentName", String)
	Attribute("requestedGrants", ArrayOf(Grant))
	Attribute("requestedMinutes", Int)
	Attribute("reason", String)
	Attribute("status", String)
	Attribute("approvedGrants", ArrayOf(Grant))
	Attribute("approvedMinutes", Int)
	Attribute("reviewerId", Int64)
	Attribute("reviewerName", String)
	Attribute("reviewNote", String)
	Attribute("createdAt", String)
	Attribute("reviewedAt", String)
	Attribute("expiresAt", String)
	Required("departmentId", "departmentName", "id", "applicantId", "applicantName", "requestedGrants", "requestedMinutes", "reason", "status", "approvedGrants", "approvedMinutes", "reviewerId", "reviewerName", "reviewNote", "createdAt", "reviewedAt", "expiresAt")
})
var TemporaryAccessWorkspace = Type("TemporaryAccessWorkspace", func() {
	Attribute("requests", ArrayOf(TemporaryAccessRecord))
	Attribute("catalog", ArrayOf(PermissionDefinition))
	Attribute("departments", ArrayOf(ScopeDepartmentChoice))
	Attribute("effectiveGrants", ArrayOf(Grant))
	Attribute("canReview", Boolean)
	Required("requests", "catalog", "departments", "effectiveGrants", "canReview")
})

func temporaryDuration() {
	Attribute("minutes", Int, func() { Enum(30, 60, 120, 240, 480, 1440, 4320, 10080) })
	Required("minutes")
}
func temporaryMethods() {
	for _, op := range []string{"temporary_access", "request_temporary_access", "review_temporary_access", "cancel_temporary_access", "revoke_temporary_access"} {
		Method(op, func() {
			for _, code := range []string{"unauthenticated", "forbidden", "invalid", "not_found", "conflict"} {
				Error(code, IAMProblem)
			}
			Payload(func() {
				cookiePayload()
				if op == "request_temporary_access" || op == "review_temporary_access" {
					Attribute("grants", ArrayOf(Grant), func() { MaxLength(30) })
					Required("grants")
					temporaryDuration()
				}
				if op == "request_temporary_access" {
					Attribute("reason", String, func() { MinLength(5); MaxLength(500) })
					Required("reason")
				}
				if op == "review_temporary_access" {
					Attribute("decision", String, func() { Enum("approve", "reject") })
					Attribute("note", String, func() { MaxLength(500) })
					Required("decision", "note")
				}
				if op == "review_temporary_access" || op == "cancel_temporary_access" || op == "revoke_temporary_access" {
					Attribute("id", Int64, func() { Minimum(1) })
					Required("id")
				}
			})
			if op == "temporary_access" {
				Result(TemporaryAccessWorkspace)
			} else {
				Result(TemporaryAccessRecord)
			}
			HTTP(func() {
				Header("cookie:Cookie")
				switch op {
				case "temporary_access":
					GET("/api/admin/temporary-access")
				case "request_temporary_access":
					POST("/api/admin/temporary-access")
				case "review_temporary_access":
					POST("/api/admin/temporary-access/{id}/review")
				case "cancel_temporary_access":
					POST("/api/admin/temporary-access/{id}/cancel")
				case "revoke_temporary_access":
					POST("/api/admin/temporary-access/{id}/revoke")
				}
				Response(StatusOK)
				Response("unauthenticated", StatusUnauthorized)
				Response("forbidden", StatusForbidden)
				Response("invalid", StatusBadRequest)
				Response("not_found", StatusNotFound)
				Response("conflict", StatusConflict)
			})
		})
	}
}

package design

import . "goa.design/goa/v3/dsl"

// The AI service is the task face of the platform: start a goal driven task,
// read its trace, and check runtime quality. Business execution and approvals
// live in the capability service; this service never exposes a second path.

var AIPage = Type("AIPage", func() {
	Description("Current page context. It is validated against the route and entity catalog and treated as data, not instructions.")
	Attribute("route", String, func() { MaxLength(120) })
	Attribute("entity", String, func() { MaxLength(40) })
	Attribute("entityId", Int64, func() { Minimum(0) })
})

var AIStatus = Type("AIStatus", func() {
	Description("Runtime availability and quality snapshot. Credentials and endpoints are never returned.")
	Attribute("enabled", Boolean)
	Attribute("configured", Boolean)
	Attribute("model", String)
	Attribute("registryVersion", String)
	Attribute("capabilityCount", Int)
	Attribute("tasksTotal", Int64)
	Attribute("tasksCompleted", Int64)
	Attribute("tasksFailed", Int64)
	Attribute("completionRate", Float64)
	Attribute("averageDurationMs", Int64)
	Attribute("promptTokens", Int64)
	Attribute("completionTokens", Int64)
	Required("enabled", "configured", "model", "registryVersion", "capabilityCount", "tasksTotal", "tasksCompleted", "tasksFailed", "completionRate", "averageDurationMs", "promptTokens", "completionTokens")
})

var AIRun = Type("AIRun", func() {
	Description("One AI task with the business calls it caused. Business invocation state stays independent.")
	Attribute("id", Int64)
	Attribute("question", String)
	Attribute("answer", String)
	Attribute("status", String, func() { Enum("running", "completed", "awaiting_approval", "failed", "cancelled") })
	Attribute("agent", String)
	Attribute("model", String)
	Attribute("instructionVersion", String)
	Attribute("origin", String, func() { Enum("human-ui", "agent", "workflow", "api", "event", "approval") })
	Attribute("errorCode", String)
	Attribute("toolCalls", Int)
	Attribute("promptTokens", Int)
	Attribute("completionTokens", Int)
	Attribute("durationMs", Int)
	Attribute("traceId", String)
	Attribute("correlationId", String)
	Attribute("causationId", String)
	Attribute("triggerDepth", Int)
	Attribute("triggerPolicy", String)
	Attribute("eventId", String)
	Attribute("createdAt", String)
	Attribute("finishedAt", String)
	Attribute("invocations", ArrayOf(CapabilityInvocation))
	Required("id", "question", "answer", "status", "agent", "model", "instructionVersion", "origin", "errorCode", "toolCalls", "promptTokens", "completionTokens", "durationMs", "traceId", "correlationId", "causationId", "triggerDepth", "triggerPolicy", "eventId", "createdAt", "finishedAt", "invocations")
})

var AIConversation = Type("AIConversation", func() {
	Description("One conversation: the tasks recorded under a session, so the workbench can reopen and continue it. An archived conversation is kept and can be restored.")
	Attribute("sessionId", String)
	Attribute("title", String)
	Attribute("updatedAt", String)
	Attribute("messageCount", Int)
	Attribute("archived", Boolean)
	Required("sessionId", "title", "updatedAt", "messageCount", "archived")
})

var _ = Service("ai", func() {
	Description("Agent workbench: start a task, read its trace and inspect runtime quality.")
	Security(SessionCookie)
	Error("unauthenticated", IAMProblem)
	Error("forbidden", IAMProblem)
	Error("invalid", IAMProblem)
	Error("not_found", IAMProblem)
	Error("unavailable", IAMProblem)
	Error("internal", IAMProblem)
	HTTP(func() {
		Header("cookie:Cookie")
		Response("unauthenticated", StatusUnauthorized)
		Response("forbidden", StatusForbidden)
		Response("invalid", StatusBadRequest)
		Response("not_found", StatusNotFound)
		Response("unavailable", StatusServiceUnavailable)
		Response("internal", StatusInternalServerError)
	})
	Method("status", func() {
		Description("Report whether the agent is enabled, the catalog size and the tenant quality snapshot.")
		Payload(func() { cookiePayload() })
		Result(AIStatus)
		HTTP(func() { GET("/api/ai/status"); Response(StatusOK) })
	})
	Method("run", func() {
		Description("Execute one agent task. Business actions inside it go through the capability gateway.")
		Payload(func() {
			cookiePayload()
			Attribute("question", String, func() { MinLength(1); MaxLength(4000) })
			Attribute("sessionId", String, func() { MaxLength(80) })
			Attribute("page", AIPage)
			Required("question")
		})
		Result(AIRun)
		HTTP(func() { POST("/api/ai/runs"); Response(StatusCreated) })
	})
	Method("runs", func() {
		Description("List the caller's own tasks, newest first. A session id narrows it to one conversation.")
		Payload(func() {
			cookiePayload()
			Attribute("limit", Int, func() { Minimum(1); Maximum(100); Default(30) })
			Attribute("sessionId", String, func() { MaxLength(80) })
		})
		Result(ArrayOf(AIRun))
		HTTP(func() {
			GET("/api/ai/runs")
			Param("limit")
			Param("sessionId")
			Response(StatusOK)
		})
	})
	Method("conversations", func() {
		Description("List the caller's conversations, newest first. The workbench uses it as its task list.")
		Payload(func() {
			cookiePayload()
			Attribute("limit", Int, func() { Minimum(1); Maximum(100); Default(30) })
		})
		Result(ArrayOf(AIConversation))
		HTTP(func() {
			GET("/api/ai/conversations")
			Param("limit")
			Response(StatusOK)
		})
	})
	Method("runDetail", func() {
		Description("Read one task with the capability calls it caused.")
		Payload(func() { cookiePayload(); resourceId() })
		Result(AIRun)
		HTTP(func() { GET("/api/ai/runs/{id}"); Response(StatusOK) })
	})
	Method("updateConversation", func() {
		Description("Rename or archive one conversation of the caller. A renamed title replaces the derived first question; an archived conversation stays readable and is restored with archived=false.")
		Payload(func() {
			cookiePayload()
			Attribute("sessionId", String, func() {
				Description("Session the tasks of the conversation belong to.")
				MinLength(1)
				MaxLength(80)
			})
			Attribute("title", String, func() {
				Description("New title. Omitted leaves the current title untouched.")
				MinLength(1)
				MaxLength(200)
			})
			Attribute("archived", Boolean, func() {
				Description("True archives the conversation, false restores it. Omitted leaves the current state untouched.")
			})
			Required("sessionId")
		})
		Result(AIConversation)
		HTTP(func() {
			PATCH("/api/ai/conversations/{sessionId}")
			Response(StatusOK)
		})
	})
})

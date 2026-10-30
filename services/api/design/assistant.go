package design

import . "goa.design/goa/v3/dsl"

// The embedded assistant is a browser-side agent: page tools stay in the page and
// the model is reached only through this authenticated endpoint. Model, endpoint
// and credential are deployment settings, never browser input.

var AssistantMessage = Type("AssistantMessage", func() {
	Description("One turn of the assistant conversation. Tool output is attributed to its call id.")
	Attribute("role", String, func() { Enum("system", "user", "assistant", "tool") })
	Attribute("content", String, func() { MaxLength(8000) })
	Attribute("toolCallId", String, func() { MaxLength(120) })
	Attribute("toolCalls", ArrayOf(AssistantToolCall), func() { MaxLength(10) })
	Required("role", "content")
})

var AssistantTool = Type("AssistantTool", func() {
	Description("Page tool definition supplied by the browser. The model may request it; the page executes it.")
	Attribute("name", String, func() { MinLength(1); MaxLength(64) })
	Attribute("description", String, func() { MaxLength(2000) })
	Attribute("parameters", Any)
	Required("name", "description", "parameters")
})

var AssistantToolCall = Type("AssistantToolCall", func() {
	Attribute("id", String)
	Attribute("name", String)
	Attribute("arguments", String)
	Required("id", "name", "arguments")
})

var AssistantReply = Type("AssistantReply", func() {
	Attribute("content", String)
	Attribute("toolCalls", ArrayOf(AssistantToolCall))
	Attribute("model", String)
	Required("content", "toolCalls", "model")
})

var AssistantStatus = Type("AssistantStatus", func() {
	Description("Availability projection for the launcher. Credentials and endpoints are never returned.")
	Attribute("enabled", Boolean)
	Attribute("configured", Boolean)
	Attribute("model", String)
	Required("enabled", "configured", "model")
})

var _ = Service("assistant", func() {
	Description("Embedded assistant gateway. Resolves the deployment model configuration on the server; the browser never receives the credential.")
	Security(SessionCookie)
	Error("unauthenticated", IAMProblem)
	Error("forbidden", IAMProblem)
	Error("invalid", IAMProblem)
	Error("unavailable", IAMProblem)
	Error("internal", IAMProblem)
	HTTP(func() {
		Header("cookie:Cookie")
		Response("unauthenticated", StatusUnauthorized)
		Response("forbidden", StatusForbidden)
		Response("invalid", StatusBadRequest)
		Response("unavailable", StatusServiceUnavailable)
		Response("internal", StatusInternalServerError)
	})
	Method("status", func() {
		Description("Report whether the assistant is enabled for this workspace and which model is configured.")
		Payload(func() { cookiePayload() })
		Result(AssistantStatus)
		HTTP(func() { GET("/api/assistant/status"); Response(StatusOK) })
	})
	Method("chat", func() {
		Description("Send the conversation and the currently registered page tools; returns assistant text plus any requested tool calls.")
		Payload(func() {
			cookiePayload()
			Attribute("messages", ArrayOf(AssistantMessage), func() { MinLength(1); MaxLength(60) })
			Attribute("tools", ArrayOf(AssistantTool), func() { MaxLength(40) })
			Required("messages")
		})
		Result(AssistantReply)
		HTTP(func() { POST("/api/assistant/chat"); Response(StatusOK) })
	})
})

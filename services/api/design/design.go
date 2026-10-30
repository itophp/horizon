package design

import . "goa.design/goa/v3/dsl"

var _ = API("baseline", func() {
	Title("Baseline API")
	Description("Application infrastructure and technical demonstrations")
	Server("api", func() { Host("local", func() { URI("http://127.0.0.1:8080") }) })
})
var Record = Type("TechnicalRecord", func() {
	Attribute("id", Int64)
	Attribute("label", String)
	Required("id", "label")
})
var _ = Service("system", func() {
	Method("health", func() {
		Result(func() { Attribute("status", String); Required("status") })
		HTTP(func() { GET("/api/health"); Response(StatusOK) })
	})
	Method("echo", func() {
		Payload(func() { Attribute("name", String, func() { MinLength(1); MaxLength(80) }); Required("name") })
		Result(func() {
			Attribute("name", String)
			Attribute("timestamp", String, func() { Format(FormatDateTime) })
			Required("name", "timestamp")
		})
		HTTP(func() { GET("/api/echo"); Param("name"); Response(StatusOK) })
	})
})
var Bearer = APIKeySecurity("bearer", func() { Description("Authorization header containing Bearer followed by a verified credential") })
var AccessError = Type("AccessError", func() { Attribute("code", String, func() { Meta("struct:error:name") }); Required("code") })
var _ = Service("demo", func() {
	Description("Protected technical-only demo. Bearer identity is verified by transport middleware.")
	Security(Bearer)
	Error("unauthenticated", AccessError)
	Error("forbidden", AccessError)
	Error("unavailable")
	Error("internal")
	HTTP(func() {
		Response("unauthenticated", StatusUnauthorized)
		Response("forbidden", StatusForbidden)
		Header("token:Authorization")
		Response("unavailable", StatusServiceUnavailable)
		Response("internal", StatusInternalServerError)
	})
	Method("list", func() {
		Payload(func() { APIKey("bearer", "token", String); Required("token") })
		Result(ArrayOf(Record))
		HTTP(func() { GET("/api/demo/records"); Response(StatusOK) })
	})
	Method("get", func() {
		Error("not_found")
		Payload(func() {
			APIKey("bearer", "token", String)
			Required("token")
			Attribute("id", Int64, func() { Minimum(1) })
			Required("id")
		})
		Result(Record)
		HTTP(func() { GET("/api/demo/records/{id}"); Response("not_found", StatusNotFound); Response(StatusOK) })
	})
	Method("update", func() {
		Error("not_found")
		Payload(func() {
			APIKey("bearer", "token", String)
			Required("token")
			Attribute("id", Int64, func() { Minimum(1) })
			Attribute("label", String, func() { MinLength(1); MaxLength(80) })
			Required("id", "label")
		})
		Result(Record)
		HTTP(func() { PATCH("/api/demo/records/{id}"); Response("not_found", StatusNotFound); Response(StatusOK) })
	})
	Method("delete", func() {
		Error("not_found")
		Payload(func() {
			APIKey("bearer", "token", String)
			Required("token")
			Attribute("id", Int64, func() { Minimum(1) })
			Required("id")
		})
		HTTP(func() {
			DELETE("/api/demo/records/{id}")
			Response("not_found", StatusNotFound)
			Response(StatusNoContent)
		})
	})
})

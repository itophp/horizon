package design

import . "goa.design/goa/v3/dsl"

var _ = API("baseline", func() {
	Title("Baseline API")
	Description("Application infrastructure and technical demonstrations")
	Server("api", func() { Host("local", func() { URI("http://127.0.0.1:8080") }) })
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

// resourceId is the shared payload fragment for methods addressed by primary key.
func resourceId() { Attribute("id", Int64, func() { Minimum(1) }); Required("id") }

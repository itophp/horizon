package design

import . "goa.design/goa/v3/dsl"

var ConfigEntry = Type("ConfigEntry", func() {
	Attribute("key", String)
	Attribute("value", String)
	Attribute("source", String)
	Attribute("overridden", Boolean)
	Attribute("configured", Boolean)
	Required("key", "value", "source", "overridden", "configured")
})
var EffectiveConfiguration = Type("EffectiveConfiguration", func() {
	Attribute("entries", ArrayOf(ConfigEntry))
	Attribute("revision", String)
	Required("entries", "revision")
})
var FileContent = Type("FileContent", func() {
	Attribute("name", String)
	Attribute("mime", String)
	Attribute("content", Bytes)
	Required("name", "mime", "content")
})

// Public branding exposes only a configured deployment workspace's allowlisted display keys.
var _ = Service("branding", func() {
	Error("internal", IAMProblem)
	HTTP(func() { Response("internal", StatusInternalServerError) })
	Method("get", func() {
		NoSecurity()
		Result(EffectiveConfiguration)
		HTTP(func() { GET("/api/public/branding"); Response(StatusOK) })
	})
	Method("logo", func() {
		NoSecurity()
		Error("not_found", IAMProblem)
		Result(FileContent)
		HTTP(func() { GET("/api/public/branding/logo"); Response(StatusOK); Response("not_found", StatusNotFound) })
	})
})

package governance

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Definition struct {
	Key, Group, Kind, Default string
	Scopes, Choices           []string
	Sensitive, Secret         bool
	Min, Max                  int64
	UI                        bool
}

var Registry = buildRegistry()

func buildRegistry() []Definition {
	workspace := []string{"workspace"}
	branding := []string{"workspace"}
	display := []string{"workspace", "account"}
	out := []Definition{}
	add := func(key, group, kind, def string, scopes []string, sensitive, secret, ui bool, min, max int64, choices ...string) {
		out = append(out, Definition{key, group, kind, def, scopes, choices, sensitive, secret, min, max, ui})
	}
	add("branding.name", "branding", "string", `"Horizon"`, branding, false, false, true, 1, 100)
	add("branding.tagline", "branding", "string", `"Administration workspace"`, branding, false, false, true, 0, 200)
	add("branding.loginTitle", "branding", "string", `""`, branding, false, false, true, 0, 200)
	add("branding.loginDescription", "branding", "string", `""`, branding, false, false, true, 0, 500)
	add("branding.footer", "branding", "string", `""`, branding, false, false, true, 0, 300)
	add("branding.contact", "branding", "string", `""`, branding, false, false, true, 0, 200)
	add("branding.notice", "branding", "string", `""`, branding, false, false, true, 0, 500)
	add("branding.logo", "branding", "integer", `0`, branding, false, false, true, 0, 1<<53-1)
	add("display.language", "display", "enum", `"en"`, display, false, false, true, 0, 0, "en", "zh-CN")
	add("display.theme", "display", "enum", `"dark"`, display, false, false, true, 0, 0, "light", "dark")
	add("display.timezone", "display", "string", `"Asia/Shanghai"`, display, false, false, true, 1, 100)
	add("display.dateStyle", "display", "enum", `"short"`, display, false, false, true, 0, 0, "short", "medium", "long")
	add("display.hour24", "display", "boolean", `true`, display, false, false, true, 0, 0)
	add("display.grouping", "display", "boolean", `true`, display, false, false, true, 0, 0)
	add("display.pageSize", "display", "integer", `8`, display, false, false, true, 1, 100)
	add("security.passwordMin", "security", "integer", `12`, workspace, true, false, true, 12, 128)
	add("security.requireUpper", "security", "boolean", `false`, workspace, true, false, false, 0, 0)
	add("security.requireLower", "security", "boolean", `false`, workspace, true, false, false, 0, 0)
	add("security.requireDigit", "security", "boolean", `false`, workspace, true, false, false, 0, 0)
	add("security.requireSymbol", "security", "boolean", `false`, workspace, true, false, false, 0, 0)
	add("security.lockAttempts", "security", "integer", `5`, workspace, true, false, false, 3, 20)
	add("security.lockMinutes", "security", "integer", `15`, workspace, true, false, false, 5, 1440)
	add("security.sessionMinutes", "security", "integer", `480`, workspace, true, false, false, 15, 1440)
	add("security.idleMinutes", "security", "integer", `0`, workspace, true, false, false, 0, 1440)
	add("security.initialStatus", "security", "enum", `"active"`, workspace, true, false, true, 0, 0, "active", "inactive")
	add("temporary.allowedMinutes", "temporary", "minutes", `[30,60,120,240,480,1440,4320,10080]`, workspace, true, false, true, 0, 0)
	add("temporary.maximumMinutes", "temporary", "integer", `10080`, workspace, true, false, true, 30, 10080)
	add("temporary.reasonMin", "temporary", "integer", `5`, workspace, false, false, true, 5, 500)
	smtp := []string{"workspace"}
	add("smtp.enabled", "smtp", "boolean", `false`, smtp, true, false, false, 0, 0)
	add("smtp.host", "smtp", "string", `""`, smtp, true, false, false, 0, 253)
	add("smtp.port", "smtp", "integer", `587`, smtp, true, false, false, 1, 65535)
	add("smtp.tls", "smtp", "enum", `"starttls"`, smtp, true, false, false, 0, 0, "starttls", "tls")
	add("smtp.username", "smtp", "string", `""`, smtp, true, false, false, 0, 200)
	add("smtp.password", "smtp", "secret", `""`, smtp, true, true, false, 0, 4096)
	add("smtp.from", "smtp", "string", `""`, smtp, true, false, false, 0, 254)
	add("notifications.inApp", "notifications", "boolean", `true`, display, false, false, true, 0, 0)
	add("notifications.email", "notifications", "boolean", `true`, display, false, false, true, 0, 0)
	for _, event := range []string{"requested", "approved", "rejected", "revoked"} {
		for _, lang := range []string{"en", "zh-CN"} {
			subject := `"Access request {{requestId}}: {{status}}"`
			body := `"{{applicant}} / {{department}}: {{status}}. {{link}}"`
			if lang == "zh-CN" {
				subject = `"权限申请 {{requestId}}：{{status}}"`
				body = `"{{applicant}} / {{department}}：{{status}}。{{link}}"`
			}
			add("templates."+event+"."+lang+".subject", "templates", "string", subject, smtp, false, false, false, 1, 200)
			add("templates."+event+"."+lang+".body", "templates", "string", body, smtp, false, false, false, 1, 5000)
		}
	}
	add("storage.driver", "storage", "enum", `"local"`, workspace, true, false, false, 0, 0, "local")
	add("storage.directory", "storage", "string", `"uploads"`, workspace, true, false, false, 1, 200)
	add("storage.baseUrl", "storage", "string", `""`, workspace, true, false, false, 0, 500)
	limits := []string{"workspace"}
	add("files.maxBytes", "files", "integer", `20971520`, limits, false, false, false, 1024, 20971520)
	add("files.logoMaxBytes", "files", "integer", `2097152`, limits, false, false, false, 1024, 2097152)
	add("files.extensions", "files", "set", `["png","jpg","jpeg","webp","pdf","txt","csv","docx","xlsx"]`, limits, false, false, false, 0, 0, "png", "jpg", "jpeg", "webp", "pdf", "txt", "csv", "docx", "xlsx")
	return out
}
func FindDefinition(key string) *Definition {
	for i := range Registry {
		if Registry[i].Key == key {
			return &Registry[i]
		}
	}
	return nil
}
func Has(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

var placeholder = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func (d Definition) Validate(raw string) error {
	if !json.Valid([]byte(raw)) || strings.TrimSpace(raw) == "null" {
		return Problem("invalid")
	}
	switch d.Kind {
	case "string", "enum", "secret":
		var v string
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return Problem("invalid")
		}
		if len([]rune(v)) < int(d.Min) || len([]rune(v)) > int(d.Max) && d.Max > 0 {
			return Problem("invalid")
		}
		if d.Kind == "enum" && !Has(d.Choices, v) {
			return Problem("invalid")
		}
		if strings.HasPrefix(d.Key, "smtp.") && strings.ContainsAny(v, "\r\n\x00") {
			return Problem("invalid")
		}
		if d.Key == "smtp.from" && v != "" {
			if _, err := mail.ParseAddress(v); err != nil {
				return Problem("invalid")
			}
		}
		if d.Key == "display.timezone" {
			if _, err := time.LoadLocation(v); err != nil {
				return Problem("invalid")
			}
		}
		if d.Key == "storage.directory" && (filepath.IsAbs(v) || !filepath.IsLocal(v) || strings.ContainsAny(v, "\\\x00")) {
			return Problem("invalid")
		}
		if d.Key == "storage.baseUrl" && v != "" {
			u, err := url.Parse(v)
			if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
				return Problem("invalid")
			}
		}
		if strings.HasPrefix(d.Key, "templates.") {
			for _, m := range placeholder.FindAllStringSubmatch(v, -1) {
				if !Has([]string{"applicant", "department", "requestId", "status", "link"}, m[1]) {
					return Problem("invalid")
				}
			}
			if strings.HasSuffix(d.Key, ".subject") && strings.ContainsAny(v, "\r\n") {
				return Problem("invalid")
			}
		}
	case "integer":
		var v int64
		if err := json.Unmarshal([]byte(raw), &v); err != nil || v < d.Min || v > d.Max {
			return Problem("invalid")
		}
	case "boolean":
		var v bool
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return Problem("invalid")
		}
	case "set":
		var v []string
		if err := json.Unmarshal([]byte(raw), &v); err != nil || v == nil || len(v) == 0 {
			return Problem("invalid")
		}
		seen := map[string]bool{}
		for _, a := range v {
			if seen[a] || !Has(d.Choices, a) {
				return Problem("invalid")
			}
			seen[a] = true
		}
	case "minutes":
		var v []int
		if err := json.Unmarshal([]byte(raw), &v); err != nil || v == nil || len(v) == 0 {
			return Problem("invalid")
		}
		seen := map[int]bool{}
		for _, a := range v {
			if seen[a] || !ValidMinutes(a) {
				return Problem("invalid")
			}
			seen[a] = true
		}
	default:
		return Problem("invalid")
	}
	if raw == "null" {
		return Problem("invalid")
	}
	return nil
}
func ValidMinutes(v int) bool {
	for _, x := range []int{30, 60, 120, 240, 480, 1440, 4320, 10080} {
		if x == v {
			return true
		}
	}
	return false
}

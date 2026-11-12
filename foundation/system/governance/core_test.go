package governance

import (
	"testing"
)

func TestEncryptionBindsTenantScopeAndKey(t *testing.T) {
	c := New(nil, nil, Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	v, err := c.Seal("a", "workspace:0", "smtp.password", `"private"`)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := c.Unseal("a", "workspace:0", "smtp.password", v)
	if err != nil || decoded != `"private"` {
		t.Fatal("roundtrip", err)
	}
	for _, tuple := range [][3]string{{"b", "workspace:0", "smtp.password"}, {"a", "company:1", "smtp.password"}, {"a", "workspace:0", "other"}} {
		if _, err = c.Unseal(tuple[0], tuple[1], tuple[2], v); err == nil {
			t.Fatal("ciphertext moved between security contexts")
		}
	}
	if _, err = c.Unseal("a", "workspace:0", "smtp.password", v[:len(v)-3]); err == nil {
		t.Fatal("tampering accepted")
	}
}
func TestRegisteredSettingsRejectUnsafeInput(t *testing.T) {
	for _, test := range []struct{ key, value string }{{"storage.directory", `"../../etc"`}, {"storage.baseUrl", `"javascript:alert(1)"`}, {"smtp.from", `"a@example.test\r\nBcc:b@example.test"`}, {"templates.requested.en.subject", `"{{password}}"`}, {"security.passwordMin", `0`}, {"display.timezone", `"Not/AZone"`}, {"files.extensions", `["exe"]`}, {"display.hour24", `null`}, {"display.pageSize", `null`}, {"branding.footer", `null`}, {"files.extensions", `null`}} {
		if err := FindDefinition(test.key).Validate(test.value); err == nil {
			t.Fatalf("accepted unsafe %s", test.key)
		}
	}
}

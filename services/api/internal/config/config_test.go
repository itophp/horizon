package config

import "testing"

func TestDevelopmentTokenRequiresLoopback(t *testing.T) {
	t.Setenv("DEMO_AUTH_TOKEN", "test-only")
	t.Setenv("API_ADDR", "0.0.0.0:8080")
	if _, err := Load(); err == nil {
		t.Fatal("development identity exposed publicly")
	}
	t.Setenv("API_ADDR", "127.0.0.1:8080")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

func TestSecureDeploymentRequiresPublicHTTPSOrigin(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("API_ADDR", "127.0.0.1:8080")
	t.Setenv("DEMO_AUTH_TOKEN", "")
	for _, raw := range []string{"", "http://public.test", "https://user:secret@public.test", "https://public.test/path", "https://public.test?token=value"} {
		t.Setenv("PUBLIC_APP_URL", raw)
		if _, err := Load(); err == nil {
			t.Fatalf("invalid public origin accepted: %s", raw)
		}
	}
	t.Setenv("PUBLIC_APP_URL", "https://public.test")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

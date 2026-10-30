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

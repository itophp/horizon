package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentKeepsOtherSettingsAndPrefersProcessEnvironment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("# local\r\nDATABASE_URL='postgres://user:old@localhost/db'\r\nCOOKIE_SECURE=false\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env, err := loadEnvironment(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://user:override@localhost/db")
	if got := env.get("DATABASE_URL"); !strings.Contains(got, ":override@") {
		t.Fatal("expected process override")
	}
	const saved = "postgres://user:p%23ss@localhost/db"
	if err = env.save(map[string]string{"DATABASE_URL": saved, "REDIS_URL": "redis://localhost:63790"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"# local\n", "DATABASE_URL=" + saved + "\n", "COOKIE_SECURE=false\n", "REDIS_URL=redis://localhost:63790\n"} {
		if !strings.Contains(string(raw), line) {
			t.Errorf("missing expected configuration line")
		}
	}
}

func TestEnvironmentChecksRedactInvalidCredentials(t *testing.T) {
	const secret = "not-a-valid-connection-secret"
	for _, check := range []envCheck{probeDatabase(secret), probeRedis(secret)} {
		if check.Status != "unavailable" || strings.Contains(check.Hint, secret) {
			t.Errorf("expected redacted error for %s", check.Name)
		}
	}
	if check := probeDatabase(""); check.Status != "unconfigured" || !check.Required {
		t.Error("database should be required")
	}
	if check := probeRedis(""); check.Status != "unconfigured" || check.Required {
		t.Error("redis should be optional")
	}
}

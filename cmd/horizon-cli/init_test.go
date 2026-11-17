package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitClonesProjectOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	source := filepath.Join(root, "template")
	const content = "Horizon project fixture\n"
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "README.md"},
		{"-c", "user.name=Horizon Test", "-c", "user.email=horizon@example.test", "-c", "commit.gpgsign=false", "commit", "-m", "Project fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "horizon", args: []string{"init", "--repo", source}},
		{name: "my project", args: []string{"init", "--repo", source, "my project"}},
	} {
		if err := run(test.args); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, test.name)
		if actual, err := os.ReadFile(filepath.Join(target, "README.md")); err != nil || string(actual) != content {
			t.Fatalf("cloned README: %q, %v", actual, err)
		}
		if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
			t.Fatal(err)
		}
		if err := run(test.args); err == nil {
			t.Fatalf("init accepted existing directory %s", target)
		}
		if actual, err := os.ReadFile(filepath.Join(target, "README.md")); err != nil || string(actual) != content {
			t.Fatalf("existing README changed: %q, %v", actual, err)
		}
	}
}

func TestInitFailedCloneLeavesNoEmptyProject(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := run([]string{"init", "--repo", filepath.Join(root, "missing"), "retry"}); err == nil {
		t.Fatal("init accepted a missing source")
	}
	if _, err := os.Lstat("retry"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed init left the destination behind: %v", err)
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowSpecGenerationAndSemanticFailures(t *testing.T) {
	dir := t.TempDir()
	if err := run(dir, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "example-lifecycle.workflow.json")
	if err := run("", path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	// The pure generator validates structure; semantic checks live separately.
	v["steps"].([]any)[0].(map[string]any)["unknownField"] = true
	raw, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = run("", path); err == nil {
		t.Fatal("unknown field passed structure check")
	}
	if err = run("", ""); err == nil {
		t.Fatal("missing mode accepted")
	}
	if err = run(dir, path); err == nil {
		t.Fatal("ambiguous mode accepted")
	}
}

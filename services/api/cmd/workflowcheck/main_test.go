package main

import (
	"baseline/services/api/internal/ai/orchestration/spec"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticCheckRejectsUnknownCapability(t *testing.T) {
	file := filepath.Join(t.TempDir(), "flow.json")
	d := spec.LifecycleExample()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = run(file); err != nil {
		t.Fatal(err)
	}
	d.Steps[0].Capability = "unknown.action"
	raw, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = run(file); err == nil {
		t.Fatal("unknown capability accepted")
	}
	if err = run(""); err == nil {
		t.Fatal("missing file accepted")
	}
}

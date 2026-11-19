package main

import (
	"encoding/json"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/workflow/spec"
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticCheckRejectsUnknownCapability(t *testing.T) {
	file := filepath.Join(t.TempDir(), "flow.json")
	d := neutralFlow()
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

// neutralFlow is a workflowcheck-owned fixture: no application identifiers.
func neutralFlow() spec.Definition {
	return spec.Definition{Name: "fixture-flow", Version: "1.0.0", Input: contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"), Steps: []spec.Step{
		{ID: "context", Capability: "settings.workspace.read", Version: "1.0.0", Input: map[string]spec.Mapping{}},
	}}
}

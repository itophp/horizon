// Command workflowspec generates workflow JSON Schema/examples and checks
// structure without application dependencies. workflowcheck validates semantics.
package main

import (
	runtime "baseline/services/api/internal/ai/orchestration/spec"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	out := flag.String("out", "", "write generated schema and built-in workflow examples to this directory")
	check := flag.String("check", "", "validate a workflow JSON file; does not execute or register it")
	flag.Parse()
	if err := run(*out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(out, file string) error {
	if (out == "") == (file == "") {
		return fmt.Errorf("specify exactly one of -out or -check")
	}

	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		d, err := runtime.DecodeDefinition(raw)
		if err != nil {
			return err
		}
		fmt.Printf("workflow structure valid: %s@%s (use workflowcheck for catalog semantics)\n", d.Name, d.Version)
		return nil
	}
	schema, err := runtime.SpecificationSchema()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	expected := map[string]bool{"workflow.schema.json": true}
	write := func(name string, v any) error {
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, name), append(raw, '\n'), 0644)
	}
	if err = write("workflow.schema.json", schema); err != nil {
		return err
	}
	for _, d := range runtime.Builtins() {
		name := d.Name + ".workflow.json"
		expected[name] = true
		if err = write(name, d); err != nil {
			return err
		}
	}
	// This directory contains only generated output. Removed built-ins must not
	// leave apparently executable stale examples behind.
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".workflow.json") && !expected[entry.Name()] {
			if err = os.Remove(filepath.Join(out, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

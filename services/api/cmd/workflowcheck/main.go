// Command workflowcheck checks JSON workflow declarations against the actual
// application's capability catalog without opening a database or executing them.
package main

import (
	"baseline/services/api/internal/ai/orchestration/spec"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/platform/capability/registry"
	"flag"
	"fmt"
	"os"
)

func main() {
	file := flag.String("file", "", "workflow JSON to validate")
	flag.Parse()
	if err := run(*file); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(file string) error {
	if file == "" {
		return fmt.Errorf("specify -file")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	d, err := spec.DecodeDefinition(raw)
	if err != nil {
		return err
	}
	catalog := registry.New((app.CapabilityModules{}).Entries())
	if catalog.Err() != nil {
		return fmt.Errorf("capability catalog invalid")
	}
	if err = d.Validate(catalog); err != nil {
		return err
	}
	fmt.Printf("workflow valid: %s@%s (not executed or authorized)\n", d.Name, d.Version)
	return nil
}

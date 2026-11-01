// Command entitycheck validates the declared business entities against the
// schema produced by the migrations. It needs no database: migrations are
// embedded and parsed in process, which keeps it inside the fast gate.
package main

import (
	"fmt"
	"os"
	"strings"

	"baseline/services/api/internal/app"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/metadata"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "entity check failed:")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// acceptedGaps lists entity names that stay undeclared on purpose. They are
// reported separately so the to-do list keeps meaning "needs work".
var acceptedGaps = map[string]string{
	"dictionary_version": "shares gov_versions with settings_version; modelled as one entity",
	"dictionary_draft":   "shares gov_drafts with settings_draft; rows differ only by domain",
	"settings_account":   "per-account rows of gov_values; settings already models the table",
}

func run() error {
	tables, err := metadata.Load()
	if err != nil {
		return fmt.Errorf("parse migrations: %w", err)
	}
	declared := app.Entities()
	structural, contracts := 0, 0
	for _, item := range declared {
		for _, problem := range item.Problems() {
			fmt.Fprintf(os.Stderr, "E0 %s\n", problem)
			structural++
		}
	}
	all := metadata.Validate(tables, declared)
	issues := metadata.Errors(all)
	hints := 0
	for _, issue := range all {
		if issue.Severity == metadata.SeverityWarning {
			fmt.Fprintf(os.Stderr, "hint %s\n", issue.String())
			hints++
			continue
		}
		fmt.Fprintln(os.Stderr, issue.String())
	}
	columns := 0
	for _, table := range tables {
		columns += len(table.Columns)
	}
	entries := (app.CapabilityModules{}).Entries()
	for _, entry := range entries {
		for _, problem := range entry.Definition.Problems() {
			fmt.Fprintf(os.Stderr, "C0 %s: %s\n", entry.Definition.Name, problem)
			contracts++
		}
	}
	catalog := registry.New(entries)
	catalog.RegisterEntities(declared)
	if contracts > 0 {
		// An invalid contract empties the catalog, which would turn every
		// transition into a misleading "unknown capability". Report the real
		// cause and skip the graph instead of burying it in noise.
		fmt.Fprintf(os.Stderr, "skipped transition checks: the capability catalog is invalid\n")
	} else {
		for _, issue := range metadata.ValidateTransitions(catalog.All(), declared) {
			if issue.Severity == metadata.SeverityWarning {
				fmt.Fprintf(os.Stderr, "hint %s\n", issue.String())
				hints++
				continue
			}
			fmt.Fprintln(os.Stderr, issue.String())
			issues = append(issues, issue)
		}
	}
	undeclared := catalog.UndeclaredEntities()
	fmt.Printf("entity check: %d declared, %d tables, %d columns, %d declaration problems, %d contract problems, %d schema issues\n",
		len(declared), len(tables), columns, structural, contracts, len(issues))
	if hints > 0 {
		fmt.Printf("hints (%d): statements that are true but have no key or capability enforcing them\n", hints)
	}
	var missing, accepted []string
	for _, name := range undeclared {
		if reason, ok := acceptedGaps[name]; ok {
			accepted = append(accepted, fmt.Sprintf("%s (%s)", name, reason))
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		fmt.Printf("not declared yet (%d): %s\n", len(missing), strings.Join(missing, ", "))
	}
	if len(accepted) > 0 {
		fmt.Printf("accepted gaps (%d): %s\n", len(accepted), strings.Join(accepted, "; "))
	}
	if structural > 0 || contracts > 0 || len(issues) > 0 {
		return fmt.Errorf("%d declaration problems, %d contract problems and %d schema issues",
			structural, contracts, len(issues))
	}
	return nil
}

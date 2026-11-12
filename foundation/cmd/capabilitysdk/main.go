// Command capabilitysdk derives TypeScript types from the registered Memory
// Center capability contracts. It never executes capabilities or opens a DB.
package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/runtime/app"
)

func nameOf(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		if r == '.' || r == '-' || r == '_' {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
			upper = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func tsType(s *contracts.Schema) string {
	if s == nil {
		panic("capability schema is missing")
	}
	switch s.Type {
	case "string":
		if len(s.Enum) == 0 {
			return "string"
		}
		values := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			values[i] = strconv.Quote(v)
		}
		return strings.Join(values, " | ")
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "Array<" + tsType(s.Items) + ">"
	case "object":
		if s.AdditionalProperties && len(s.Properties) == 0 {
			return "Record<string, unknown>"
		}
		keys := make([]string, 0, len(s.Properties))
		for key := range s.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		required := map[string]bool{}
		for _, key := range s.Required {
			required[key] = true
		}
		var b strings.Builder
		b.WriteString("{ ")
		for _, key := range keys {
			b.WriteString(strconv.Quote(key))
			if !required[key] {
				b.WriteString("?")
			}
			b.WriteString(": ")
			b.WriteString(tsType(s.Properties[key]))
			b.WriteString("; ")
		}
		b.WriteString("}")
		if s.AdditionalProperties {
			b.WriteString(" & Record<string, unknown>")
		}
		return b.String()
	default:
		panic("unsupported capability schema type: " + s.Type)
	}
}

func main() {
	entries := (app.CapabilityModules{}).Entries()
	if err := registry.New(entries).Err(); err != nil {
		fmt.Fprintln(os.Stderr, "invalid capability catalog:", err)
		os.Exit(1)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Definition.Name < entries[j].Definition.Name })
	var b strings.Builder
	b.WriteString("// Code generated from Memory Center capability definitions. DO NOT EDIT.\n\n")
	seen := map[string]bool{}
	for _, entry := range entries {
		d := entry.Definition
		if !strings.HasPrefix(d.Name, "memory.") || slices.Contains(d.Tags, "internal-only") {
			continue
		}
		if seen[nameOf(d.Name)] {
			fmt.Fprintln(os.Stderr, "duplicate TypeScript capability name:", d.Name)
			os.Exit(1)
		}
		seen[nameOf(d.Name)] = true
		fmt.Fprintf(&b, "export type %sInput = %s;\n", nameOf(d.Name), tsType(d.Input))
		fmt.Fprintf(&b, "export type %sOutput = %s;\n\n", nameOf(d.Name), tsType(d.Output))
	}
	if err := os.WriteFile("../apps/web/src/api/generated/capability-memory.ts", []byte(b.String()), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var plans strings.Builder
	plans.WriteString("// Code generated from Execution Plan capability definitions. DO NOT EDIT.\n\n")
	for _, entry := range entries {
		d := entry.Definition
		if !strings.HasPrefix(d.Name, "execution.plan.") {
			continue
		}
		fmt.Fprintf(&plans, "export type %sInput = %s;\n", nameOf(d.Name), tsType(d.Input))
		fmt.Fprintf(&plans, "export type %sOutput = %s;\n\n", nameOf(d.Name), tsType(d.Output))
	}
	if err := os.WriteFile("../apps/web/src/api/generated/capability-execution-plans.ts", []byte(plans.String()), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

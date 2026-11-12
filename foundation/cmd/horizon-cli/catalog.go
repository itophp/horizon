package main

import (
	"fmt"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/workflow/spec"
	"horizon/foundation/runtime/app"
	"os"
	"runtime/debug"
	"slices"
	"strings"
)

type item struct {
	Kind        string   `json:"kind"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Route       string   `json:"route,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Definition  any      `json:"definition,omitempty"`
}

type catalog struct {
	SchemaVersion int    `json:"schemaVersion"`
	Source        string `json:"source"`
	Revision      string `json:"revision,omitempty"`
	Modified      string `json:"modified,omitempty"`
	Items         []item `json:"items"`
}

type pageDefinition struct {
	Page      ar.PageV3     `json:"page"`
	Providers []ar.Provider `json:"providers"`
}

func loadCatalog() (catalog, *registry.Registry, error) {
	return loadCatalogKind("")
}

func loadCatalogKind(kind string) (catalog, *registry.Registry, error) {
	result := catalog{SchemaVersion: 1, Source: "compiled", Items: []item{}}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				result.Revision = setting.Value
			case "vcs.modified":
				result.Modified = setting.Value
			}
		}
	}
	var r *registry.Registry
	if kind == "" || kind == "workflow" {
		for _, d := range app.WorkflowDefinitions() {
			result.Items = append(result.Items, item{Kind: "workflow", ID: d.Name, Version: d.Version, Description: fmt.Sprintf("Workflow %s with %d steps and a declared input contract.", d.Name, len(d.Steps)), Definition: d})
		}
	}
	if kind == "" || kind == "graph" {
		for _, d := range app.GraphDefinitions() {
			result.Items = append(result.Items, item{Kind: "graph", ID: d.Name, Version: d.Version, Description: fmt.Sprintf("Bounded analysis Graph %s with %d declared capability reads.", d.Name, len(d.Reads)), Definition: d})
		}
	}
	if kind != "workflow" && kind != "graph" {
		plugins := app.BuiltinPlugins(nil)
		entries := (app.CapabilityModules{Plugins: plugins}).Entries()
		r = registry.New(entries)
		if err := r.Err(); err != nil {
			return result, nil, fmt.Errorf("capability registry: %w", err)
		}
		definitions := make([]contracts.Definition, 0, len(entries))
		for _, entry := range entries {
			d, _ := r.Resolve(entry.Definition.Name, entry.Definition.Version)
			def := d.Definition
			definitions = append(definitions, def)
			if kind == "" || kind == "capability" {
				result.Items = append(result.Items, item{Kind: "capability", ID: def.Name, Version: def.Version, Title: def.Title, Description: def.Description, Tags: def.Tags, Definition: def})
			}
		}
		if kind != "capability" {
			_, versions, err := ar.AssembleDeployment(app.RuntimeContributions(plugins), app.EntitiesWithPlugins(plugins), definitions)
			if err != nil {
				return result, nil, fmt.Errorf("runtime deployment: %w", err)
			}
			result.Items = append(result.Items, runtimeItems(kind, versions)...)
		}
	}
	slices.SortFunc(result.Items, func(a, b item) int {
		return strings.Compare(a.Kind+"/"+a.ID+"@"+a.Version, b.Kind+"/"+b.ID+"@"+b.Version)
	})
	return result, r, nil
}

func runtimeItems(kind string, versions []ar.ApplicationV2) []item {
	items := []item{}
	for _, a := range versions {
		if kind != "page" {
			items = append(items, item{Kind: "application", ID: a.ID, Version: a.Version, Title: a.Protocol.LabelKey, Route: a.Protocol.Route, Description: fmt.Sprintf("Runtime application %s from %s with %d V3 pages.", a.ID, a.Source, len(a.Protocol.Pages)), Definition: a})
		}
		if kind == "application" {
			continue
		}
		for _, page := range a.Protocol.Pages {
			providerIDs := map[string]bool{}
			walkPage(page.Root, func(layout *ar.LayoutSpec) {
				for _, data := range layout.Data {
					providerIDs[data.Provider] = true
				}
			}, func(*ar.ComponentSpec) {})
			providers := slices.DeleteFunc(slices.Clone(a.Protocol.Providers), func(p ar.Provider) bool { return !providerIDs[p.ID] })
			items = append(items, item{Kind: "page", ID: a.ID + "/" + page.ID, Version: a.Version, Title: page.TitleKey, Route: page.Route, Description: fmt.Sprintf("V3 page %s at %s in application %s.", page.ID, page.Route, a.ID), Definition: pageDefinition{page, providers}})
		}
	}
	return items
}

func inspect(args []string) error {
	if handled, err := inspectSchema(args); handled {
		return err
	}
	kind := ""
	if slices.Contains(schemaKinds, args[0]) {
		kind = args[0]
	}
	list := (args[0] == "catalog" || kind != "") && (len(args) == 1 || len(args) == 2 && args[1] == "--full")
	switch {
	case list:
	case len(args) == 2 && (args[0] == "search" || args[0] == "refs"):
	case len(args) == 3 && args[0] == "validate" && args[1] == "workflow":
	case len(args) == 3 && args[1] == "get" && slices.Contains(schemaKinds, args[0]):
	default:
		return fmt.Errorf("invalid command, see horizon-cli --help")
	}
	c, r, err := loadCatalogKind(kind)
	if err != nil {
		return err
	}
	if list {
		if len(args) == 1 {
			for i := range c.Items {
				c.Items[i].Definition = nil
			}
		}
		return printJSON(c)
	}
	switch args[0] {
	case "search":
		matches := []item{}
		for _, entry := range c.Items {
			haystack := strings.ToLower(strings.Join([]string{entry.Kind, entry.ID, entry.Title, entry.Description, entry.Route, strings.Join(entry.Tags, " ")}, " "))
			if strings.Contains(haystack, strings.ToLower(args[1])) {
				entry.Definition = nil
				matches = append(matches, entry)
			}
		}
		c.Items = matches
		return printJSON(c)
	case "refs":
		key, ok := strings.CutPrefix(args[1], "capability:")
		if !ok || key == "" {
			return fmt.Errorf("expected capability:<name[@version]>")
		}
		return printJSON(references(c.Items, key))
	case "validate":
		file, err := os.ReadFile(args[2])
		if err != nil {
			return err
		}
		def, err := spec.DecodeDefinition(file)
		if err != nil {
			return err
		}
		if err = def.ValidateWithGraphs(r, app.GraphDefinitions()); err != nil {
			return err
		}
		return printJSON(map[string]any{"valid": true, "name": def.Name, "version": def.Version})
	default:
		entry, err := findItem(c.Items, args[0], args[2])
		if err != nil {
			return err
		}
		return printJSON(entry)
	}
}

func findItem(items []item, kind, ref string) (item, error) {
	id, version, pinned := strings.Cut(ref, "@")
	matches := []item{}
	for _, entry := range items {
		if entry.Kind == kind && entry.ID == id && (!pinned || entry.Version == version) {
			matches = append(matches, entry)
		}
	}
	switch len(matches) {
	case 0:
		return item{}, fmt.Errorf("%s %q not found", kind, ref)
	case 1:
		return matches[0], nil
	default:
		return item{}, fmt.Errorf("%s %q has multiple versions, specify id@version", kind, id)
	}
}

func walkPage(node ar.Node, layout func(*ar.LayoutSpec), component func(*ar.ComponentSpec)) {
	if node.Layout != nil {
		layout(node.Layout)
		for _, child := range node.Layout.Children {
			walkPage(child, layout, component)
		}
	}
	if node.Component != nil {
		component(node.Component)
		if node.Component.Content != nil {
			walkPage(ar.Node{Layout: node.Component.Content}, layout, component)
		}
	}
}

type reference struct {
	Kind     string `json:"kind"`
	Owner    string `json:"owner"`
	Location string `json:"location"`
	Target   string `json:"target"`
}

func references(items []item, key string) []reference {
	found := []reference{}
	for _, entry := range items {
		add := func(location, target string) {
			name, _, _ := strings.Cut(target, "@")
			if key == target || key == name {
				found = append(found, reference{entry.Kind, entry.ID + "@" + entry.Version, location, target})
			}
		}
		switch d := entry.Definition.(type) {
		case spec.Definition:
			for _, step := range d.Steps {
				if step.Capability != "" {
					add(step.ID, step.Capability+"@"+step.Version)
				}
				if step.Compensation != nil {
					add(step.ID+"/compensation", step.Compensation.Capability+"@"+step.Compensation.Version)
				}
				if step.Agent != nil {
					for _, tool := range step.Agent.Tools {
						add(step.ID+"/tool", tool)
					}
				}
			}
		case spec.GraphDefinition:
			for _, read := range d.Reads {
				add("reads", read.Capability+"@"+read.Version)
			}
		case ar.ApplicationV2:
			for _, p := range d.Protocol.Providers {
				if p.Capability.Name != "" {
					add(p.ID, p.Capability.Name+"@"+p.Capability.Version)
				}
			}
		case pageDefinition:
			direct := map[string]bool{}
			walkPage(d.Page.Root, func(*ar.LayoutSpec) {}, func(c *ar.ComponentSpec) {
				for _, event := range c.Events {
					if event.Action == "invoke" {
						direct[event.Target] = true
						add(c.ID+"/"+event.Event, event.Target)
					}
				}
			})
			for _, p := range d.Providers {
				if p.Capability.Name != "" {
					add("provider/"+p.ID, p.Capability.Name+"@"+p.Capability.Version)
				}
			}
			for _, ref := range ar.ProtocolPageCapabilityRefs(d.Page) {
				target := ref.Name + "@" + ref.Version
				if !direct[target] {
					add("schema", target)
				}
			}
		}
	}
	slices.SortFunc(found, func(a, b reference) int {
		return strings.Compare(a.Kind+"/"+a.Owner+"/"+a.Location+"/"+a.Target, b.Kind+"/"+b.Owner+"/"+b.Location+"/"+b.Target)
	})
	return found
}

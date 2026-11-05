package applicationruntime_test

import (
	"baseline/services/api/applications/articles"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/runtime/app"
	"strings"
	"testing"
)

func fixture() ([]ar.ApplicationV2, []contracts.Entity, []contracts.Definition) {
	plugins := app.BuiltinPlugins(nil)
	entries := (app.CapabilityModules{Plugins: plugins}).Entries()
	defs := []contracts.Definition{}
	for _, e := range entries {
		defs = append(defs, e.Definition)
	}
	return append([]ar.ApplicationV2{articles.RuntimeApplication()}, plugins.Catalog.Applications...), app.EntitiesWithPlugins(plugins), defs
}
func TestMultipleApplicationsAndIndependentCommands(t *testing.T) {
	sources, entities, defs := fixture()
	apps, err := ar.Assemble(sources, entities, defs)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatal("second module missing")
	}
	project := apps[1]
	form := project.Pages[0].Forms[0]
	if form.Create.Permission != "projects:admin" || form.Edit.Permission != "projects:write" {
		t.Fatal(form)
	}
	if strings.Contains(form.Edit.InputSchema, "revision") {
		t.Fatal("article version injected into project")
	}
	if form.Edit.Capability.Name == "" {
		t.Fatal("non-transition update lost")
	}
	if len(project.Pages[0].Table.Columns) < 5 {
		t.Fatal("derived columns lost")
	}
	if len(project.Pages[1].Relations) == 0 {
		t.Fatal("relations missing")
	}
}
func TestCatalogRejectsUnsafeDefinitions(t *testing.T) {
	sources, entities, defs := fixture()
	cases := []struct {
		name   string
		mutate func([]ar.ApplicationV2) []ar.ApplicationV2
	}{
		{"standalone form missing command", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Pages[0].Type = "form"
			a[0].Pages[0].Actions = nil
			return a
		}},
		{"duplicate app", func(a []ar.ApplicationV2) []ar.ApplicationV2 { return append(a, a[0]) }},
		{"unknown version", func(a []ar.ApplicationV2) []ar.ApplicationV2 { a[0].SchemaVersion = "99"; return a }},
		{"unknown provider", func(a []ar.ApplicationV2) []ar.ApplicationV2 { a[0].Pages[0].Provider = "absent"; return a }},
		{"unknown capability version", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Providers[0].Capability.Version = "99.0.0"
			return a
		}},
		{"unknown field", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Pages[0].Table.Columns[0].Name = "tokenHash"
			return a
		}},
		{"sensitive projection", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Pages[0].Table.Columns = append(a[0].Pages[0].Table.Columns, ar.TextField("body", "articles:body"))
			return a
		}},
		{"input type", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[1].Pages[0].Forms[0].Create.Input[0] = ar.Const("code", "true")
			return a
		}},
		{"missing input path", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[1].Pages[0].Forms[0].Edit.Input[0] = ar.Bind("name", "form.unknown")
			return a
		}},
		{"prototype", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Pages[1].Actions[1].Input[0] = ar.Bind("flow", "record.__proto__.token")
			return a
		}},
		{"cycle", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[0].Dependencies = []string{"projects"}
			a[1].Dependencies = []string{"articles"}
			return a
		}},
		{"unknown component", func(a []ar.ApplicationV2) []ar.ApplicationV2 {
			a[1].Pages[0].Table.Columns[0].Component = "eval"
			return a
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ar.Assemble(test.mutate(ar.Canonical(sources)), entities, defs); err == nil {
				t.Fatal("invalid contribution accepted")
			}
		})
	}
}

func TestLegacyUpgradePreservesSensitiveFieldsAndRejectsUnknownVersion(t *testing.T) {
	old := articles.Application()
	converted, err := ar.Upgrade(old)
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Pages) != 2 {
		t.Fatal(converted)
	}
	if converted.Pages[1].Forms[0].Edit.Input[len(converted.Pages[1].Forms[0].Edit.Input)-1].Target != "revision" {
		t.Fatal("revision mapping lost")
	}
	old.Pages[0].SchemaVersion = "unknown"
	if _, err = ar.Upgrade(old); err == nil {
		t.Fatal("unsupported legacy version accepted")
	}
}

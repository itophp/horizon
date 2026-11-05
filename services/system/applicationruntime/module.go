// Package applicationruntime projects immutable definitions for the trusted principal.
package applicationruntime

import (
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Module struct {
	Applications []ar.ApplicationV2
	Versions     []ar.ApplicationV2
	Pool         *pgxpool.Pool
	Invoke       func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error)
	Definitions  map[string]contracts.Definition
}

func (m *Module) Entries() []registry.Entry {
	entries := []registry.Entry{}
	entities := []string{}
	seen := map[string]bool{}
	for _, application := range m.Applications {
		for _, page := range application.Pages {
			if !seen[page.EntityRef] {
				entities = append(entities, page.EntityRef)
				seen[page.EntityRef] = true
			}
		}
	}
	primary := ""
	if len(entities) > 0 {
		primary = entities[0]
	}

	for _, name := range []string{"application.runtime.resolve", "application.runtime.page.resolve", "application.runtime.list"} {
		props := map[string]*contracts.Schema{}
		required := []string{}
		if name != "application.runtime.list" {
			props["applicationId"] = contracts.Str(1, 80)
			required = append(required, "applicationId")
		}
		if name == "application.runtime.page.resolve" {
			props["pageId"] = contracts.Str(1, 100)
			required = append(required, "pageId")
		}
		d := contracts.Definition{Name: name, Version: "1.0.0", Title: name, Description: "Authorized runtime definition projection; contains no business records.", Domain: "runtime", Entity: primary, Entities: entities, Layer: contracts.LayerAtomic, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true, Executor: "go:" + name, Input: contracts.Object(props, required...), Output: ar.Schema(ar.ApplicationV2{})}
		if name == "application.runtime.page.resolve" {
			d.Output = ar.Schema(ar.PageV2{})
		}
		if name == "application.runtime.list" {
			d.Output = contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: ar.Schema(ar.ApplicationV2{})}}, "items")
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
			p, err := identity.Current(ctx)
			if err != nil {
				return contracts.Output{}, err
			}
			apps := []ar.ApplicationV2{}
			for _, source := range m.contributions() {
				if !p.Can(source.Permission) {
					continue
				}
				if m.Pool != nil {
					row, e := dbgen.New(m.Pool).RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: source.Source})
					state, se := dbgen.New(m.Pool).RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID})
					if e != nil || se != nil || row.Status != "enabled" || row.Version != source.PluginVersion || !state.Enabled {
						continue
					}
				}
				app := ar.Canonical(source)
				if m.Pool != nil {
					r, e := dbgen.New(m.Pool).RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: source.Source})
					if e != nil {
						return contracts.Output{}, e
					}
					app.InstallationRevision = r.Revision
					state, e := dbgen.New(m.Pool).RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID})
					if e != nil {
						return contracts.Output{}, e
					}
					app.ApplicationRevision = state.Revision
				}
				pages := []ar.PageV2{}
				for _, page := range app.Pages {
					if !p.Can(page.Permission) {
						continue
					}
					m.projectPageFields(app, &page, p)
					forms := []ar.FormV2{}
					commands := map[string]ar.FormV2{}
					for _, f := range page.Forms {
						if f.Create.Capability.Name != "" && !p.Can(f.Create.Permission) {
							f.Create = ar.Command{}
						}
						if f.Edit.Capability.Name != "" && !p.Can(f.Edit.Permission) {
							f.Edit = ar.Command{}
						}
						commands[f.ID] = f
						if f.Create.Capability.Name != "" || f.Edit.Capability.Name != "" {
							forms = append(forms, f)
						}
					}
					page.Forms = forms
					actions := []ar.Action{}
					for _, a := range page.Actions {
						if a.Permission != "" && !p.Can(a.Permission) {
							continue
						}
						if a.Type == "dialog" {
							f := commands[a.Target]
							if a.Mode == "create" && f.Create.Capability.Name == "" || a.Mode == "edit" && f.Edit.Capability.Name == "" {
								continue
							}
						}
						actions = append(actions, a)
					}
					page.Actions = actions
					if page.Type == "form" {
						hasCommand := false
						for _, action := range actions {
							if action.Type == "dialog" && (action.Mode == "create" || action.Mode == "edit") {
								hasCommand = true
							}
						}
						if !hasCommand {
							continue
						}
					}
					pages = append(pages, page)
				}
				visiblePages := map[string]bool{}
				for _, page := range pages {
					visiblePages[page.ID] = true
				}
				for i := range pages {
					visible := []ar.Action{}
					for _, a := range pages[i].Actions {
						if (a.Type == "navigation" || a.Type == "dialog" && a.Mode == "detail") && !visiblePages[a.Target] {
							continue
						}
						visible = append(visible, a)
					}
					pages[i].Actions = visible
				}
				app.Pages = pages
				providers := []ar.Provider{}
				for _, provider := range app.Providers {
					if p.Can(provider.Permission) {
						providers = append(providers, provider)
					}
				}
				app.Providers = providers
				extensions := []ar.ExtensionV2{}
				for _, x := range app.Extensions {
					allowed := true
					for _, permission := range x.Permissions {
						if !p.Can(permission) {
							allowed = false
						}
					}
					if allowed {
						extensions = append(extensions, x)
					}
				}
				app.Extensions = extensions
				app = ar.Canonical(app)
				if name == "application.runtime.list" {
					apps = append(apps, app)
					continue
				}
				if app.ID != in["applicationId"] {
					continue
				}
				var value any = app
				if name == "application.runtime.page.resolve" {
					found := false
					for _, page := range app.Pages {
						if page.ID == in["pageId"] {
							value = page
							found = true
						}
					}
					if !found {
						return contracts.Output{}, contracts.Problem("not_found")
					}
				}
				return projection(value)
			}
			if name == "application.runtime.list" {
				return projection(map[string]any{"items": apps})
			}
			return contracts.Output{}, contracts.Problem("not_found")
		}}})
	}
	return append(append(entries, m.lifecycleEntries()...), m.dataEntries()...)
}
func projection(value any) (contracts.Output, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return contracts.Output{}, err
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: data}, nil
}

func (m *Module) contributions() []ar.ApplicationV2 {
	if len(m.Versions) > 0 {
		return m.Versions
	}
	return m.Applications
}

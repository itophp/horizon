package applicationruntime

import (
	"encoding/json"
	"fmt"
)

// Upgrade converts the shipped v1 shape in memory. It never changes Entity or storage.
func Upgrade(old Application) (ApplicationV2, error) {
	for _, page := range old.Pages {
		if page.SchemaVersion != "1.0" {
			return ApplicationV2{}, fmt.Errorf("schema_version_unsupported")
		}
	}
	app := ApplicationV2{SchemaVersion: "2.0", ID: old.ID, Version: old.Version, Source: "builtin:" + old.ID, RuntimeCompatibility: "2.0", LabelKey: old.LabelKey, Route: old.Route}
	for _, p := range old.Pages {
		if p.Type == "form" {
			continue
		}
		page := PageV2{SchemaVersion: "2.0", ID: p.ID, Type: p.Type, Mode: "metadata", Route: old.Route, TitleKey: p.TitleKey, EntityRef: p.EntityRef, Layout: LayoutV2{Type: "standard", Columns: 1}, Provider: p.ID}
		provider := Provider{ID: p.ID, Capability: Ref(p.ReadCapability), KeyField: "id", Pagination: "none"}
		if p.Type == "detail" {
			page.Route += "/:id"
			provider.Input = []Mapping{Bind("id", "route.id")}
		} else {
			provider.ResultPath = "items"
			provider.Pagination = "server"
			provider.PageSize = 20
			provider.Input = []Mapping{Bind("page", "query.page"), Bind("pageSize", "query.pageSize")}
		}
		for _, f := range p.Fields {
			field := FieldV2{Name: f.Name, LabelKey: f.LabelKey, Type: f.Type, Component: f.Component, ComponentVersion: "2.0", Required: f.Required, ReadOnly: f.ReadOnly, Sensitive: f.Sensitive, Hidden: f.Hidden, MaxLength: f.MaxLength, Enum: f.Enum, ValueLabelPrefix: f.ValueLabelPrefix}
			if f.Component == "status" {
				field.Component = "text"
			}
			if f.Link {
				field.LinkPage = old.ID[:len(old.ID)] + ".detail"
			}
			if p.Type == "list" {
				page.Table.Columns = append(page.Table.Columns, field)
			} else {
				page.Fields = append(page.Fields, field)
			}
		}
		for _, form := range old.Pages {
			if form.Type != "form" {
				continue
			}
			f := FormV2{ID: form.ID, LabelKey: form.TitleKey, Tags: []string{old.ID}}
			f.Create.Capability = Ref(form.CreateCapability)
			f.Edit.Capability = Ref(form.UpdateCapability)
			for _, field := range form.Fields {
				v := FieldV2{Name: field.Name, LabelKey: field.LabelKey, Component: field.Component, ComponentVersion: "2.0", Type: field.Type, Hidden: field.Hidden}
				if !field.ReadOnly {
					f.Create.Fields = append(f.Create.Fields, v)
					f.Edit.Fields = append(f.Edit.Fields, v)
					f.Create.Input = append(f.Create.Input, Bind(field.Name, "form."+field.Name))
					f.Edit.Input = append(f.Edit.Input, Bind(field.Name, "form."+field.Name))
				}
			}
			f.Edit.Input = append(f.Edit.Input, Bind("id", "record.id"))
			if len(form.EditableStates) > 0 {
				raw, _ := json.Marshal(form.EditableStates)
				f.Edit.When = Condition("in", Path("record.status"), Literal(string(raw)))
				f.Edit.Input = append(f.Edit.Input, Bind("revision", "record.revision"))
			}
			page.Forms = append(page.Forms, f)
		}
		app.Pages = append(app.Pages, page)
		app.Providers = append(app.Providers, provider)
	}
	return Canonical(app), nil
}
func TextField(name, label string) FieldV2 {
	return FieldV2{Name: name, LabelKey: label, ComponentVersion: "2.0"}
}
func ActionCall(id, label, name string, input ...Mapping) Action {
	return Action{ID: id, Type: "capability", LabelKey: label, Placement: "detail", Capability: Ref(name), Input: input, Tags: []string{}}
}
func DialogAction(id, label, target, mode, placement string) Action {
	return Action{ID: id, Type: "dialog", LabelKey: label, Target: target, Mode: mode, Placement: placement}
}

package applicationruntime_test

import (
	"baseline/services/api/applications/articles"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"encoding/json"
	"testing"
)

func TestArticleMetadataAdapter(t *testing.T) {
	app := articles.Application()
	if app.MetadataVersion == "" || len(app.Pages) != 3 {
		t.Fatal(app)
	}
	raw, _ := json.Marshal(app)
	var input contracts.Input
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if err := ar.Schema(app).Validate(input); err != nil {
		t.Fatal(err)
	}
	for _, p := range app.Pages {
		for _, f := range p.Fields {
			if f.Name == "body" && (!f.Sensitive || f.MaxLength != 20000 && p.Type == "form") {
				t.Fatal(f)
			}
			if f.Name == "status" && !f.ReadOnly {
				t.Fatal(f)
			}
		}
	}
	if ar.Stamp(app).MetadataVersion != app.MetadataVersion {
		t.Fatal("unstable digest")
	}
}
func TestAdapterRejectsUnsafeFieldsAndVersions(t *testing.T) {
	entity := articles.Entities()[0]
	entries := (&articles.Module{}).Entries()
	get, write := entries[1].Definition, entries[3].Definition
	base := ar.Page{SchemaVersion: ar.Version, EntityRef: "article", Type: "detail", Layout: "standard", Fields: []ar.Field{{Name: "body", LabelKey: "articles:body", Component: "text"}}}
	for _, change := range []func(*ar.Page){func(p *ar.Page) { p.SchemaVersion = "2" }, func(p *ar.Page) { p.Fields[0].Name = "tenantId" }, func(p *ar.Page) { p.Fields[0].Component = "script" }, func(p *ar.Page) { p.Type = "script" }} {
		p := base
		p.Fields = append([]ar.Field{}, base.Fields...)
		change(&p)
		if _, err := ar.Adapt(p, entity, get, write); err == nil {
			t.Fatal("unsafe definition accepted", p)
		}
	}
}

func TestRelationProjectionOmitsStorageMapping(t *testing.T) {
	entity := articles.Entities()[0]
	entity.Relations = []contracts.Relation{{Name: "owner", Target: "account", Kind: "one", Via: "created_by"}}
	p := articles.Application().Pages[1]
	entries := (&articles.Module{}).Entries()
	got, err := ar.Adapt(p, entity, entries[1].Definition, entries[3].Definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Relations) != 1 || got.Relations[0].Target != "account" {
		t.Fatal(got.Relations)
	}
	raw, _ := json.Marshal(got.Relations)
	var value []map[string]any
	_ = json.Unmarshal(raw, &value)
	if _, exists := value[0]["via"]; exists {
		t.Fatal("storage mapping exposed")
	}
}

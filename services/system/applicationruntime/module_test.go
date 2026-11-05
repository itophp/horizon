package applicationruntime

import (
	"baseline/services/api/applications/articles"
	gen "baseline/services/api/gen/identity"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"testing"
)

func TestProjectionDoesNotMutateSharedDefinitions(t *testing.T) {
	source := articles.RuntimeApplication()
	for i := range source.Pages {
		source.Pages[i].Forms[0].Create.Permission = "articles:write"
		source.Pages[i].Forms[0].Edit.Permission = "articles:write"
	}
	m := Module{Applications: []ar.ApplicationV2{source}}
	entry := m.Entries()[0]
	if _, err := entry.Handler.Execute(context.Background(), contracts.Input{"applicationId": "articles"}); err == nil {
		t.Fatal("anonymous allowed")
	}
	account := &gen.AccountRecord{ID: 1}
	p, err := identity.NewPrincipal(account, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), p), contracts.Input{"applicationId": "articles"}); err == nil {
		t.Fatal("application without read permission exposed")
	}
	p, err = identity.NewPrincipal(account, "tenant-a", []*gen.PermissionGrant{{Permission: "articles:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), p), contracts.Input{"applicationId": "articles"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result.Data)
	var app ar.ApplicationV2
	if err = json.Unmarshal(raw, &app); err != nil {
		t.Fatal(err)
	}
	if len(app.Pages[0].Forms) != 0 {
		t.Fatal("write forms exposed")
	}
	if m.Applications[0].Pages[0].Forms[0].Edit.Capability.Name == "" {
		t.Fatal("shared definition mutated")
	}
	source.Pages[0].Type = "form"
	m.Applications = []ar.ApplicationV2{source}
	result, err = m.Entries()[0].Handler.Execute(identity.WithPrincipal(context.Background(), p), contracts.Input{"applicationId": "articles"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &app); err != nil {
		t.Fatal(err)
	}
	for _, page := range app.Pages {
		if page.ID == source.Pages[0].ID {
			t.Fatal("standalone write form exposed to reader")
		}
	}
}

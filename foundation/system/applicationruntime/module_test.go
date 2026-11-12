package applicationruntime

import (
	"context"
	"encoding/json"
	"horizon/applications/articles"
	gen "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"testing"
)

func TestProjectionRemovesUnauthorizedProtocolPagesAndEvents(t *testing.T) {
	source := articles.RuntimeApplication()
	definitions := map[string]contracts.Definition{}
	for _, entry := range (&articles.Module{}).Entries() {
		definitions[entry.Definition.Key()] = entry.Definition
	}
	for i := range source.Protocol.Providers {
		provider := &source.Protocol.Providers[i]
		provider.Permission = definitions[provider.Capability.Name+"@"+provider.Capability.Version].Permission
	}
	m := Module{Applications: []ar.ApplicationV2{source}, Definitions: definitions}
	entry := m.Entries()[0]
	if _, err := entry.Handler.Execute(context.Background(), contracts.Input{"applicationId": "articles"}); err == nil {
		t.Fatal("anonymous allowed")
	}
	account := &gen.AccountRecord{ID: 1}
	principal, err := identity.NewPrincipal(account, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), principal), contracts.Input{"applicationId": "articles"}); err == nil {
		t.Fatal("application without read permission exposed")
	}
	principal, err = identity.NewPrincipal(account, "tenant-a", []*gen.PermissionGrant{{Permission: "articles:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), principal), contracts.Input{"applicationId": "articles"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result.Data)
	var app ar.ApplicationV2
	if err = json.Unmarshal(raw, &app); err != nil {
		t.Fatal(err)
	}
	if app.Protocol == nil {
		t.Fatal("protocol missing")
	}
	if len(app.Protocol.Pages) < 2 {
		t.Fatal("read pages missing", principal.Can("articles:read"), app.Permission, app.Protocol.Pages)
	}
	for _, page := range app.Protocol.Pages {
		if page.Permission != "articles:read" {
			t.Fatal("write page exposed to reader", page.ID)
		}
	}
	listPage := app.Protocol.Pages[0]
	if listPage.ID != "article.list" {
		t.Fatal("unexpected page order", listPage.ID)
	}
	listComponent := listPage.Root.Layout.Children[1].Layout.Children[0].Component
	if _, exists := listComponent.Props["forms"]; exists {
		t.Fatal("unauthorized form commands exposed to reader")
	}
	detail := app.Protocol.Pages[1]
	if detail.ID != "article.detail" || detail.Root.Layout == nil {
		t.Fatal("detail page missing", app.Protocol.Pages)
	}
	detailComponent := detail.Root.Layout.Children[0].Layout.Children[0].Component
	if detailComponent == nil || len(detailComponent.Events) != 0 {
		t.Fatal("unauthorized detail actions exposed")
	}
	shared := m.Applications[0].Protocol.Pages[1].Root.Layout.Children[0].Layout.Children[0].Component
	if shared == nil || len(shared.Events) == 0 {
		t.Fatal("shared definition mutated")
	}
}

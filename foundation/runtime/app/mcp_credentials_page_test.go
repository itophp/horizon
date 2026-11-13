package app

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/integrations/mcp"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/applicationruntime"
	"horizon/foundation/system/identity"
)

func TestCredentialPanelCompilesAndProjectsForCurrentAccount(t *testing.T) {
	definitions := []contracts.Definition{}
	byKey := map[string]contracts.Definition{}
	for _, entry := range mcp.New(nil, nil).Entries() {
		definitions = append(definitions, entry.Definition)
		byKey[entry.Definition.Key()] = entry.Definition
	}

	const provider = "tokens"
	application := platformPage("agent-collaboration", "/mcp", "mcp:hub.title", "mcp:hub.subtitle", []ar.Provider{
		{ID: provider, Capability: ar.Ref("mcp.token.list"), KeyField: "id", Pagination: "none"},
	}, mcpCredentialPanel(provider, nil))
	catalog, err := ar.Assemble([]ar.ApplicationV2{application}, nil, definitions)
	if err != nil {
		t.Fatal(err)
	}
	module := &applicationruntime.Module{Applications: catalog, Definitions: byKey}
	var resolve contracts.Handler
	for _, entry := range module.Entries() {
		if entry.Definition.Name == "application.runtime.resolve" {
			resolve = entry.Handler
			break
		}
	}
	if resolve.Execute == nil {
		t.Fatal("Runtime resolve capability was unavailable")
	}

	for _, tc := range []struct {
		name   string
		grants []*genidentity.PermissionGrant
		want   map[string]bool
	}{
		{"owner", nil, map[string]bool{"mcp.token.create@1.0.0": true}},
		{"operator", []*genidentity.PermissionGrant{{Permission: mcpRevokePermission, Effect: "allow"}},
			map[string]bool{
				"mcp.token.create@1.0.0": true, "mcp.token.rotate@1.0.0": true,
				"mcp.token.revoke@1.0.0": false, "mcp.token.admin_revoke@1.0.0": false,
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			principal, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 1}, "tenant", tc.grants)
			if err != nil {
				t.Fatal(err)
			}
			result, err := resolve.Execute(identity.WithPrincipal(context.Background(), principal), contracts.Input{"applicationId": application.ID})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			var projected ar.ApplicationV2
			if err := json.Unmarshal(raw, &projected); err != nil {
				t.Fatal(err)
			}
			if projected.Protocol == nil || len(projected.Protocol.Pages) != 1 || len(projected.Protocol.Providers) != 1 {
				t.Fatal("the authorized page or its token provider was removed")
			}
			actual := map[string]bool{}
			var visit func(ar.Node)
			visit = func(node ar.Node) {
				if node.Layout != nil {
					for _, child := range node.Layout.Children {
						visit(child)
					}
				}
				if node.Component == nil {
					return
				}
				for _, event := range node.Component.Events {
					if event.Action != "invoke" {
						continue
					}
					if event.InputSchema == "" || event.IdempotencyKey == nil || event.ResultTarget != "" {
						t.Fatalf("unsafe or untyped invocation: %s", event.Target)
					}
					actual[event.Target] = event.Disclosure != nil
				}
				if node.Component.Content != nil {
					visit(ar.Node{Type: "layout", Layout: node.Component.Content})
				}
			}
			visit(projected.Protocol.Pages[0].Root)
			if !maps.Equal(actual, tc.want) {
				t.Fatalf("projected invocations and disclosures = %v, want %v", actual, tc.want)
			}
		})
	}
}

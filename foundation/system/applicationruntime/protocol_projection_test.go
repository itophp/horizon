package applicationruntime

import (
	"context"
	"encoding/json"
	"testing"

	genidentity "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

func protocolProjectionFixture(t *testing.T) (*Module, *identity.Principal) {
	t.Helper()
	service := identity.New(nil, false)
	definitions := []contracts.Definition{}
	for _, entry := range service.Entries() {
		definitions = append(definitions, entry.Definition)
	}
	applications, _, err := ar.AssembleDeployment([]ar.ApplicationV2{identity.RuntimeProtocolApplication()}, identity.Entities(), definitions)
	if err != nil {
		t.Fatal(err)
	}
	definitionMap := map[string]contracts.Definition{}
	for _, definition := range definitions {
		definitionMap[definition.Key()] = definition
	}
	principal, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 1}, "tenant", []*genidentity.PermissionGrant{{Permission: "accounts:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	return &Module{Applications: applications, Definitions: definitionMap}, principal
}

func TestProtocolProjectionFiltersUnauthorizedPagesAndProviders(t *testing.T) {
	module, reader := protocolProjectionFixture(t)
	entry := module.Entries()[0]
	result, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), reader), contracts.Input{"applicationId": "runtime-protocol"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result.Data)
	var allowed ar.ApplicationV2
	if err := json.Unmarshal(raw, &allowed); err != nil {
		t.Fatal(err)
	}
	if len(allowed.Protocol.Pages) != 1 || len(allowed.Protocol.Providers) != 1 {
		t.Fatalf("authorized protocol projection changed: %+v", allowed.Protocol)
	}

	denied, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 2}, "tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Handler.Execute(identity.WithPrincipal(context.Background(), denied), contracts.Input{"applicationId": "runtime-protocol"}); err == nil {
		t.Fatal("protocol application exposed without application permission")
	}
}

func TestAuthenticatedActionsRequireTrustedBuiltInAudience(t *testing.T) {
	module, reader := protocolProjectionFixture(t)
	app := ar.ApplicationV3{ID: "preferences", Source: "builtin:preferences", Permission: ar.AuthenticatedPermission}
	module.Applications = append(module.Applications, ar.ApplicationV2{
		ID: app.ID, Source: app.Source, Permission: app.Permission,
	})
	allowed := contracts.Definition{
		Name: "example.personal.update", Version: "1.0.0", Tags: []string{contracts.AuthenticatedActionTag},
	}
	untagged := contracts.Definition{Name: "example.other.update", Version: "1.0.0"}
	module.Definitions[allowed.Key()] = allowed
	module.Definitions[untagged.Key()] = untagged

	projected := module.allowedEventTargets(reader, app)
	if !projected[allowed.Key()] || projected[untagged.Key()] {
		t.Fatal("unexpected caller-scoped action projection")
	}
	app.Source = "tenant:preferences"
	if module.allowedEventTargets(reader, app)[allowed.Key()] {
		t.Fatal("a tenant-owned application received a built-in action")
	}
}

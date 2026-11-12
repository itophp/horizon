package identity_test

import (
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"testing"
)

func TestRuntimeProtocolApplicationAssembles(t *testing.T) {
	service := identity.New(nil, false)
	definitions := []contracts.Definition{}
	for _, entry := range service.Entries() {
		definitions = append(definitions, entry.Definition)
	}
	applications, _, err := ar.AssembleDeployment(
		[]ar.ApplicationV2{identity.RuntimeProtocolApplication()},
		identity.Entities(),
		definitions,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(applications) != 1 || applications[0].Protocol == nil {
		t.Fatal("protocol application missing")
	}
	provider := applications[0].Protocol.Providers[0]
	if provider.Permission != "accounts:read" || provider.InputSchema == "" || provider.OutputSchema == "" {
		t.Fatalf("provider not bound to capability contract: %+v", provider)
	}
}

func TestRuntimeProtocolRejectsNonReadOnlyProvider(t *testing.T) {
	service := identity.New(nil, false)
	definitions := []contracts.Definition{}
	for _, entry := range service.Entries() {
		definitions = append(definitions, entry.Definition)
	}
	source := identity.RuntimeProtocolApplication()
	source.Protocol.Providers[0].Capability.Name = "identity.account.create"
	if _, _, err := ar.AssembleDeployment([]ar.ApplicationV2{source}, identity.Entities(), definitions); err == nil {
		t.Fatal("write capability accepted as protocol provider")
	}
}

func TestRuntimeProtocolRejectsUnknownSystemAdapter(t *testing.T) {
	service := identity.New(nil, false)
	definitions := []contracts.Definition{}
	for _, entry := range service.Entries() {
		definitions = append(definitions, entry.Definition)
	}
	source := identity.RuntimeProtocolApplication()
	source.Protocol.Providers[0].Adapter = "system.unregistered"
	if _, _, err := ar.AssembleDeployment([]ar.ApplicationV2{source}, identity.Entities(), definitions); err == nil {
		t.Fatal("unregistered system adapter accepted")
	}
}

package identity_test

import (
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"testing"
)

func TestAccountsRuntimeApplicationAssembles(t *testing.T) {
	service := identity.New(nil, false)
	definitions := []contracts.Definition{}
	for _, entry := range service.Entries() {
		definitions = append(definitions, entry.Definition)
	}
	app := identity.AccountsRuntimeApplication()
	if _, _, err := ar.AssembleDeployment([]ar.ApplicationV2{app}, identity.Entities(), definitions); err != nil {
		t.Fatal(err)
	}
}

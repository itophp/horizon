//go:build integration

package tests

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"horizon/deployment"
	"horizon/foundation/gen/system"
	"horizon/foundation/runtime/transport"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
)

func deploymentHandler(service system.Service, core *governance.Core, identities ...*identity.Service) http.Handler {
	handler, _ := deploymentHandlerWithRuntime(service, core, identities...)
	return handler
}

func deploymentHandlerWithRuntime(service system.Service, core *governance.Core, identities ...*identity.Service) (http.Handler, func(context.Context)) {
	// The HTTP harness assembles the full deployment, including plugin
	// contributions, so managed applications reconcile like production.
	manager := deployment.BuiltinPlugins(core)
	for _, iam := range identities {
		_ = iam.ConfigurePluginPermissions(manager.Catalog.Permissions)
	}
	return transport.HandlerWithRegistration(service, core, deployment.WithPlugins(core, manager), identities...)
}

// pluginIAM returns an identity service whose catalog carries the declarative
// permissions contributed by the builtin application plugins, mirroring the
// production assembly where the transport layer merges plugin permissions into
// the identity instance before any principal is constructed.
func pluginIAM(t *testing.T, pool *pgxpool.Pool) *identity.Service {
	t.Helper()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{
		EncryptionKey: []byte("01234567890123456789012345678901"),
		StorageRoot:   t.TempDir(),
	})
	manager := deployment.BuiltinPlugins(core)
	if err := iam.ConfigurePluginPermissions(manager.Catalog.Permissions); err != nil {
		t.Fatal(err)
	}
	return iam
}

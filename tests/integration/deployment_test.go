//go:build integration

package tests

import (
	"context"
	"net/http"

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
	return transport.HandlerWithRegistration(service, core, deployment.Register(core), identities...)
}

package transport

import (
	genassistant "baseline/services/api/gen/assistant"
	genbranding "baseline/services/api/gen/branding"
	gendictionaries "baseline/services/api/gen/dictionaries"
	genfiles "baseline/services/api/gen/files"
	assistanthttp "baseline/services/api/gen/http/assistant/server"
	brandinghttp "baseline/services/api/gen/http/branding/server"
	dictionarieshttp "baseline/services/api/gen/http/dictionaries/server"
	fileshttp "baseline/services/api/gen/http/files/server"
	mcphttp "baseline/services/api/gen/http/mcp_management/server"
	notificationshttp "baseline/services/api/gen/http/notifications/server"
	settingshttp "baseline/services/api/gen/http/settings/server"
	genmcp "baseline/services/api/gen/mcp_management"
	gennotifications "baseline/services/api/gen/notifications"
	gensettings "baseline/services/api/gen/settings"
	assistantservice "baseline/services/api/internal/assistant"
	"baseline/services/api/internal/dictionaries"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/mcp"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/settings"
	"context"
	goahttp "goa.design/goa/v3/http"
	"net/http"
)

func mountGovernance(mux goahttp.Muxer, core *governance.Core, onError func(context.Context, http.ResponseWriter, error)) {
	core.Identity.Policies = core
	mcpService := mcp.New(core.Pool, core.Identity)
	mcphttp.Mount(mux, mcphttp.New(genmcp.NewEndpoints(mcpService), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method, "/mcp", mcpService.Handler(core).ServeHTTP)
	}
	f, ok := core.Logos.(*files.Service)
	if !ok {
		f = files.New(core)
	}
	b := &files.Branding{Files: f}
	settingshttp.Mount(mux, settingshttp.New(gensettings.NewEndpoints(settings.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	assistanthttp.Mount(mux, assistanthttp.New(genassistant.NewEndpoints(assistantservice.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	dictionarieshttp.Mount(mux, dictionarieshttp.New(gendictionaries.NewEndpoints(dictionaries.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	fileshttp.Mount(mux, fileshttp.New(genfiles.NewEndpoints(f), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	notificationshttp.Mount(mux, notificationshttp.New(gennotifications.NewEndpoints(notifications.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	brandinghttp.Mount(mux, brandinghttp.New(genbranding.NewEndpoints(b), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
}

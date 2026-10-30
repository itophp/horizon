package transport

import (
	genbranding "baseline/services/api/gen/branding"
	gendictionaries "baseline/services/api/gen/dictionaries"
	genfiles "baseline/services/api/gen/files"
	brandinghttp "baseline/services/api/gen/http/branding/server"
	dictionarieshttp "baseline/services/api/gen/http/dictionaries/server"
	fileshttp "baseline/services/api/gen/http/files/server"
	notificationshttp "baseline/services/api/gen/http/notifications/server"
	settingshttp "baseline/services/api/gen/http/settings/server"
	gennotifications "baseline/services/api/gen/notifications"
	gensettings "baseline/services/api/gen/settings"
	"baseline/services/api/internal/dictionaries"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/settings"
	"context"
	goahttp "goa.design/goa/v3/http"
	"net/http"
)

func mountGovernance(mux goahttp.Muxer, core *governance.Core, onError func(context.Context, http.ResponseWriter, error)) {
	core.Identity.Policies = core
	f, ok := core.Logos.(*files.Service)
	if !ok {
		f = files.New(core)
	}
	b := &files.Branding{Files: f}
	settingshttp.Mount(mux, settingshttp.New(gensettings.NewEndpoints(settings.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	dictionarieshttp.Mount(mux, dictionarieshttp.New(gendictionaries.NewEndpoints(dictionaries.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	fileshttp.Mount(mux, fileshttp.New(genfiles.NewEndpoints(f), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	notificationshttp.Mount(mux, notificationshttp.New(gennotifications.NewEndpoints(notifications.New(core)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	brandinghttp.Mount(mux, brandinghttp.New(genbranding.NewEndpoints(b), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
}

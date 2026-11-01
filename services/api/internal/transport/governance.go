package transport

import (
	genai "baseline/services/api/gen/ai"
	genassistant "baseline/services/api/gen/assistant"
	genbranding "baseline/services/api/gen/branding"
	gencapability "baseline/services/api/gen/capability"
	aihttp "baseline/services/api/gen/http/ai/server"
	assistanthttp "baseline/services/api/gen/http/assistant/server"
	brandinghttp "baseline/services/api/gen/http/branding/server"
	capabilityhttp "baseline/services/api/gen/http/capability/server"
	assistantservice "baseline/services/api/internal/ai/assistant"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/ai/knowledge"
	orchestrationruntime "baseline/services/api/internal/ai/orchestration"
	"baseline/services/api/internal/ai/triggers"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/modules/articles"
	"baseline/services/api/internal/modules/files"
	"baseline/services/api/internal/modules/mcp"
	"baseline/services/api/internal/platform/approval"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/eventhub"
	"baseline/services/api/internal/platform/outbox"
	"context"
	"log/slog"
	"net/http"

	goahttp "goa.design/goa/v3/http"
)

// mountGovernance wires the capability platform and returns the background
// starter (outbox relay and AI trigger engine). Every business module now
// contributes capability entries instead of its own HTTP endpoints.
func mountGovernance(mux goahttp.Muxer, core *governance.Core, onError func(context.Context, http.ResponseWriter, error)) func(context.Context) {
	core.Identity.Policies = core

	f, _ := core.Logos.(*files.Service)
	if f == nil {
		f = files.New(core)
	}
	brandinghttp.Mount(mux, brandinghttp.New(genbranding.NewEndpoints(brandingProjection{inner: &files.Branding{Files: f}}), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))

	// The MCP protocol endpoint is not a business command: it authenticates a
	// token and then invokes capabilities like any other adapter.
	mcpService := mcp.New(core.Pool, core.Identity)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		mux.Handle(method, "/mcp", mcpService.Handler(core).ServeHTTP)
	}

	documents := &knowledge.Store{Q: core.Q, Core: core}
	knowledgeService := &knowledge.Service{Store: documents}
	runs := harness.NewStore(core.Pool)
	auditStore := audit.NewStore(core.Pool)
	approvals := approval.NewStore(core.Pool)
	hub := eventhub.NewHub()
	events := eventhub.NewStore(core.Pool)

	orchestrator := orchestrationruntime.New(core, orchestrationruntime.Builtins())
	articleModule := &articles.Module{Pool: core.Pool}
	entries := (app.CapabilityModules{Articles: articleModule, Core: core, MCP: mcpService, Orchestration: orchestrator, Documents: documents, Knowledge: knowledgeService}).Entries()
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		slog.Error("capability catalog is invalid", "code", "internal")
	}
	catalog.RegisterEntities(app.Entities())
	for _, problem := range catalog.EntityProblems() {
		slog.Error("entity declaration is invalid", "detail", problem)
	}

	dispatcher := gateway.New(gateway.Options{
		Registry: catalog, Identity: core.Identity, Pool: core.Pool,
		Approvals: approvals, Audit: auditStore, Events: events,
	})
	if err := orchestrator.Attach(dispatcher, catalog); err != nil {
		panic("invalid workflow definitions")
	}
	mcpService.AttachGateway(dispatcher)
	agentService := harness.New(core, catalog, dispatcher, runs, auditStore, documents)
	articleModule.Gateway = dispatcher
	articleModule.Classify = agentService.ClassifyArticle
	orchestrator.AgentModel = agentService.WorkflowModel
	engine := triggers.NewEngine(triggers.NewStore(core.Pool), hub, agentService, core.Identity)
	relay := outbox.NewRelay(core.Pool, hub)

	capabilityhttp.Mount(mux, capabilityhttp.New(gencapability.NewEndpoints(newCapability(dispatcher, catalog)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	aihttp.Mount(mux, aihttp.New(genai.NewEndpoints(newAI(agentService)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	assistanthttp.Mount(mux, assistanthttp.New(genassistant.NewEndpoints(newAssistant(assistantservice.New(core), agentService, knowledgeService)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))

	return func(ctx context.Context) {
		engine.Start(ctx)
		go orchestrator.Run(ctx)
		go relay.Run(ctx)
	}
}

// brandingProjection adapts the domain branding projection to the public Goa
// contract. The conversion is the only place the public endpoint touches the
// module types; no business rule lives here.
type brandingProjection struct{ inner *files.Branding }

func (b brandingProjection) Get(ctx context.Context) (*genbranding.EffectiveConfiguration, error) {
	res, err := b.inner.Get(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]*genbranding.ConfigEntry, 0, len(res.Entries))
	for _, entry := range res.Entries {
		entries = append(entries, &genbranding.ConfigEntry{
			Key: entry.Key, Value: entry.Value, Source: entry.Source,
			Overridden: entry.Overridden, Configured: entry.Configured,
		})
	}
	return &genbranding.EffectiveConfiguration{Entries: entries, Revision: res.Revision}, nil
}

func (b brandingProjection) Logo(ctx context.Context) (*genbranding.FileContent, error) {
	res, err := b.inner.Logo(ctx)
	if err != nil {
		return nil, err
	}
	return &genbranding.FileContent{Name: res.Name, Mime: res.Mime, Content: res.Content}, nil
}

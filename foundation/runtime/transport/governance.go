package transport

import (
	"context"
	assistantservice "horizon/foundation/ai/assistant"
	"horizon/foundation/ai/harness"
	"horizon/foundation/ai/knowledge"
	"horizon/foundation/ai/triggers"
	genai "horizon/foundation/gen/ai"
	genassistant "horizon/foundation/gen/assistant"
	genbranding "horizon/foundation/gen/branding"
	gencapability "horizon/foundation/gen/capability"
	aihttp "horizon/foundation/gen/http/ai/server"
	assistanthttp "horizon/foundation/gen/http/assistant/server"
	brandinghttp "horizon/foundation/gen/http/branding/server"
	capabilityhttp "horizon/foundation/gen/http/capability/server"
	"horizon/foundation/integrations/mcp"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/kernel/executionplan"
	"horizon/foundation/kernel/outbox"
	orchestrationruntime "horizon/foundation/kernel/workflow"
	"horizon/foundation/runtime/app"
	"horizon/foundation/system/applicationruntime"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	memorysystem "horizon/foundation/system/memory"
	"horizon/internal/repository/dbgen"
	"net/http"
	"strings"
	"sync"

	goahttp "goa.design/goa/v3/http"
)

// mountGovernance wires the capability platform and returns the background
// starter (outbox relay and AI trigger engine). Every business module now
// contributes capability entries instead of its own HTTP endpoints.
func mountGovernance(mux goahttp.Muxer, core *governance.Core, registration app.Registration, observer *memorysystem.Observer, onError func(context.Context, http.ResponseWriter, error)) func(context.Context) {
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
	hub.Subscribe(documents.CleanUpdatedDocument)
	events := eventhub.NewStore(core.Pool)

	orchestrator := orchestrationruntime.NewWithGraphs(core, registration.WorkflowDefinitions(), registration.Graphs)
	orchestrator.Observations = observer
	memoryModule := memorysystem.New(core.Pool)
	memoryModule.AttachIdentity(core.Identity)
	plugins := registration.Plugins
	if plugins != nil {
		if err := core.Identity.ConfigurePluginPermissions(plugins.Catalog.Permissions); err != nil {
			panic(err)
		}
	}
	runtimeModule := &applicationruntime.Module{}
	entries := (app.CapabilityModules{Runtime: runtimeModule, Registration: registration, Core: core, MCP: mcpService, Orchestration: orchestrator, Documents: documents, Knowledge: knowledgeService, Memory: memoryModule}).Entries()
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		panic("capability catalog is invalid: " + err.Error())
	}
	catalog.RegisterEntities(app.EntitiesWithRegistration(registration))
	if err := catalog.Err(); err != nil {
		panic("invalid entity declarations: " + strings.Join(catalog.EntityProblems(), "; "))
	}

	for _, definition := range catalog.All() {
		if err := executionplan.Seed(context.Background(), dbgen.New(core.Pool), definition); err != nil {
			panic("execution plan initialization failed")
		}
	}
	dispatcher := gateway.New(gateway.Options{
		Registry: catalog, Identity: core.Identity, Pool: core.Pool, RuntimeCheck: runtimeModule.ValidateInvocation,
		Approvals: approvals, ApprovalCipher: core, Audit: auditStore, Events: events, Observations: observer,
	})
	for _, attach := range registration.AttachGateway {
		attach(dispatcher)
	}
	core.Identity.AttachAnalyticsGateway(func(ctx context.Context, name, version string, input contracts.Input) (*contracts.Result, error) {
		return dispatcher.Invoke(ctx, gateway.Request{Name: name, Version: version, Input: input})
	})
	runtimeModule.Invoke = func(ctx context.Context, name, version string, input contracts.Input, key string) (*contracts.Result, error) {
		return dispatcher.Invoke(ctx, gateway.Request{Name: name, Version: version, Input: input, IdempotencyKey: key})
	}
	if err := runtimeModule.Reconcile(context.Background(), dispatcher); err != nil {
		panic(err)
	}
	memoryModule.AttachGateway(dispatcher)
	if err := orchestrator.Attach(dispatcher, catalog); err != nil {
		panic("invalid workflow definitions")
	}
	if plugins != nil {
		plugins.AttachCapabilities(runtimeModule.Invoke)
	}
	mcpService.AttachGateway(dispatcher)
	agentService := harness.New(core, catalog, dispatcher, runs, auditStore, documents)
	agentService.Profile = runtimeModule.AgentProfile
	runtimeModule.GenerateAI = agentService.GenerateRuntimeAI
	runtimeModule.GenerateAIWithImage = agentService.GenerateRuntimeAIWithImage
	if plugins != nil {
		plugins.AttachModel(agentService.GenerateProjectImportAI)
	}
	orchestrator.AgentModel = agentService.WorkflowModel
	engine := triggers.NewEngine(triggers.NewStore(core.Pool), hub, agentService, core.Identity)
	relay := outbox.NewRelay(core.Pool, hub)
	memoryConsumer := memorysystem.NewConsumer(core.Pool)

	capabilityhttp.Mount(mux, capabilityhttp.New(gencapability.NewEndpoints(newCapability(dispatcher, catalog)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	aihttp.Mount(mux, aihttp.New(genai.NewEndpoints(newAI(agentService)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))
	assistanthttp.Mount(mux, assistanthttp.New(genassistant.NewEndpoints(newAssistant(assistantservice.New(core), agentService, knowledgeService)), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, onError, nil))

	workers := append(app.PlatformWorkers(core), registration.Workers...)
	var once sync.Once
	return func(ctx context.Context) {
		once.Do(func() {
			engine.Start(ctx)
			for _, worker := range workers {
				go worker(ctx, dispatcher, agentService)
			}
			go orchestrator.Run(ctx)
			go relay.Run(ctx)
			go memoryConsumer.Run(ctx)
		})
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

package deployment

import (
	"context"
	"strings"

	"horizon/applications/articles"
	"horizon/applications/drive"
	"horizon/applications/example"
	"horizon/applications/projects"
	"horizon/foundation/ai/harness"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/plugin"
	workflow "horizon/foundation/kernel/workflow/spec"
	"horizon/foundation/runtime/app"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
)

func Register(core *governance.Core) app.Registration {
	return WithPlugins(core, nil)
}

func WithPlugins(core *governance.Core, plugins *plugin.Manager) app.Registration {
	if core == nil {
		core = governance.New(nil, identity.New(nil, false), governance.Options{})
	}
	if plugins == nil {
		plugins = BuiltinPlugins(projects.New(core.Pool))
	}
	articleModule := &articles.Module{Pool: core.Pool}
	articleModule.Blocked = func(ctx context.Context, p *identity.Principal, content string) (bool, bool, error) {
		raw, err := core.Setting(ctx, p, "articles.blockedWords")
		if err != nil {
			if contracts.Code(err) == "invalid" {
				return false, false, nil
			}
			return false, false, err
		}
		words, err := governance.String(raw)
		if err != nil || strings.TrimSpace(words) == "" {
			return false, false, err
		}
		return articles.ContainsBlocked(content, words), true, nil
	}
	fileService, _ := core.Logos.(*files.Service)
	if fileService == nil {
		fileService = files.New(core)
	}
	exampleModule := example.New(core.Pool)
	entries := append(articleModule.Entries(), drive.NewWithFiles(core.Pool, fileService).Entries()...)
	entries = append(entries, exampleModule.Entries()...)
	for i := range entries {
		if entries[i].Definition.Domain == "articles" {
			entries[i].Definition.Installation = "builtin:articles"
			entries[i].Definition.Application = "articles"
		}
	}
	return app.Registration{
		Plugins: plugins, Entries: entries,
		Workflows: append(workflow.Builtins(), workflow.ApplicationAnalyses()...), Graphs: workflow.BuiltinGraphs(),
		Entities:      append(append(articles.Entities(), drive.Entities()...), example.Entities()...),
		Applications:  []ar.ApplicationV2{articles.RuntimeApplication(), app.CapabilityRuntimeApplication()},
		AttachGateway: []func(*gateway.Gateway){exampleModule.AttachGateway},
		Workers: []app.Worker{func(ctx context.Context, gw *gateway.Gateway, _ *harness.Service) {
			drive.RunCleanup(ctx, core.Pool, gw)
		}},
	}
}

func BuiltinPlugins(module *projects.Module) *plugin.Manager {
	if module == nil {
		module = projects.New(nil)
	}
	manager := plugin.New(projects.Plugin{Module: module})
	if err := manager.Register(context.Background()); err != nil {
		panic(err)
	}
	for i := range manager.Catalog.Entries {
		manager.Catalog.Entries[i].Definition.Installation = "com.horizon.projects"
		manager.Catalog.Entries[i].Definition.Application = "projects"
	}
	if err := identity.New(nil, false).ConfigurePluginPermissions(manager.Catalog.Permissions); err != nil {
		panic(err)
	}
	return manager
}

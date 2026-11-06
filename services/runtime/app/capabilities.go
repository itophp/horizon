package app

import (
	"baseline/services/api/ai/knowledge"
	"baseline/services/api/applications/articles"
	"baseline/services/api/applications/drive"
	"baseline/services/api/applications/example"
	"baseline/services/api/applications/projects"
	"baseline/services/api/integrations/mcp"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/plugin"
	runtime "baseline/services/api/kernel/workflow"
	armodule "baseline/services/api/system/applicationruntime"
	"baseline/services/api/system/dictionaries"
	"baseline/services/api/system/executionplans"
	"baseline/services/api/system/files"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"baseline/services/api/system/memory"
	"baseline/services/api/system/notifications"
	"baseline/services/api/system/orchestration"
	"baseline/services/api/system/settings"
)

// CapabilityModules is the shared composition root for runtime and offline
// checks. Each domain still owns its definitions; tools must not copy catalogs.
// Zero value builds declarations without opening a database or executing them.
type CapabilityModules struct {
	Example        *example.Module
	ExecutionPlans *executionplans.Module
	Runtime        *armodule.Module
	Articles       *articles.Module
	Projects       *projects.Module
	Plugins        *plugin.Manager
	Core           *governance.Core
	MCP            *mcp.Service
	Orchestration  *runtime.Engine
	Documents      *knowledge.Store
	Knowledge      *knowledge.Service
	Memory         *memory.Module
}

func (m CapabilityModules) Entries() []registry.Entry {
	if m.Core == nil {
		m.Core = governance.New(nil, identity.New(nil, false), governance.Options{})
	}
	if m.MCP == nil {
		m.MCP = mcp.New(m.Core.Pool, m.Core.Identity)
	}
	if m.Orchestration == nil {
		m.Orchestration = runtime.NewWithGraphs(m.Core, WorkflowDefinitions(), GraphDefinitions())
	}
	if m.Documents == nil {
		m.Documents = &knowledge.Store{Q: m.Core.Q, Core: m.Core}
	}
	if m.Knowledge == nil {
		m.Knowledge = &knowledge.Service{Store: m.Documents}
	}
	if m.Articles == nil {
		m.Articles = &articles.Module{Pool: m.Core.Pool}
	}
	if m.Projects == nil {
		m.Projects = projects.New(m.Core.Pool)
	}
	fileService, _ := m.Core.Logos.(*files.Service)
	if fileService == nil {
		fileService = files.New(m.Core)
	}
	if m.ExecutionPlans == nil {
		m.ExecutionPlans = &executionplans.Module{Pool: m.Core.Pool}
	}
	m.ExecutionPlans.ValidateWorkflow = m.Orchestration.ValidatePlanWithQueries
	m.ExecutionPlans.ValidateGraph = m.Orchestration.ValidateGraphPlan
	m.ExecutionPlans.References = m.Orchestration.PlanReferences
	m.ExecutionPlans.Inspect = m.Orchestration.InspectPlanReference
	entries := m.ExecutionPlans.Entries()

	entries = append(entries, m.Articles.Entries()...)
	if m.Plugins == nil {
		m.Plugins = BuiltinPlugins(m.Projects)
	}
	entries = append(entries, m.Plugins.Catalog.Entries...)
	entries = append(entries, drive.NewWithFiles(m.Core.Pool, fileService).Entries()...)
	entries = append(entries, (&orchestration.Module{Engine: m.Orchestration}).Entries()...)
	entries = append(entries, m.Core.Identity.Entries()...)
	entries = append(entries, settings.New(m.Core).Entries()...)
	entries = append(entries, dictionaries.New(m.Core).Entries()...)
	entries = append(entries, fileService.Entries()...)
	entries = append(entries, notifications.New(m.Core).Entries()...)
	entries = append(entries, m.MCP.Entries()...)
	if m.Example == nil {
		m.Example = example.New(m.Core.Pool)
	}
	entries = append(entries, m.Example.Entries()...)
	if m.Memory == nil {
		m.Memory = memory.New(m.Core.Pool)
	}
	entries = append(entries, m.Memory.Entries()...)
	entries = append(entries, m.Documents.Entries()...)
	entries = append(entries, m.Knowledge.Entries()...)
	definitions := []contracts.Definition{}
	for _, entry := range entries {
		definitions = append(definitions, entry.Definition)
	}
	sources := append([]ar.ApplicationV2{articles.RuntimeApplication()}, m.Plugins.Catalog.Applications...)
	applications, versions, err := ar.AssembleDeployment(sources, EntitiesWithPlugins(m.Plugins), definitions)
	if err != nil {
		panic(err)
	}
	for i := range entries {
		switch entries[i].Definition.Domain {
		case "articles":
			entries[i].Definition.Installation = "builtin:articles"
			entries[i].Definition.Application = "articles"
		case "projects":
			entries[i].Definition.Installation = "com.horizon.projects"
			entries[i].Definition.Application = "projects"
		}
	}
	if m.Runtime == nil {
		m.Runtime = &armodule.Module{}
	}
	m.Runtime.Entities = EntitiesWithPlugins(m.Plugins)
	m.Runtime.Applications = applications
	m.Runtime.Versions = versions
	m.Runtime.Pool = m.Core.Pool
	m.Runtime.Definitions = map[string]contracts.Definition{}
	for _, entry := range entries {
		m.Runtime.Definitions[entry.Definition.Key()] = entry.Definition
	}
	entries = append(entries, m.Runtime.Entries()...)
	m.ExecutionPlans.Definitions = map[string]contracts.Definition{}
	for _, entry := range entries {
		m.ExecutionPlans.Definitions[entry.Definition.Key()] = entry.Definition
	}
	return entries
}

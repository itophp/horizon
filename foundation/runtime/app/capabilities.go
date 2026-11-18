package app

import (
	"horizon/foundation/ai/automation"
	"horizon/foundation/ai/configuration"
	"horizon/foundation/ai/harness"
	"horizon/foundation/ai/knowledge"
	"horizon/foundation/integrations/mcp"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	runtime "horizon/foundation/kernel/workflow"
	armodule "horizon/foundation/system/applicationruntime"
	"horizon/foundation/system/dictionaries"
	"horizon/foundation/system/executionplans"
	"horizon/foundation/system/files"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/memory"
	"horizon/foundation/system/notifications"
	"horizon/foundation/system/orchestration"
	"horizon/foundation/system/settings"
)

// CapabilityModules is the shared composition root for runtime and offline
// checks. Each domain still owns its definitions; tools must not copy catalogs.
// Zero value builds declarations without opening a database or executing them.
type CapabilityModules struct {
	ExecutionPlans *executionplans.Module
	Runtime        *armodule.Module
	Automation     *automation.Module
	Registration   Registration
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
		m.Orchestration = runtime.NewWithGraphs(m.Core, m.Registration.WorkflowDefinitions(), m.Registration.Graphs)
	}
	if m.Documents == nil {
		m.Documents = &knowledge.Store{Q: m.Core.Q, Core: m.Core}
	}
	if m.Knowledge == nil {
		m.Knowledge = &knowledge.Service{Store: m.Documents}
	}
	if m.Automation == nil {
		m.Automation = automation.New(m.Core.Pool)
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

	entries = append(entries, m.Registration.Entries...)
	entries = append(entries, m.Automation.Entries()...)
	if plugins := m.Registration.Plugins; plugins != nil {
		entries = append(entries, plugins.Catalog.Entries...)
	}
	entries = append(entries, (&orchestration.Module{Engine: m.Orchestration}).Entries()...)
	entries = append(entries, m.Core.Identity.Entries()...)
	entries = append(entries, settings.New(m.Core).Entries()...)
	entries = append(entries, configuration.Entries(m.Core)...)
	entries = append(entries, dictionaries.New(m.Core).Entries()...)
	entries = append(entries, fileService.Entries()...)
	entries = append(entries, notifications.New(m.Core).Entries()...)
	entries = append(entries, m.MCP.Entries()...)
	if m.Memory == nil {
		m.Memory = memory.New(m.Core.Pool)
	}
	entries = append(entries, m.Memory.Entries()...)
	entries = append(entries, m.Documents.Entries()...)
	entries = append(entries, m.Knowledge.Entries()...)
	if m.Runtime == nil {
		m.Runtime = &armodule.Module{}
	}
	entries = append(entries, m.Runtime.AIEntries()...)
	entries = append(entries, harness.ExcelParseEntry())
	definitions := []contracts.Definition{}
	for _, entry := range entries {
		definitions = append(definitions, entry.Definition)
	}
	sources := RuntimeContributions(m.Registration)
	applications, versions, err := ar.AssembleDeployment(sources, EntitiesWithRegistration(m.Registration), definitions)
	if err != nil {
		panic(err)
	}
	for i := range entries {
		switch entries[i].Definition.Domain {
		case "automation":
			entries[i].Definition.Installation = "builtin:automation"
			entries[i].Definition.Application = "automation"
		}
	}
	m.Runtime.Entities = EntitiesWithRegistration(m.Registration)
	m.Runtime.Applications = applications
	m.Runtime.Versions = versions
	m.Runtime.Pool = m.Core.Pool
	m.Runtime.BasePlugins = BaseApplicationPlugins()
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

// RuntimeContributions is shared by server assembly and the spec generator.
func RuntimeContributions(registration Registration) []ar.ApplicationV2 {
	contributions := []ar.ApplicationV2{
		automation.RuntimeApplication(), identity.PermissionsRuntimeApplication(),
		identity.OrganizationsRuntimeApplication(), identity.AccountsRuntimeApplication(),
		identity.RuntimeProtocolApplication(), analyticsRuntimeApplication("1.0.0"), AnalyticsRuntimeApplication(),
		legacyAgentCollaborationApplication(), agentCollaborationVersion("1.0.1"), agentCollaborationApplication(),
		legacyPersonalPreferencesApplication(), personalPreferencesApplication(),
		legacySystemSettingsApplication(), systemSettingsApplication("1.0.1"), systemSettingsApplication("1.0.2"),
		systemSettingsApplication("1.0.3"), systemSettingsApplication("1.0.4"), legacyAISettingsApplication(), activeAISettingsApplication("1.0.1"), activeAISettingsApplication("1.0.2"), activeAISettingsApplication("1.0.3"),
	}
	contributions = append(contributions, registration.Applications...)
	if plugins := registration.Plugins; plugins != nil {
		contributions = append(contributions, plugins.Catalog.Applications...)
	}
	return contributions
}

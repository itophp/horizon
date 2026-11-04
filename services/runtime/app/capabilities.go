package app

import (
	"baseline/services/api/ai/knowledge"
	"baseline/services/api/applications/articles"
	"baseline/services/api/applications/drive"
	"baseline/services/api/applications/example"
	"baseline/services/api/applications/projects"
	"baseline/services/api/integrations/mcp"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/plugin"
	runtime "baseline/services/api/kernel/workflow"
	"baseline/services/api/system/dictionaries"
	"baseline/services/api/system/files"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"baseline/services/api/system/notifications"
	"baseline/services/api/system/orchestration"
	"baseline/services/api/system/settings"
)

// CapabilityModules is the shared composition root for runtime and offline
// checks. Each domain still owns its definitions; tools must not copy catalogs.
// Zero value builds declarations without opening a database or executing them.
type CapabilityModules struct {
	Articles      *articles.Module
	Projects      *projects.Module
	Plugins       *plugin.Manager
	Core          *governance.Core
	MCP           *mcp.Service
	Orchestration *runtime.Engine
	Documents     *knowledge.Store
	Knowledge     *knowledge.Service
}

func (m CapabilityModules) Entries() []registry.Entry {
	if m.Core == nil {
		m.Core = governance.New(nil, identity.New(nil, false), governance.Options{})
	}
	if m.MCP == nil {
		m.MCP = mcp.New(m.Core.Pool, m.Core.Identity)
	}
	if m.Orchestration == nil {
		m.Orchestration = runtime.New(m.Core, nil)
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
	entries := []registry.Entry{}
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
	entries = append(entries, example.New(m.Core.Pool).Entries()...)
	entries = append(entries, m.Documents.Entries()...)
	entries = append(entries, m.Knowledge.Entries()...)
	return entries
}

package app

import (
	"baseline/services/api/internal/ai/knowledge"
	runtime "baseline/services/api/internal/ai/orchestration"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/articles"
	"baseline/services/api/internal/modules/dictionaries"
	"baseline/services/api/internal/modules/example"
	"baseline/services/api/internal/modules/files"
	"baseline/services/api/internal/modules/mcp"
	"baseline/services/api/internal/modules/notifications"
	"baseline/services/api/internal/modules/orchestration"
	"baseline/services/api/internal/modules/settings"
	"baseline/services/api/internal/platform/capability/registry"
)

// CapabilityModules is the shared composition root for runtime and offline
// checks. Each domain still owns its definitions; tools must not copy catalogs.
// Zero value builds declarations without opening a database or executing them.
type CapabilityModules struct {
	Articles      *articles.Module
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
	entries := []registry.Entry{}
	entries = append(entries, m.Articles.Entries()...)
	entries = append(entries, (&orchestration.Module{Engine: m.Orchestration}).Entries()...)
	entries = append(entries, m.Core.Identity.Entries()...)
	entries = append(entries, settings.New(m.Core).Entries()...)
	entries = append(entries, dictionaries.New(m.Core).Entries()...)
	entries = append(entries, files.New(m.Core).Entries()...)
	entries = append(entries, notifications.New(m.Core).Entries()...)
	entries = append(entries, m.MCP.Entries()...)
	entries = append(entries, example.New(m.Core.Pool).Entries()...)
	entries = append(entries, m.Documents.Entries()...)
	entries = append(entries, m.Knowledge.Entries()...)
	return entries
}

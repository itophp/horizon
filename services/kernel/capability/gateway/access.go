package gateway

import (
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"sort"
	"strings"
)

func (g *Gateway) available(ctx context.Context, q *dbgen.Queries, p *identity.Principal, d contracts.Definition, lock bool) error {
	if d.Installation == "" {
		return nil
	}
	var status string
	if lock {
		row, err := q.RuntimeInstallationReadLock(ctx, dbgen.RuntimeInstallationReadLockParams{TenantID: p.TenantID, PluginID: d.Installation})
		if err != nil {
			return contracts.Problem("unavailable")
		}
		status = row.Status
	} else {
		row, err := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: d.Installation})
		if err != nil {
			return contracts.Problem("unavailable")
		}
		status = row.Status
	}
	if status != "enabled" {
		return contracts.Problem("unavailable")
	}
	if d.Application != "" {
		row, err := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: d.Application})
		if err != nil || !row.Enabled {
			return contracts.Problem("unavailable")
		}
		head, e := q.RuntimeDefinitionHead(ctx, dbgen.RuntimeDefinitionHeadParams{TenantID: p.TenantID, ApplicationID: d.Application})
		if e != nil || head.Compatibility != "compatible" || head.PublishedVersion == "" {
			return contracts.Problem("unavailable")
		}
	}
	return nil
}
func checkInputAccess(p *identity.Principal, d contracts.Definition, input contracts.Input) error {
	for _, rule := range d.InputAccess {
		if !p.Can(rule.Permission) && hasPath(input, strings.Split(rule.Path, ".")) {
			return contracts.Problem("forbidden")
		}
	}
	return nil
}
func hasPath(v any, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	switch x := v.(type) {
	case contracts.Input:
		return hasPath(map[string]any(x), parts)
	case map[string]any:
		n, ok := x[parts[0]]
		return ok && hasPath(n, parts[1:])
	case []any:
		for _, n := range x {
			if hasPath(n, parts) {
				return true
			}
		}
	}
	return false
}
func projectOutput(p *identity.Principal, d contracts.Definition, value any) (any, error) {
	if len(d.OutputAccess) == 0 {
		return value, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, contracts.Problem("internal")
	}
	var out any
	if json.Unmarshal(raw, &out) != nil {
		return nil, contracts.Problem("internal")
	}
	for _, rule := range d.OutputAccess {
		if !p.Can(rule.Permission) {
			removePath(out, strings.Split(rule.Path, "."))
		}
	}
	return out, nil
}
func removePath(v any, parts []string) {
	if len(parts) == 0 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		if len(parts) == 1 {
			delete(x, parts[0])
		} else {
			removePath(x[parts[0]], parts[1:])
		}
	case []any:
		for _, item := range x {
			removePath(item, parts)
		}
	}
}

// DefinitionsFor resolves pinned tool references, including retained versions.
// It never upgrades a requested tool to the catalog's newest version.
func (g *Gateway) DefinitionsFor(ctx context.Context, keys []string) ([]contracts.Definition, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	out := []contracts.Definition{}
	for _, key := range keys {
		name, version, ok := strings.Cut(key, "@")
		if !ok {
			return nil, contracts.Problem("invalid")
		}
		entry, ok := g.registry.Resolve(name, version)
		if !ok {
			return nil, contracts.Problem("invalid")
		}
		d := entry.Definition
		if !callerOf(p).Allowed(d.Permission) || !d.AllowsOrigin(contracts.OriginAgent) || g.available(ctx, dbgen.New(g.pool), p, d, false) != nil {
			continue
		}
		d.Input = contracts.AuthorizedSchema(d.Input, d.InputAccess, p.Can)
		if d.Input == nil {
			continue
		}
		d.Output = contracts.AuthorizedSchema(d.Output, d.OutputAccess, p.Can)
		if d.Output == nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func entrySources(entry registry.Entry, input contracts.Input) ([]contracts.Activation, error) {
	refs := []contracts.Activation{}
	if entry.Definition.Installation != "" {
		refs = append(refs, contracts.Activation{Installation: entry.Definition.Installation, Application: entry.Definition.Application})
	}
	if entry.Handler.ActivationSources != nil {
		other, err := entry.Handler.ActivationSources(input)
		if err != nil {
			return nil, err
		}
		refs = append(refs, other...)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Installation == refs[j].Installation {
			return refs[i].Application < refs[j].Application
		}
		return refs[i].Installation < refs[j].Installation
	})
	return refs, nil
}
func (g *Gateway) checkEntry(ctx context.Context, q *dbgen.Queries, p *identity.Principal, entry registry.Entry, input contracts.Input, lock bool) error {
	refs, err := entrySources(entry, input)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err = g.available(ctx, q, p, contracts.Definition{Installation: ref.Installation, Application: ref.Application}, lock); err != nil {
			return err
		}
	}
	return nil
}

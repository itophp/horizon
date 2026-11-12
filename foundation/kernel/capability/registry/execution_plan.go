package registry

import "horizon/foundation/kernel/capability/contracts"

// ResolveExecution resolves a contract and its deployment-owned Direct plan
// together. Unknown names/versions fail closed exactly like Resolve. Tenant
// overrides live in kernel/executionplan; this catalog exposes only the
// deployment fallback and never owns mutable runtime bindings.
func (r *Registry) ResolveExecution(name, version string) (Entry, contracts.ExecutionBinding, bool) {
	entry, ok := r.Resolve(name, version)
	if !ok {
		return Entry{}, contracts.ExecutionBinding{}, false
	}
	return entry, contracts.DefaultExecutionBinding(entry.Definition), true
}

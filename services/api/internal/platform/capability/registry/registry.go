// Package registry is the definition center of the platform. It holds immutable
// capability definitions and their executors in process memory; a call never
// makes a network hop to discover or authorize a capability.
package registry

import (
	"sort"
	"strconv"
	"strings"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/metadata"
)

// Version changes whenever a registered definition changes shape. Runs store it
// so a trace can be read against the catalog that produced it.
const Version = "2026-11-01.1"

// Entry binds one definition to its implementation.
type Entry struct {
	Definition contracts.Definition
	Handler    contracts.Handler
}

// Registry is an immutable catalog built at process start.
type Registry struct {
	byKey  map[string]Entry
	byName map[string][]string
	tools  map[string]string
	err    error

	declared       map[string]contracts.Entity
	entityProblems []string
}

// New validates and freezes the catalog. A catalog that cannot execute every
// declared capability is a startup error, never a request time surprise.
func New(entries []Entry) *Registry {
	r := &Registry{byKey: map[string]Entry{}, byName: map[string][]string{}, tools: map[string]string{}}
	for _, entry := range entries {
		definition := entry.Definition.Normalized()
		entry.Definition = definition
		if err := definition.Validate(); err != nil {
			r.err = contracts.Problem("invalid")
			return r
		}
		if entry.Handler.Execute == nil && entry.Handler.ExecuteTx == nil {
			r.err = contracts.Problem("invalid")
			return r
		}
		if entry.Handler.Execute != nil && entry.Handler.ExecuteTx != nil {
			r.err = contracts.Problem("invalid")
			return r
		}
		if _, exists := r.byKey[definition.Key()]; exists {
			r.err = contracts.Problem("invalid")
			return r
		}
		// Multiple versions of one capability share a tool name; different
		// capabilities must never collide.
		if existing, exists := r.tools[definition.ToolName()]; exists && existing != definition.Name {
			r.err = contracts.Problem("invalid")
			return r
		}
		r.tools[definition.ToolName()] = definition.Name
		r.byKey[definition.Key()] = entry
		r.byName[definition.Name] = append(r.byName[definition.Name], definition.Key())
	}
	for name := range r.byName {
		sort.SliceStable(r.byName[name], func(i, j int) bool {
			return newer(r.byKey[r.byName[name][i]].Definition.Version, r.byKey[r.byName[name][j]].Definition.Version)
		})
	}
	return r
}

// Err reports a catalog construction failure.
func (r *Registry) Err() error {
	if r == nil || r.err == nil {
		return nil
	}
	return r.err
}

// Resolve finds a definition by name. An empty version selects the newest
// registered version, which is what a UI button means when it does not pin one.
func (r *Registry) Resolve(name, version string) (Entry, bool) {
	if r == nil || r.err != nil {
		return Entry{}, false
	}
	keys, ok := r.byName[name]
	if !ok || len(keys) == 0 {
		return Entry{}, false
	}
	if version == "" {
		return r.byKey[keys[0]], true
	}
	entry, ok := r.byKey[name+"@"+version]
	return entry, ok
}

// All returns every definition in stable order.
func (r *Registry) All() []contracts.Definition {
	if r == nil || r.err != nil {
		return nil
	}
	out := make([]contracts.Definition, 0, len(r.byKey))
	for _, entry := range r.byKey {
		out = append(out, entry.Definition)
	}
	return contracts.SortedDefinitions(out)
}

// Discover returns the newest version of every capability the caller may use.
// Old versions stay resolvable for running instances but are not offered again.
func (r *Registry) Discover(caller contracts.Caller) []contracts.Definition {
	out := []contracts.Definition{}
	seen := map[string]bool{}
	for _, definition := range r.All() {
		if seen[definition.Name] {
			continue
		}
		seen[definition.Name] = true
		if caller.Allowed(definition.Permission) && definition.Approval != contracts.ApprovalForbidden {
			out = append(out, definition)
		}
	}
	return out
}

// RegisterEntities attaches the declared business entity model. Declarations own
// attributes, relations and states; capabilities and events stay derived from the
// capability catalog so the two can never disagree about which action touches
// which object.
func (r *Registry) RegisterEntities(entities []contracts.Entity) {
	if r == nil {
		return
	}
	if r.declared == nil {
		r.declared = map[string]contracts.Entity{}
	}
	for _, entity := range entities {
		if problems := entity.Problems(); len(problems) > 0 {
			r.err = contracts.Problem("invalid")
			r.entityProblems = append(r.entityProblems, problems...)
			continue
		}
		if _, exists := r.declared[entity.Name]; exists {
			r.err = contracts.Problem("invalid")
			r.entityProblems = append(r.entityProblems, entity.Name+": declared twice")
			continue
		}
		r.declared[entity.Name] = entity
	}
}

// UndeclaredEntities lists entity names referenced by capabilities but not yet
// described. It is the to-do list for the remaining entity declarations.
func (r *Registry) UndeclaredEntities() []string {
	if r == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, definition := range r.All() {
		for _, name := range definition.EntityNames() {
			if _, ok := r.declared[name]; !ok {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// EntityProblems lists declaration mistakes found at registration time.
func (r *Registry) EntityProblems() []string {
	if r == nil {
		return nil
	}
	return r.entityProblems
}

// Entities returns the business entity catalog: declared entities enriched with
// their capabilities and events, plus name-only entries for capabilities whose
// entity has not been described yet.
func (r *Registry) Entities() []contracts.Entity {
	if r == nil || r.err != nil {
		return nil
	}
	derived := metadata.Build(r.All())
	known := make(map[string]contracts.Entity, len(derived))
	for _, entity := range derived {
		known[entity.Name] = entity
	}
	out := make([]contracts.Entity, 0, len(derived)+len(r.declared))
	used := map[string]bool{}
	for name, entity := range r.declared {
		if source, ok := known[name]; ok {
			entity.Capabilities = source.Capabilities
			entity.Events = source.Events
			if entity.Domain == "" {
				entity.Domain = source.Domain
			}
		}
		out = append(out, entity)
		used[name] = true
	}
	for _, entity := range derived {
		if !used[entity.Name] {
			out = append(out, entity)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// EntitySet returns the known entity names for page context validation.
func (r *Registry) EntitySet() map[string]bool {
	out := map[string]bool{}
	for _, entity := range r.Entities() {
		out[entity.Name] = true
	}
	return out
}

// newer reports whether version a is newer than b using numeric segments.
func newer(a, b string) bool {
	left := strings.Split(a, ".")
	right := strings.Split(b, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		ln, le := strconv.Atoi(left[i])
		rn, re := strconv.Atoi(right[i])
		switch {
		case le == nil && re == nil && ln != rn:
			return ln > rn
		case le == nil && re != nil:
			return true
		case le != nil && re == nil:
			return false
		case left[i] != right[i]:
			return left[i] > right[i]
		}
	}
	return len(left) > len(right)
}

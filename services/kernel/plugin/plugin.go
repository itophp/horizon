// Package plugin provides the contracts for trusted, statically linked plugins.
package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
)

const APIVersion = "1.0"

type Manifest struct {
	ID           string       `json:"id" yaml:"id"`
	Version      string       `json:"version" yaml:"version"`
	APIVersion   string       `json:"apiVersion" yaml:"apiVersion"`
	Mode         string       `json:"mode" yaml:"mode"`
	Dependencies []Dependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Extensions   []string     `json:"extensions" yaml:"extensions"`
}

// Dependency pins a required builtin plugin to an exact semantic version.
type Dependency struct {
	ID      string `json:"id" yaml:"id"`
	Version string `json:"version" yaml:"version"`
}

// ParseManifest accepts a single strict YAML document (JSON is also valid YAML).
func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("invalid manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return manifest, fmt.Errorf("multiple manifest documents")
		}
		return manifest, err
	}
	return manifest, nil
}

type Plugin interface {
	Manifest() Manifest
	Register(context.Context, *Registrar) error
	Start(context.Context) error
	Stop(context.Context) error
}

// Permission describes an authorization catalog entry; registering it grants no access.
type Permission struct {
	Code              string
	Module            string
	Function          string
	Action            string
	ScopeConfigurable bool
	TemporaryEligible bool
}

type Extension struct{ PluginID, PluginVersion, Type, Name, Version string }
type Registrar struct {
	manifest    Manifest
	entries     []registry.Entry
	entities    []contracts.Entity
	permissions []Permission
	extensions  []Extension
}

func (r *Registrar) Capabilities(entries []registry.Entry) {
	for _, e := range entries {
		r.entries = append(r.entries, e)
		r.extensions = append(r.extensions, Extension{r.manifest.ID, r.manifest.Version, "capability", e.Definition.Name, e.Definition.Version})
	}
}
func (r *Registrar) Entities(entities []contracts.Entity) {
	for _, e := range entities {
		r.entities = append(r.entities, e)
		r.extensions = append(r.extensions, Extension{r.manifest.ID, r.manifest.Version, "entity", e.Name, r.manifest.Version})
	}
}
func (r *Registrar) Permissions(definitions ...Permission) {
	for _, definition := range definitions {
		r.permissions = append(r.permissions, definition)
		r.references("permission", []string{definition.Code})
	}
}
func (r *Registrar) Events(names ...string) { r.references("event", names) }
func (r *Registrar) references(kind string, names []string) {
	for _, name := range names {
		r.extensions = append(r.extensions, Extension{r.manifest.ID, r.manifest.Version, kind, name, r.manifest.Version})
	}
}

type Catalog struct {
	Entries     []registry.Entry
	Entities    []contracts.Entity
	Permissions []Permission
	Extensions  []Extension
}
type Manager struct {
	plugins    []Plugin
	started    []Plugin
	Catalog    Catalog
	registered bool
	running    bool
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]*$`)
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func New(plugins ...Plugin) *Manager { return &Manager{plugins: append([]Plugin(nil), plugins...)} }

// Register validates and stages all extensions before publishing the catalog.
func (m *Manager) Register(ctx context.Context) error {
	if m.registered {
		return fmt.Errorf("plugins are already registered")
	}
	if m.running {
		return fmt.Errorf("cannot register running plugins")
	}
	byID := map[string]Plugin{}
	states := map[string]int{}
	var ordered []Plugin
	for _, p := range m.plugins {
		if p == nil {
			return fmt.Errorf("nil plugin")
		}
		v := p.Manifest()
		if !idPattern.MatchString(v.ID) || !versionPattern.MatchString(v.Version) || v.APIVersion != APIVersion || v.Mode != "builtin" || v.Extensions == nil {
			return fmt.Errorf("invalid manifest for plugin %q", v.ID)
		}
		if _, ok := byID[v.ID]; ok {
			return fmt.Errorf("duplicate plugin %q", v.ID)
		}
		byID[v.ID] = p
	}
	var visit func(string) error
	visit = func(id string) error {
		if states[id] == 1 {
			return fmt.Errorf("plugin dependency cycle at %q", id)
		}
		if states[id] == 2 {
			return nil
		}
		p, ok := byID[id]
		if !ok {
			return fmt.Errorf("missing plugin dependency %q", id)
		}
		states[id] = 1
		deps := append([]Dependency(nil), p.Manifest().Dependencies...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].ID < deps[j].ID })
		seenDeps := map[string]bool{}
		for _, dep := range deps {
			if seenDeps[dep.ID] || !versionPattern.MatchString(dep.Version) {
				return fmt.Errorf("invalid dependency %q of %s", dep.ID, id)
			}
			seenDeps[dep.ID] = true
			required, ok := byID[dep.ID]
			if !ok {
				return fmt.Errorf("missing plugin dependency %q", dep.ID)
			}
			if required.Manifest().Version != dep.Version {
				return fmt.Errorf("plugin %s requires %s@%s", id, dep.ID, dep.Version)
			}
			if err := visit(dep.ID); err != nil {
				return err
			}
		}
		states[id] = 2
		ordered = append(ordered, p)
		return nil
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	var staged Catalog
	seen := map[string]bool{}
	for _, p := range ordered {
		r := &Registrar{manifest: p.Manifest()}
		allowed := map[string]bool{}
		for _, kind := range r.manifest.Extensions {
			if kind != "capability" && kind != "entity" && kind != "permission" && kind != "event" {
				return fmt.Errorf("unknown extension type %q", kind)
			}
			if allowed[kind] {
				return fmt.Errorf("duplicate extension type %q", kind)
			}
			allowed[kind] = true
		}
		if err := p.Register(ctx, r); err != nil {
			return fmt.Errorf("register plugin %s: %w", p.Manifest().ID, err)
		}
		for _, e := range r.extensions {
			if !allowed[e.Type] {
				return fmt.Errorf("plugin %s did not declare %s extensions", r.manifest.ID, e.Type)
			}
			key := e.Type + ":" + e.Name
			if e.Type == "capability" {
				key += "@" + e.Version
			}
			if e.Name == "" || seen[key] {
				return fmt.Errorf("duplicate or empty extension %s", key)
			}
			seen[key] = true
		}
		for _, permission := range r.permissions {
			if permission.Code == "" || permission.Module == "" || permission.Function == "" || permission.Action == "" || permission.Code != permission.Function+":"+permission.Action {
				return fmt.Errorf("plugin %s has invalid permission %q", r.manifest.ID, permission.Code)
			}
		}
		// Report the offending declaration rather than only the generic catalog error.
		for _, entry := range r.entries {
			if err := entry.Definition.Normalized().Validate(); err != nil {
				return fmt.Errorf("plugin %s capability %s@%s: %w", r.manifest.ID, entry.Definition.Name, entry.Definition.Version, err)
			}
			if (entry.Handler.Execute == nil) == (entry.Handler.ExecuteTx == nil) {
				return fmt.Errorf("plugin %s capability %s must have exactly one executor", r.manifest.ID, entry.Definition.Name)
			}
		}
		for _, entity := range r.entities {
			if problems := entity.Problems(); len(problems) > 0 {
				return fmt.Errorf("plugin %s entity %s: %s", r.manifest.ID, entity.Name, strings.Join(problems, "; "))
			}
		}
		// Permission and event references must exactly reflect the capability source.
		needed, declared := map[string]bool{}, map[string]bool{}
		for _, entry := range r.entries {
			needed["permission:"+entry.Definition.Permission] = true
			for _, event := range entry.Definition.SuccessEvents {
				needed["event:"+event] = true
			}
		}
		for _, ext := range r.extensions {
			if ext.Type == "event" || ext.Type == "permission" {
				declared[ext.Type+":"+ext.Name] = true
			}
		}
		for key := range needed {
			if !declared[key] {
				return fmt.Errorf("plugin %s missing %s reference", r.manifest.ID, key)
			}
		}
		for key := range declared {
			if !needed[key] {
				return fmt.Errorf("plugin %s has unused %s reference", r.manifest.ID, key)
			}
		}
		staged.Entries = append(staged.Entries, r.entries...)
		staged.Entities = append(staged.Entities, r.entities...)
		staged.Permissions = append(staged.Permissions, r.permissions...)
		staged.Extensions = append(staged.Extensions, r.extensions...)
	}
	cat := registry.New(staged.Entries)
	if err := cat.Err(); err != nil {
		return fmt.Errorf("invalid plugin capability catalog: %w", err)
	}
	cat.RegisterEntities(staged.Entities)
	if err := cat.Err(); err != nil {
		return fmt.Errorf("invalid plugin entities: %w", err)
	}
	m.plugins = ordered
	m.Catalog = staged
	m.registered = true
	return nil
}
func (m *Manager) Start(ctx context.Context) error {
	if !m.registered {
		return fmt.Errorf("plugins are not registered")
	}
	if m.running {
		return fmt.Errorf("plugins are already running")
	}
	m.running = true
	for _, p := range m.plugins {
		if err := p.Start(ctx); err != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			stopErr := m.Stop(cleanup)
			cancel()
			return errors.Join(fmt.Errorf("start plugin %s: %w", p.Manifest().ID, err), stopErr)
		}
		m.started = append(m.started, p)
	}
	return nil
}
func (m *Manager) Stop(ctx context.Context) error {
	var first error
	for i := len(m.started) - 1; i >= 0; i-- {
		if err := m.started[i].Stop(ctx); err != nil && first == nil {
			first = err
		}
	}
	m.started = nil
	m.running = false
	return first
}

// Package plugin provides the contracts for trusted, statically linked plugins.
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
)

const APIVersion = "1.0"

type Manifest struct {
	ID         string `json:"id" yaml:"id"`
	Version    string `json:"version" yaml:"version"`
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Mode       string `json:"mode" yaml:"mode"`
	// Installation is the runtime application-package key this plugin
	// publishes under. It defaults to the plugin ID. Legacy packages pin
	// their existing key so installed tenants keep resolving the same
	// installation without a data migration.
	Installation string `json:"installation,omitempty" yaml:"installation,omitempty"`
	// Managed marks the contributed application as deployment-baseline
	// managed: every tenant receives it and tenant lifecycle stays forbidden.
	Managed      bool         `json:"managed,omitempty" yaml:"managed,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Extensions   []string     `json:"extensions" yaml:"extensions"`
	Permissions  []Permission `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	FilePurposes []string     `json:"filePurposes,omitempty" yaml:"filePurposes,omitempty"`
	// Settings declares configuration keys owned by the plugin's business
	// domain. Keys must be prefixed by one of the declared permission modules.
	Settings []Setting `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// Setting describes a configuration key contributed by a plugin. The value
// grammar mirrors the platform registry: Default is JSON encoded.
type Setting struct {
	Key       string   `json:"key" yaml:"key"`
	Group     string   `json:"group" yaml:"group"`
	Kind      string   `json:"kind" yaml:"kind"`
	Default   string   `json:"default" yaml:"default"`
	Scopes    []string `json:"scopes" yaml:"scopes"`
	Choices   []string `json:"choices,omitempty" yaml:"choices,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty" yaml:"sensitive,omitempty"`
	Secret    bool     `json:"secret,omitempty" yaml:"secret,omitempty"`
	UI        bool     `json:"ui" yaml:"ui"`
	Min       int64    `json:"min,omitempty" yaml:"min,omitempty"`
	Max       int64    `json:"max,omitempty" yaml:"max,omitempty"`
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
	Code              string `json:"code" yaml:"code"`
	Module            string `json:"module" yaml:"module"`
	Function          string `json:"function" yaml:"function"`
	Action            string `json:"action" yaml:"action"`
	ScopeConfigurable bool   `json:"scopeConfigurable" yaml:"scopeConfigurable"`
	TemporaryEligible bool   `json:"temporaryEligible" yaml:"temporaryEligible"`
}

type Extension struct{ PluginID, PluginVersion, Type, Name, Version string }
type Registrar struct {
	applications []ar.ApplicationV2
	manifest     Manifest
	entries      []registry.Entry
	entities     []contracts.Entity
	permissions  []Permission
	settings     []Setting
	failures     []error
	extensions   []Extension
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
	Applications []ar.ApplicationV2
	Entries      []registry.Entry
	Entities     []contracts.Entity
	Permissions  []Permission
	Extensions   []Extension
	// FilePurposes lists plugin-declared file purposes merged at startup.
	FilePurposes []string
	// Managed lists installation keys of deployment-baseline-managed packages.
	Managed []string
	// Settings lists plugin-declared configuration keys merged at startup.
	Settings []Setting
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
var installPattern = regexp.MustCompile(`^[a-z][a-z0-9.:-]*$`)
var purposePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

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
		if v.Installation != "" && !installPattern.MatchString(v.Installation) {
			return fmt.Errorf("invalid installation key for plugin %q", v.ID)
		}
		for _, purpose := range v.FilePurposes {
			if !purposePattern.MatchString(purpose) {
				return fmt.Errorf("invalid file purpose %q for plugin %q", purpose, v.ID)
			}
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
			if kind != "capability" && kind != "entity" && kind != "permission" && kind != "event" && kind != "application" && kind != "file-purpose" && kind != "settings" {
				return fmt.Errorf("unknown extension type %q", kind)
			}
			if allowed[kind] {
				return fmt.Errorf("duplicate extension type %q", kind)
			}
			allowed[kind] = true
		}
		if len(r.manifest.FilePurposes) > 0 && !allowed["file-purpose"] {
			return fmt.Errorf("plugin %s declares file purposes without the file-purpose extension", r.manifest.ID)
		}
		// Manifest-declared permissions and settings enter the registrar
		// before Register so capability references validate against the
		// declarative catalog.
		r.Permissions(r.manifest.Permissions...)
		if err := r.manifestSettings(); err != nil {
			return err
		}
		if err := p.Register(ctx, r); err != nil {
			return fmt.Errorf("register plugin %s: %w", p.Manifest().ID, err)
		}
		for _, e := range r.extensions {
			if !allowed[e.Type] {
				return fmt.Errorf("plugin %s did not declare %s extensions", r.manifest.ID, e.Type)
			}
			key := e.Type + ":" + e.Name
			if e.Type == "capability" || e.Type == "application" {
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
		installKey := r.manifest.Installation
		if installKey == "" {
			installKey = r.manifest.ID
		}
		appIDs := map[string]bool{}
		for _, application := range r.applications {
			if application.Source != installKey {
				return fmt.Errorf("plugin %s has invalid application ownership: %q is not the installation key", r.manifest.ID, application.Source)
			}
			appIDs[application.ID] = true
		}
		// Capabilities of a plugin that publishes an application package belong
		// to that installation; the stable keys stay deployment data.
		if allowed["application"] {
			for i := range r.entries {
				r.entries[i].Definition.Installation = installKey
				if len(appIDs) == 1 {
					for id := range appIDs {
						r.entries[i].Definition.Application = id
					}
				}
			}
		}
		staged.Entries = append(staged.Entries, r.entries...)
		staged.Entities = append(staged.Entities, r.entities...)
		staged.Applications = append(staged.Applications, r.applications...)
		seenPermissions := map[string]bool{}
		for _, code := range staged.Permissions {
			seenPermissions[code.Code] = true
		}
		for _, permission := range r.permissions {
			if seenPermissions[permission.Code] {
				return fmt.Errorf("plugin %s declares permission %q more than once", r.manifest.ID, permission.Code)
			}
			seenPermissions[permission.Code] = true
			staged.Permissions = append(staged.Permissions, permission)
		}
		for _, purpose := range r.manifest.FilePurposes {
			extension := Extension{r.manifest.ID, r.manifest.Version, "file-purpose", purpose, r.manifest.Version}
			key := extension.Type + ":" + extension.Name
			if seen[key] {
				return fmt.Errorf("duplicate or empty extension %s", key)
			}
			seen[key] = true
			staged.Extensions = append(staged.Extensions, extension)
			staged.FilePurposes = append(staged.FilePurposes, purpose)
		}
		if len(r.failures) > 0 {
			return r.failures[0]
		}
		seenSettings := map[string]bool{}
		for _, def := range staged.Settings {
			seenSettings[def.Key] = true
		}
		for _, def := range r.settings {
			if seenSettings[def.Key] {
				return fmt.Errorf("plugin %s declares setting %q more than once", r.manifest.ID, def.Key)
			}
			seenSettings[def.Key] = true
			staged.Settings = append(staged.Settings, def)
		}
		if len(r.manifest.Settings) > 0 && !allowed["settings"] {
			return fmt.Errorf("plugin %s declares settings without the settings extension", r.manifest.ID)
		}
		if r.manifest.Managed {
			staged.Managed = append(staged.Managed, installKey)
		}
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

// AttachCapabilities supplies the execution dependency after Gateway construction.
func (m *Manager) AttachCapabilities(invoke func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error)) {
	for _, p := range m.plugins {
		if consumer, ok := p.(interface {
			AttachCapabilities(func(context.Context, string, string, contracts.Input, string) (*contracts.Result, error))
		}); ok {
			consumer.AttachCapabilities(invoke)
		}
	}
}

// AttachModel supplies the tool-free model interface to plugins that extract structured data.
func (m *Manager) AttachModel(generate func(context.Context, string, string) (string, error)) {
	for _, p := range m.plugins {
		if consumer, ok := p.(interface {
			AttachModel(func(context.Context, string, string) (string, error))
		}); ok {
			consumer.AttachModel(generate)
		}
	}
}

// Plugins exposes the registered plugin instances so composition layers can
// adapt optional capabilities (gateway attachment, background workers) with
// type assertions. The plugin package itself stays free of transport and AI
// dependencies.
func (m *Manager) Plugins() []Plugin { return m.plugins }

// manifestSettings seeds registrar state from the manifest declaration.
func (r *Registrar) manifestSettings() error {
	r.Settings(r.manifest.Settings...)
	return nil
}

// Settings registers plugin-owned configuration keys. The registrar validates
// that each key lives under a permission module the manifest declares, so a
// plugin can only own settings inside its own business domain.
func (r *Registrar) Settings(defs ...Setting) {
	if len(defs) == 0 {
		return
	}
	modules := map[string]bool{}
	for _, permission := range r.manifest.Permissions {
		modules[permission.Module] = true
	}
	for _, def := range defs {
		domain := def.Key
		if dot := strings.Index(def.Key, "."); dot > 0 {
			domain = def.Key[:dot]
		}
		if domain == "" || def.Key == domain {
			r.failures = append(r.failures, fmt.Errorf("setting key %q needs a domain prefix", def.Key))
			continue
		}
		if !modules[domain] {
			r.failures = append(r.failures, fmt.Errorf("setting %q is outside the declared modules of %s", def.Key, r.manifest.ID))
			continue
		}
		if json.Valid([]byte(def.Default)) == false {
			r.failures = append(r.failures, fmt.Errorf("setting %q has a non-JSON default", def.Key))
			continue
		}
		r.settings = append(r.settings, def)
		r.extensions = append(r.extensions, Extension{r.manifest.ID, r.manifest.Version, "settings", def.Key, r.manifest.Version})
	}
}

func (r *Registrar) Applications(apps ...ar.ApplicationV2) {
	for _, app := range apps {
		if app.PluginVersion == "" {
			app.PluginVersion = r.manifest.Version
		}
		if len(app.PluginDependencies) == 0 {
			for _, dep := range r.manifest.Dependencies {
				app.PluginDependencies = append(app.PluginDependencies, ar.PluginDependency{ID: dep.ID, Version: dep.Version})
			}
		}
		r.applications = append(r.applications, app)
		r.extensions = append(r.extensions, Extension{r.manifest.ID, r.manifest.Version, "application", app.ID, app.Version})
	}
}

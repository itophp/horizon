package plugin

import (
	"context"
	"strings"
	"testing"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
)

// declPlugin drives the declarative manifest features: permissions and file
// purposes live in the manifest, capabilities and applications stay in Go.
type declPlugin struct {
	manifest  Manifest
	entries   []registry.Entry
	apps      []ar.ApplicationV2
	events    []string
	permCodes []string
}

func (p declPlugin) Manifest() Manifest { return p.manifest }
func (p declPlugin) Register(_ context.Context, r *Registrar) error {
	if len(p.entries) > 0 {
		r.Capabilities(p.entries)
	}
	if len(p.apps) > 0 {
		r.Applications(p.apps...)
	}
	r.Events(p.events...)
	for _, code := range p.permCodes {
		function, action := code, ""
		if split := strings.Split(code, ":"); len(split) == 2 {
			function, action = split[0], split[1]
		}
		r.Permissions(Permission{Code: code, Module: function, Function: function, Action: action})
	}
	return nil
}
func (declPlugin) Start(context.Context) error { return nil }
func (declPlugin) Stop(context.Context) error  { return nil }

func declaredManifest(extra Manifest) Manifest {
	manifest := extra
	if manifest.ID == "" {
		manifest.ID = "test.plugin"
	}
	manifest.Version, manifest.APIVersion, manifest.Mode = "1.0.0", APIVersion, "builtin"
	if manifest.Extensions == nil {
		manifest.Extensions = []string{"event"}
	}
	return manifest
}

func readOnlyEntry(name, permission string) registry.Entry {
	return registry.Entry{
		Definition: contracts.Definition{
			Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0",
			Title: name, Description: "Declarative manifest test capability.",
			Domain: "test", Entity: "widget", Permission: permission,
			Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
			Executor: "go:" + name,
			Input:    contracts.Object(map[string]*contracts.Schema{}),
			Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		},
		Handler: contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) {
			return contracts.Output{}, nil
		}},
	}
}

func TestManifestPermissionSatisfiesCapabilityReference(t *testing.T) {
	manifest := declaredManifest(Manifest{
		Extensions:  []string{"capability", "permission"},
		Permissions: []Permission{{Code: "widget:read", Module: "test", Function: "widget", Action: "read"}},
	})
	plugin := declPlugin{manifest: manifest, entries: []registry.Entry{readOnlyEntry("widget.read", "widget:read")}}
	manager := New(plugin)
	if err := manager.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(manager.Catalog.Permissions) != 1 || manager.Catalog.Permissions[0].Code != "widget:read" {
		t.Fatalf("manifest permission missing from catalog: %v", manager.Catalog.Permissions)
	}
}

func TestManifestPermissionWithoutCapabilityIsRejected(t *testing.T) {
	manifest := declaredManifest(Manifest{
		Extensions:  []string{"permission"},
		Permissions: []Permission{{Code: "widget:read", Module: "test", Function: "widget", Action: "read"}},
	})
	if err := New(declPlugin{manifest: manifest}).Register(context.Background()); err == nil {
		t.Fatal("accepted a manifest permission no capability references")
	}
}

func TestCapabilityPermissionRequiresDeclaration(t *testing.T) {
	manifest := declaredManifest(Manifest{Extensions: []string{"capability"}})
	err := New(declPlugin{manifest: manifest, entries: []registry.Entry{readOnlyEntry("widget.read", "widget:read")}}).Register(context.Background())
	if err == nil || !strings.Contains(err.Error(), "permission:widget:read") {
		t.Fatalf("expected missing permission reference, got %v", err)
	}
}

func TestPermissionDeclaredTwiceIsRejected(t *testing.T) {
	manifest := declaredManifest(Manifest{
		Extensions:  []string{"permission"},
		Permissions: []Permission{{Code: "widget:read", Module: "test", Function: "widget", Action: "read"}},
	})
	plugin := declPlugin{manifest: manifest, permCodes: []string{"widget:read"}}
	if err := New(plugin).Register(context.Background()); err == nil {
		t.Fatal("accepted the same permission from manifest and registrar")
	}
}

func TestPermissionClashAcrossPluginsIsRejected(t *testing.T) {
	manifest := declaredManifest(Manifest{
		Extensions:  []string{"permission"},
		Permissions: []Permission{{Code: "widget:read", Module: "test", Function: "widget", Action: "read"}},
	})
	if err := New(declPlugin{manifest: manifest}, declPlugin{manifest: manifest}).Register(context.Background()); err == nil {
		t.Fatal("accepted the same permission from two plugins")
	}
}

func TestFilePurposesMergeIntoCatalog(t *testing.T) {
	manifest := declaredManifest(Manifest{Extensions: []string{"file-purpose"}, FilePurposes: []string{"widget_export"}})
	manager := New(declPlugin{manifest: manifest})
	if err := manager.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(manager.Catalog.FilePurposes) != 1 || manager.Catalog.FilePurposes[0] != "widget_export" {
		t.Fatalf("file purpose missing from catalog: %v", manager.Catalog.FilePurposes)
	}
}

func TestFilePurposeValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		purposes []string
		wantErr  bool
	}{
		{"valid", []string{"widget_export"}, false},
		{"invalid pattern", []string{"Widget-Export"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := declaredManifest(Manifest{Extensions: []string{"file-purpose"}, FilePurposes: tc.purposes})
			err := New(declPlugin{manifest: manifest}).Register(context.Background())
			if tc.wantErr != (err != nil) {
				t.Fatalf("purpose %v: unexpected error %v", tc.purposes, err)
			}
		})
	}
	manifest := declaredManifest(Manifest{FilePurposes: []string{"widget_export"}})
	if err := New(declPlugin{manifest: manifest}).Register(context.Background()); err == nil {
		t.Fatal("accepted file purposes without the file-purpose extension")
	}
	first := declaredManifest(Manifest{Extensions: []string{"file-purpose"}, FilePurposes: []string{"widget_export"}})
	if err := New(declPlugin{manifest: first}, declPlugin{manifest: first}).Register(context.Background()); err == nil {
		t.Fatal("accepted the same file purpose from two plugins")
	}
}

func TestInstallationKeyAndManagedClassification(t *testing.T) {
	manifest := declaredManifest(Manifest{
		Extensions:   []string{"capability", "application", "permission"},
		Installation: "legacy:widget",
		Managed:      true,
		Permissions:  []Permission{{Code: "widget:read", Module: "test", Function: "widget", Action: "read"}},
	})
	plugin := declPlugin{
		manifest: manifest,
		entries:  []registry.Entry{readOnlyEntry("widget.read", "widget:read")},
		apps:     []ar.ApplicationV2{{ID: "widget", Version: "1.0.0", Source: "legacy:widget"}},
	}
	manager := New(plugin)
	if err := manager.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	entry := manager.Catalog.Entries[0]
	if entry.Definition.Installation != "legacy:widget" || entry.Definition.Application != "widget" {
		t.Fatalf("installation derivation failed: %+v", entry.Definition)
	}
	if len(manager.Catalog.Managed) != 1 || manager.Catalog.Managed[0] != "legacy:widget" {
		t.Fatalf("managed classification missing: %v", manager.Catalog.Managed)
	}

	manifest.Installation = "other:key"
	if err := New(declPlugin{
		manifest: manifest,
		entries:  []registry.Entry{readOnlyEntry("widget.read", "widget:read")},
		apps:     []ar.ApplicationV2{{ID: "widget", Version: "1.0.0", Source: "legacy:widget"}},
	}).Register(context.Background()); err == nil {
		t.Fatal("accepted an application published under a foreign installation key")
	}
}

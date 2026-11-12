package plugin

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type testPlugin struct {
	id                      string
	deps                    []Dependency
	log                     *[]string
	failRegister, failStart bool
}

func (p testPlugin) Manifest() Manifest {
	return Manifest{ID: p.id, Version: "1.0.0", APIVersion: APIVersion, Mode: "builtin", Dependencies: p.deps, Extensions: []string{"event"}}
}
func (p testPlugin) Register(_ context.Context, r *Registrar) error {
	if p.failRegister {
		return errors.New("registration failed")
	}
	// A plugin with no capabilities can still register no-op extensions.
	return nil
}
func (p testPlugin) Start(context.Context) error {
	*p.log = append(*p.log, "start "+p.id)
	if p.failStart {
		return errors.New("start failed")
	}
	return nil
}
func (p testPlugin) Stop(context.Context) error { *p.log = append(*p.log, "stop "+p.id); return nil }
func TestLifecycleAndRegistrationRollback(t *testing.T) {
	ctx := context.Background()
	var log []string
	a := testPlugin{id: "a", log: &log}
	b := testPlugin{id: "b", deps: []Dependency{{ID: "a", Version: "1.0.0"}}, log: &log, failStart: true}
	m := New(b, a)
	if err := m.Register(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err == nil {
		t.Fatal("expected start failure")
	}
	if !reflect.DeepEqual(log, []string{"start a", "start b", "stop a"}) {
		t.Fatal(log)
	}
	m = New(a, testPlugin{id: "bad", log: &log, failRegister: true})
	if err := m.Register(ctx); err == nil || len(m.Catalog.Extensions) != 0 {
		t.Fatal("failed registration published extensions")
	}
}
func TestInvalidDependencies(t *testing.T) {
	var log []string
	for _, plugins := range [][]Plugin{{testPlugin{id: "a", deps: []Dependency{{ID: "missing", Version: "1.0.0"}}, log: &log}}, {testPlugin{id: "a", deps: []Dependency{{ID: "b", Version: "1.0.0"}}, log: &log}, testPlugin{id: "b", deps: []Dependency{{ID: "a", Version: "1.0.0"}}, log: &log}}, {testPlugin{id: "a", log: &log}, testPlugin{id: "a", log: &log}}} {
		if New(plugins...).Register(context.Background()) == nil {
			t.Fatal("expected invalid plugin set")
		}
	}
}

func TestManifestAndLifecycleValidation(t *testing.T) {
	ctx := context.Background()
	for _, input := range []string{
		"id: demo\nunknown: true\n", "id: demo\n---\nid: other\n",
	} {
		if _, err := ParseManifest([]byte(input)); err == nil {
			t.Fatalf("accepted invalid manifest: %q", input)
		}
	}
	var log []string
	a := testPlugin{id: "a", log: &log}
	b := testPlugin{id: "b", deps: []Dependency{{ID: "a", Version: "2.0.0"}}, log: &log}
	if err := New(a, b).Register(ctx); err == nil {
		t.Fatal("accepted incompatible dependency version")
	}
	m := New(a)
	if m.Start(ctx) == nil {
		t.Fatal("started before registration")
	}
	if err := m.Register(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if m.Start(ctx) == nil || m.Register(ctx) == nil {
		t.Fatal("accepted repeated startup or registration while running")
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if m.Register(ctx) == nil {
		t.Fatal("accepted re-registration after stop")
	}
}

type invalidPlugin struct {
	manifest Manifest
	register func(*Registrar)
}

func (p invalidPlugin) Manifest() Manifest                             { return p.manifest }
func (p invalidPlugin) Register(_ context.Context, r *Registrar) error { p.register(r); return nil }
func (invalidPlugin) Start(context.Context) error                      { return nil }
func (invalidPlugin) Stop(context.Context) error                       { return nil }

func TestExtensionValidationIsAtomic(t *testing.T) {
	valid := Manifest{ID: "test.plugin", Version: "1.0.0", APIVersion: APIVersion, Mode: "builtin", Extensions: []string{"event"}}
	for _, tc := range []struct {
		name     string
		manifest Manifest
		register func(*Registrar)
	}{
		{"undeclared type", valid, func(r *Registrar) {
			r.Permissions(Permission{Code: "x:read", Module: "test", Function: "x", Action: "read"})
		}},
		{"unused reference", valid, func(r *Registrar) { r.Events("event.created") }},
		{"duplicate", valid, func(r *Registrar) { r.Events("event.created", "event.created") }},
		{"incompatible API", Manifest{ID: "bad", Version: "1.0.0", APIVersion: "2.0", Mode: "builtin", Extensions: []string{}}, func(*Registrar) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(invalidPlugin{manifest: tc.manifest, register: tc.register})
			if err := m.Register(context.Background()); err == nil {
				t.Fatal("invalid extension accepted")
			}
			if len(m.Catalog.Extensions) != 0 {
				t.Fatal("partial registration published")
			}
		})
	}
}

func TestDependencyShutdownOrder(t *testing.T) {
	var log []string
	a := testPlugin{id: "a", log: &log}
	b := testPlugin{id: "b", deps: []Dependency{{ID: "a", Version: "1.0.0"}}, log: &log}
	plugins := []Plugin{b, a}
	manager := New(plugins...)
	// Mutating the caller's list cannot replace a registered implementation.
	plugins[0] = testPlugin{id: "other", log: &log}
	if err := manager.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"start a", "start b", "stop b", "stop a"}; !reflect.DeepEqual(log, want) {
		t.Fatalf("got %v, want %v", log, want)
	}
}

package registry

import (
	"context"
	"strings"
	"testing"

	"baseline/services/api/internal/platform/capability/contracts"
)

func definition(name, version string) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: name, Version: version, Title: name, Description: name,
		Domain: "example", Entity: "example_record", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:test", Input: contracts.Object(nil),
	}
}

func handler() contracts.Handler {
	return contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) {
		return contracts.Output{}, nil
	}}
}

func TestNewRejectsInvalidCatalogs(t *testing.T) {
	valid := definition("example.record.list", "1.0.0")
	if err := New([]Entry{{Definition: valid, Handler: handler()}}).Err(); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
	if err := New([]Entry{{Definition: valid}}).Err(); err == nil {
		t.Fatal("entry without a handler must not register")
	}
	if err := New([]Entry{
		{Definition: valid, Handler: handler()},
		{Definition: valid, Handler: handler()},
	}).Err(); err == nil {
		t.Fatal("duplicate key must not register")
	}
	collision := definition("example_record_list", "1.0.0")
	if err := New([]Entry{
		{Definition: valid, Handler: handler()},
		{Definition: collision, Handler: handler()},
	}).Err(); err == nil {
		t.Fatal("tool name collision must not register")
	}
}

func TestResolveSelectsNewestVersion(t *testing.T) {
	registry := New([]Entry{
		{Definition: definition("example.record.list", "1.0.0"), Handler: handler()},
		{Definition: definition("example.record.list", "2.0.0"), Handler: handler()},
		{Definition: definition("example.record.list", "1.10.0"), Handler: handler()},
	})
	if registry.Err() != nil {
		t.Fatal(registry.Err())
	}
	entry, ok := registry.Resolve("example.record.list", "")
	if !ok || entry.Definition.Version != "2.0.0" {
		t.Fatalf("newest version expected, got %+v", entry.Definition.Version)
	}
	if entry, ok := registry.Resolve("example.record.list", "1.10.0"); !ok || entry.Definition.Version != "1.10.0" {
		t.Fatal("exact version must resolve")
	}
	if _, ok := registry.Resolve("example.record.list", "9.9.9"); ok {
		t.Fatal("unknown version must not resolve")
	}
	if _, ok := registry.Resolve("missing", ""); ok {
		t.Fatal("unknown capability must not resolve")
	}
}

func TestDiscoverFiltersByPermission(t *testing.T) {
	read := definition("example.record.list", "1.0.0")
	write := definition("example.record.archive", "1.0.0")
	write.Permission = "example:archive"
	write.ReadOnly = false
	write.Idempotency = contracts.Idempotency{Required: true}
	write.SuccessEvents = []string{"example.record.archived"}
	registry := New([]Entry{
		{Definition: read, Handler: handler()},
		{Definition: write, Handler: handler()},
	})
	discovered := registry.Discover(contracts.Caller{Grants: []string{"example:read"}})
	if len(discovered) != 1 || discovered[0].Name != read.Name {
		t.Fatalf("unexpected discovery %+v", discovered)
	}
	if all := registry.Discover(contracts.Caller{Admin: true}); len(all) != 2 {
		t.Fatalf("administrator must see both, got %d", len(all))
	}
}

func TestEntitiesAreDerived(t *testing.T) {
	create := definition("example.record.create", "1.0.0")
	create.ReadOnly = false
	create.Idempotency = contracts.Idempotency{Required: true}
	create.SuccessEvents = []string{"example.record.created"}
	registry := New([]Entry{{Definition: create, Handler: handler()}})
	entities := registry.Entities()
	if len(entities) != 1 {
		t.Fatalf("expected one entity, got %+v", entities)
	}
	entity := entities[0]
	if entity.Name != "example_record" || len(entity.Capabilities) != 1 || len(entity.Events) != 1 {
		t.Fatalf("unexpected entity metadata %+v", entity)
	}
	if !registry.EntitySet()["example_record"] {
		t.Fatal("entity set must contain the entity")
	}
}

func TestRegisterEntitiesMergesDerivedCapabilities(t *testing.T) {
	catalog := New([]Entry{{Definition: definition("example.record.list", "1.0.0"), Handler: handler()}})
	catalog.RegisterEntities([]contracts.Entity{{
		Name: "example_record", Table: "example_records", Key: "id",
		Attributes: []contracts.Attribute{{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger}},
	}})
	if err := catalog.Err(); err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}
	entities := catalog.Entities()
	if len(entities) != 1 {
		t.Fatalf("entities = %v", entities)
	}
	entity := entities[0]
	if entity.Table != "example_records" {
		t.Fatalf("declaration lost: %+v", entity)
	}
	if len(entity.Capabilities) != 1 || entity.Capabilities[0] != "example.record.list" {
		t.Fatalf("capabilities not derived: %v", entity.Capabilities)
	}
	if entity.Domain != "example" {
		t.Fatalf("domain not derived: %q", entity.Domain)
	}
}

func TestRegisterEntitiesRejectsBadDeclarations(t *testing.T) {
	catalog := New([]Entry{{Definition: definition("example.record.list", "1.0.0"), Handler: handler()}})
	catalog.RegisterEntities([]contracts.Entity{
		{Name: "broken", Table: "example_records", Attributes: []contracts.Attribute{{Name: "x", Kind: "nonsense", Type: contracts.TypeString}}},
		{Name: "dupe", Table: "example_records"},
		{Name: "dupe", Table: "example_records"},
	})
	if err := catalog.Err(); err == nil {
		t.Fatal("invalid declarations must fail the catalog")
	}
	if len(catalog.EntityProblems()) < 2 {
		t.Fatalf("expected problems to be reported: %v", catalog.EntityProblems())
	}
}

func TestEntitiesKeepsUndeclaredNames(t *testing.T) {
	catalog := New([]Entry{{Definition: definition("example.record.list", "1.0.0"), Handler: handler()}})
	entities := catalog.Entities()
	if len(entities) != 1 || entities[0].Name != "example_record" {
		t.Fatalf("undeclared entity should still be listed: %v", entities)
	}
	if entities[0].Table != "" {
		t.Fatalf("underived entity must not invent a table: %+v", entities[0])
	}
}

func TestUndeclaredEntitiesUsesEveryEntityOfACapability(t *testing.T) {
	multi := definition("example.record.touch", "1.0.0")
	multi.Entity = "example_record"
	multi.Entities = []string{"example_record", "file"}
	catalog := New([]Entry{{Definition: multi, Handler: handler()}})
	if got := strings.Join(catalog.UndeclaredEntities(), ","); got != "example_record,file" {
		t.Fatalf("undeclared = %q", got)
	}
	catalog.RegisterEntities([]contracts.Entity{{Name: "example_record", Table: "example_records", Key: "id"}})
	if got := strings.Join(catalog.UndeclaredEntities(), ","); got != "file" {
		t.Fatalf("undeclared after declaration = %q", got)
	}
}

func TestRegisterEntitiesKeepsMultiEntityCapabilitiesReachable(t *testing.T) {
	multi := definition("example.record.touch", "1.0.0")
	multi.Entity = "example_record"
	multi.Entities = []string{"example_record", "file"}
	catalog := New([]Entry{{Definition: multi, Handler: handler()}})
	catalog.RegisterEntities([]contracts.Entity{
		{Name: "example_record", Table: "example_records", Key: "id"},
		{Name: "file", Table: "gov_files", Key: "id"},
	})
	entities := catalog.Entities()
	if len(entities) != 2 {
		t.Fatalf("entities = %v", entities)
	}
	for _, entity := range entities {
		if len(entity.Capabilities) != 1 || entity.Capabilities[0] != "example.record.touch" {
			t.Fatalf("%s capabilities = %v", entity.Name, entity.Capabilities)
		}
	}
}

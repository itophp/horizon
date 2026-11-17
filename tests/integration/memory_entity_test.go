//go:build integration

package tests

import (
	"context"
	"testing"

	"horizon/applications/example"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/kernel/outbox"
	"horizon/foundation/system/memory"
)

func TestEntityMemoryReusesResourceAuthorization(t *testing.T) {
	h := newProjectHarness(t)
	m := memory.New(h.pool)
	m.AttachIdentity(h.iam)
	entries := append(example.New(h.pool).Entries(), m.Entries()...)
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	h.gw = gateway.New(gateway.Options{Registry: catalog, Identity: h.iam, Pool: h.pool, Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	m.AttachGateway(h.gw)
	owner := h.account(t, "entity-owner", "example:read", "example:create", "memory:read", "memory:write")
	other := h.account(t, "entity-other", "example:read", "example:create", "memory:read", "memory:write")
	record := h.ok(t, owner, "example.record.create", map[string]any{"title": "Source record"})
	recordID := rowID(t, record)
	if len(record.EventIDs) != 1 {
		t.Fatal("source event missing")
	}
	if _, err := outbox.NewRelay(h.pool, nil).Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := memory.NewConsumer(h.pool).Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "partition": "business", "memoryType": "note", "title": "Experience", "summary": "Private context", "sourceEventId": record.EventIDs[0]}
	created := h.ok(t, owner, "memory.entity.create", input)
	id := rowID(t, created)
	if _, err := h.call(t, owner, "memory.entity.list", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "unknown"}); err == nil {
		t.Fatal("unregistered entity type was accepted")
	}
	if _, err := h.call(t, other, "memory.entity.get", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "id": id}); err == nil {
		t.Fatal("nonowner read entity memory")
	}
	if _, err := h.call(t, other, "memory.entity.data.list", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record"}); err == nil {
		t.Fatal("direct entity data call succeeded")
	}
	sources := h.ok(t, owner, "memory.entity.sources", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "id": id})
	if len(sources.Data.(map[string]any)["sources"].([]map[string]any)) != 1 {
		t.Fatal("authorized original event missing")
	}
	second := rowID(t, h.ok(t, owner, "example.record.create", map[string]any{"title": "Other record"}))
	if _, err := h.call(t, owner, "memory.entity.create", map[string]any{"scopeType": "business_entity", "scopeId": second, "resourceType": "example_record", "partition": "business", "memoryType": "note", "title": "Mismatch", "summary": "", "sourceEventId": record.EventIDs[0]}); err == nil {
		t.Fatal("different entity source was attached")
	}
	h.ok(t, owner, "memory.entity.correct", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "id": id, "version": 1, "title": "Corrected", "summary": ""})
	h.ok(t, owner, "memory.entity.archive", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "id": id, "version": 2})
	h.ok(t, owner, "memory.entity.delete", map[string]any{"scopeType": "business_entity", "scopeId": recordID, "resourceType": "example_record", "id": id, "version": 3})
	var remaining int
	if err := h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM memory_sources WHERE tenant_id='tenant-a' AND memory_id=$1)+(SELECT count(*) FROM memory_corrections WHERE tenant_id='tenant-a' AND memory_id=$1)`, id).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("deleted entity memory copies remain: %d %v", remaining, err)
	}
}

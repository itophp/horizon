//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/system/memory"
)

func TestPersonalMemoryGatewayIsolationAndLifecycle(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	other := h.account(t, "memory-other", "memory:read", "memory:write")
	noPermission := h.account(t, "memory-denied")
	created, err := h.call(t, h.root, "memory.item.create", map[string]any{"partition": "agent", "memoryType": "preference", "title": "Personal title", "summary": "Private preference"})
	if err != nil {
		t.Fatal(err)
	}
	data := created.Data.(map[string]any)
	id, version := data["id"].(int64), data["version"].(int64)
	linked := h.ok(t, h.root, "memory.item.create", map[string]any{"partition": "agent", "memoryType": "related", "title": "Related", "summary": ""})
	linkedID := rowID(t, linked)
	h.ok(t, h.root, "memory.item.archive", map[string]any{"id": linkedID, "version": 1})
	active := h.ok(t, h.root, "memory.item.list", map[string]any{})
	if len(active.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("default list included archived memory")
	}
	archived := h.ok(t, h.root, "memory.item.list", map[string]any{"status": "archived", "createdFrom": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), "createdBefore": time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)})
	if len(archived.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("archived memory missing")
	}
	otherArchived := h.ok(t, other, "memory.item.list", map[string]any{"status": "archived"})
	if len(otherArchived.Data.(map[string]any)["items"].([]map[string]any)) != 0 {
		t.Fatal("archived memory leaked to another user")
	}
	if _, err = h.call(t, h.root, "memory.item.list", map[string]any{"createdFrom": "invalid"}); err == nil {
		t.Fatal("invalid date accepted")
	}
	if _, err = h.call(t, h.root, "memory.item.list", map[string]any{"createdFrom": "2026-01-02T00:00:00Z", "createdBefore": "2026-01-01T00:00:00Z"}); err == nil {
		t.Fatal("inverted date range accepted")
	}
	old := h.ok(t, h.root, "memory.item.list", map[string]any{"status": "archived", "createdBefore": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)})
	if len(old.Data.(map[string]any)["items"].([]map[string]any)) != 0 {
		t.Fatal("creation time filter ignored")
	}
	if _, err = h.call(t, h.root, "memory.item.list", map[string]any{"status": "deleted"}); err == nil {
		t.Fatal("deleted filter accepted")
	}
	if _, err = h.pool.Exec(context.Background(), `INSERT INTO memory_relations(tenant_id,source_memory_id,target_memory_id,relation_type) VALUES('tenant-a',$1,$2,'related')`, id, rowID(t, linked)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.item.get", map[string]any{"id": id}); err != nil {
		t.Fatal(err)
	}
	var auditCount int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_access_audit WHERE tenant_id='tenant-a' AND actor_id=$1 AND object_id=$2 AND action='memory.item.get'`, h.root.id, id).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("access audit missing: %d %v", auditCount, err)
	}
	if _, err = h.call(t, other, "memory.item.get", map[string]any{"id": id}); err == nil {
		t.Fatal("another account read the memory")
	}
	if _, err = h.call(t, noPermission, "memory.item.list", map[string]any{}); err == nil {
		t.Fatal("read without permission succeeded")
	}
	if _, err = h.callWith(t, h.root, "memory.item.get", map[string]any{"id": id}, "", contracts.OriginAgent); err == nil {
		t.Fatal("agent access is not yet authorized")
	}
	corrected, err := h.call(t, h.root, "memory.item.correct", map[string]any{"id": id, "version": version, "title": "Corrected", "summary": "Private revision"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.item.correct", map[string]any{"id": id, "version": version, "title": "Stale", "summary": ""}); err == nil {
		t.Fatal("stale correction succeeded")
	}
	version = corrected.Data.(map[string]any)["version"].(int64)
	if _, err = h.call(t, h.root, "memory.item.delete", map[string]any{"id": id, "version": version}); err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.item.get", map[string]any{"id": id}); err == nil {
		t.Fatal("deleted memory remains visible")
	}
	var corrections int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_corrections WHERE tenant_id='tenant-a' AND memory_id=$1`, id).Scan(&corrections); err != nil || corrections != 0 {
		t.Fatalf("correction copies remain: %d %v", corrections, err)
	}
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_relations WHERE tenant_id='tenant-a' AND (source_memory_id=$1 OR target_memory_id=$1)`, id).Scan(&corrections); err != nil || corrections != 0 {
		t.Fatalf("relation copies remain: %d %v", corrections, err)
	}
	var leaked int
	if err = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM cap_invocations WHERE input::text LIKE '%Private preference%' OR result::text LIKE '%Private preference%'`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("private text leaked to invocation audit: %d %v", leaked, err)
	}
}

func TestMemoryReadFailsClosedIfAccessAuditUnavailable(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	created, err := h.call(t, h.root, "memory.item.create", map[string]any{"partition": "agent", "memoryType": "preference", "title": "Title", "summary": "private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(context.Background(), `ALTER TABLE memory_access_audit ADD CONSTRAINT reject_test_inserts CHECK (false) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.item.get", map[string]any{"id": created.Data.(map[string]any)["id"]}); err == nil {
		t.Fatal("read succeeded without access audit")
	}
}

func TestMemoryValidityAndSafeProjection(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	now := time.Now().UTC().Truncate(time.Microsecond)
	input := map[string]any{"partition": "knowledge", "memoryType": "rule", "title": "Rule", "summary": "", "validFrom": now.Add(-time.Hour).Format(time.RFC3339Nano), "validTo": now.Add(time.Hour).Format(time.RFC3339Nano)}
	created := h.ok(t, h.root, "memory.item.create", input)
	got := h.ok(t, h.root, "memory.item.get", map[string]any{"id": rowID(t, created)}).Data.(map[string]any)
	if got["validFrom"] != input["validFrom"] || got["validTo"] != input["validTo"] || got["sensitivity"] != "internal" || got["verificationStatus"] != "unverified" {
		t.Fatalf("validity projection: %+v", got)
	}
	future := h.ok(t, h.root, "memory.item.create", map[string]any{"partition": "knowledge", "memoryType": "rule", "title": "Future", "summary": "", "validFrom": now.Add(time.Hour).Format(time.RFC3339Nano), "validTo": now.Add(2 * time.Hour).Format(time.RFC3339Nano)})
	if _, err := h.call(t, h.root, "memory.item.get", map[string]any{"id": rowID(t, future)}); err == nil {
		t.Fatal("not-yet-valid memory returned")
	}
	expired := h.ok(t, h.root, "memory.item.create", map[string]any{"partition": "knowledge", "memoryType": "rule", "title": "Expired", "summary": "", "validFrom": now.Add(-2 * time.Hour).Format(time.RFC3339Nano), "validTo": now.Add(-time.Hour).Format(time.RFC3339Nano)})
	if _, err := h.call(t, h.root, "memory.item.get", map[string]any{"id": rowID(t, expired)}); err == nil {
		t.Fatal("expired memory returned")
	}
	listed := h.ok(t, h.root, "memory.item.list", map[string]any{})
	if len(listed.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("ordinary list included invalid memories")
	}
	for _, invalid := range []map[string]any{
		{"validFrom": "bad"},
		{"validFrom": now.Add(time.Hour).Format(time.RFC3339Nano), "validTo": now.Format(time.RFC3339Nano)},
	} {
		for key, value := range map[string]any{"partition": "knowledge", "memoryType": "rule", "title": "Invalid", "summary": ""} {
			invalid[key] = value
		}
		if _, err := h.call(t, h.root, "memory.item.create", invalid); err == nil {
			t.Fatal("invalid validity accepted")
		}
	}
}

func TestActivityCollectionUsesTrustedIdentityAndAllowlist(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	other := h.account(t, "activity-other", "memory:read", "memory:write", "memory:collect")
	event := map[string]any{"id": "client-id", "kind": "page_view", "page": "drive", "action": "view", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano)}
	if _, err := h.call(t, h.root, "memory.activity.collect", map[string]any{"events": []any{event}}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.call(t, h.root, "memory.activity.collect", map[string]any{"events": []any{event}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id=$1`, fmt.Sprintf("web:%d:client-id", h.root.id)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("activity deduplication: %d %v", count, err)
	}
	bad := map[string]any{"id": "bad", "kind": "page_view", "page": "drive/secret", "action": "view", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano)}
	if _, err := h.call(t, h.root, "memory.activity.collect", map[string]any{"events": []any{bad}}); err == nil {
		t.Fatal("unregistered page accepted")
	}
	if _, err := h.call(t, other, "memory.item.create", map[string]any{"partition": "activity", "memoryType": "visit", "title": "Visit", "summary": "", "sourceEventId": fmt.Sprintf("web:%d:client-id", h.root.id)}); err == nil {
		t.Fatal("another user's source was attached")
	}
	created, err := h.call(t, h.root, "memory.item.create", map[string]any{"partition": "activity", "memoryType": "visit", "title": "Visit", "summary": "", "sourceEventId": fmt.Sprintf("web:%d:client-id", h.root.id)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.item.sources", map[string]any{"id": created.Data.(map[string]any)["id"]}); err != nil {
		t.Fatal(err)
	}
	own, err := h.call(t, h.root, "memory.event.list", map[string]any{"eventType": "activity.page.viewed"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(own.Data.(map[string]any)["events"].([]map[string]any)); got != 1 {
		t.Fatalf("own event count: %d", got)
	}
	foreign, err := h.call(t, other, "memory.event.list", map[string]any{"eventType": "activity.page.viewed"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(foreign.Data.(map[string]any)["events"].([]map[string]any)); got != 0 {
		t.Fatalf("other account saw %d events", got)
	}
	eventID := fmt.Sprintf("web:%d:client-id", h.root.id)
	filter := map[string]any{"eventId": eventID, "occurredFrom": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), "occurredBefore": time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)}
	filtered, err := h.call(t, h.root, "memory.event.list", filter)
	if err != nil || len(filtered.Data.(map[string]any)["events"].([]map[string]any)) != 1 {
		t.Fatalf("event and time filters: %v", err)
	}
	filtered, err = h.call(t, other, "memory.event.list", filter)
	if err != nil || len(filtered.Data.(map[string]any)["events"].([]map[string]any)) != 0 {
		t.Fatalf("event filter bypassed account isolation: %v", err)
	}
	for _, input := range []map[string]any{
		{"eventId": "missing"},
		{"resourceType": "ai_run", "resourceId": "123"},
		{"occurredBefore": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)},
	} {
		filtered, err = h.call(t, h.root, "memory.event.list", input)
		if err != nil || len(filtered.Data.(map[string]any)["events"].([]map[string]any)) != 0 {
			t.Fatalf("filter should not match: %v, %v", input, err)
		}
	}
	for _, input := range []map[string]any{
		{"occurredFrom": "invalid"},
		{"occurredFrom": "2026-01-02T00:00:00Z", "occurredBefore": "2026-01-01T00:00:00Z"},
	} {
		if _, err = h.call(t, h.root, "memory.event.list", input); err == nil {
			t.Fatalf("invalid time range accepted: %v", input)
		}
	}
}

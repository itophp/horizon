//go:build integration

package tests

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"baseline/services/api/kernel/approval"
	"baseline/services/api/kernel/audit"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/kernel/eventhub"
	canonical "baseline/services/api/kernel/memory"
	"baseline/services/api/runtime/app"
	"baseline/services/api/runtime/transport"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"
	"baseline/services/api/system/memory"
)

// The lower source ID commits last: a high-water cursor would miss it.
func TestMemoryConsumerLateCommitAndDeduplication(t *testing.T) {
	_, pool, _ := iamFixture(t)
	ctx := context.Background()
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx)
	insert := `INSERT INTO cap_events (tenant_id,event_id,event_type,aggregate_type,aggregate_id,payload,account_id,correlation_id)
		VALUES ($1,$2,'example.record.created','example_record','42','{"password":"secret"}'::jsonb,1,'corr')`
	if _, err = first.Exec(ctx, insert, "tenant-a", "late"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, insert, "tenant-b", "early"); err != nil {
		t.Fatal(err)
	}
	consumer := memory.NewConsumer(pool)
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if err = first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE event_id IN ('late','early')`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("late event lost or duplicated: count=%d err=%v", count, err)
	}
	var payload string
	if err = pool.QueryRow(ctx, `SELECT payload::text FROM memory_events WHERE tenant_id='tenant-a' AND event_id='late'`).Scan(&payload); err != nil || payload != "{}" {
		t.Fatalf("unapproved payload retained: %s %v", payload, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE memory_event_inbox SET completed_at=NULL WHERE tenant_id='tenant-a' AND event_id='late'`); err != nil {
		t.Fatal(err)
	}
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id='late'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("replay must deduplicate: count=%d err=%v", count, err)
	}
}

func TestMemoryReplayIsTenantScopedAndIdempotent(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `INSERT INTO cap_events (tenant_id,event_id,event_type,aggregate_type,aggregate_id)
	 VALUES ('tenant-a','replay-me','example.record.created','example_record','1'),
	        ('tenant-b','foreign','example.record.created','example_record','2')`)
	if err != nil {
		t.Fatal(err)
	}
	consumer := memory.NewConsumer(h.pool)
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(ctx, `UPDATE memory_event_inbox SET completed_at=NULL,dead_at=now(),attempts=5,error_code='invalid_event' WHERE tenant_id='tenant-a' AND event_id='replay-me'`); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(ctx, `UPDATE memory_event_inbox SET completed_at=NULL,dead_at=now(),attempts=5,error_code='invalid_event' WHERE tenant_id='tenant-b' AND event_id='foreign'`); err != nil {
		t.Fatal(err)
	}
	reader := h.account(t, "no-replay", "memory:read")
	if _, err = h.call(t, reader, "memory.ingestion.failures", map[string]any{}); err == nil {
		t.Fatal("unprivileged failure listing succeeded")
	}
	if _, err = h.callWith(t, h.root, "memory.ingestion.replay", map[string]any{"eventId": "replay-me"}, "agent-replay", contracts.OriginAgent); err == nil {
		t.Fatal("agent requested manual-only replay")
	}
	if _, err = h.call(t, h.root, "memory.ingestion.replay", map[string]any{"eventId": "foreign"}); err == nil {
		t.Fatal("unknown or cross-tenant event replayed")
	}
	if _, err = h.call(t, h.root, "memory.ingestion.failures", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.ingestion.replay", map[string]any{"eventId": "replay-me"}); err != nil {
		t.Fatal(err)
	}
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = h.pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id='replay-me'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("replay duplicated event: %d %v", count, err)
	}
}

func TestMemoryReplayRangeIsBoundedTenantScopedAndTransactional(t *testing.T) {
	h := newProjectHarness(t)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(memory.New(h.pool).Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `INSERT INTO cap_events (tenant_id,event_id,event_type,aggregate_type,aggregate_id) VALUES
		('tenant-a','range-1','example.record.created','example_record','1'),
		('tenant-b','range-foreign','example.record.created','example_record','2'),
		('tenant-a','range-2','example.record.created','example_record','3')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = memory.NewConsumer(h.pool).Once(ctx); err != nil {
		t.Fatal(err)
	}
	var first, last int64
	if err = h.pool.QueryRow(ctx, `SELECT min(source_event_id),max(source_event_id) FROM memory_event_inbox WHERE event_id IN ('range-1','range-2')`).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"firstSourceId": first, "lastSourceId": last, "limit": 1}
	reader := h.account(t, "range-no-permission", "memory:read")
	if _, err = h.call(t, reader, "memory.ingestion.replay_range", input); err == nil {
		t.Fatal("unprivileged replay succeeded")
	}
	if _, err = h.callWith(t, h.root, "memory.ingestion.replay_range", input, "range-agent", contracts.OriginAgent); err == nil {
		t.Fatal("agent replay succeeded")
	}
	result, err := h.call(t, h.root, "memory.ingestion.replay_range", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.(map[string]any)["queued"] != 1 || result.Data.(map[string]any)["lastQueuedSourceId"] != first || len(result.EventIDs) != 1 {
		t.Fatalf("incorrect bounded replay: %+v", result)
	}
	var pending, foreign int
	if err = h.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE tenant_id='tenant-a' AND completed_at IS NULL),count(*) FILTER (WHERE tenant_id='tenant-b' AND completed_at IS NULL) FROM memory_event_inbox WHERE event_id LIKE 'range-%'`).Scan(&pending, &foreign); err != nil || pending != 1 || foreign != 0 {
		t.Fatalf("tenant scope or limit failed: %d %d %v", pending, foreign, err)
	}
	if _, err = h.call(t, h.root, "memory.ingestion.replay_range", map[string]any{"firstSourceId": last, "lastSourceId": first, "limit": 1}); err == nil {
		t.Fatal("inverted range accepted")
	}
	if err = memory.NewConsumer(h.pool).Once(ctx); err != nil {
		t.Fatal(err)
	}
	var copies int
	if err = h.pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id='range-1'`).Scan(&copies); err != nil || copies != 1 {
		t.Fatalf("replay duplicated original: %d %v", copies, err)
	}
	if _, err = h.pool.Exec(ctx, `ALTER TABLE cap_outbox ADD CONSTRAINT reject_range_test CHECK (false) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = h.call(t, h.root, "memory.ingestion.replay_range", input); err == nil {
		t.Fatal("range replay succeeded when outbox insert failed")
	}
	var completed bool
	if err = h.pool.QueryRow(ctx, `SELECT completed_at IS NOT NULL FROM memory_event_inbox WHERE tenant_id='tenant-a' AND event_id='range-1'`).Scan(&completed); err != nil || !completed {
		t.Fatalf("range replay was not rolled back with outbox: %v %v", completed, err)
	}
}

func TestMemoryConsumerDeadLettersPoisonEventWithoutBlockingOthers(t *testing.T) {
	_, pool, _ := iamFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO cap_events (tenant_id,event_id,event_type,aggregate_type,aggregate_id)
	 VALUES ('tenant-a','poison',repeat('x',300),'example_record','1'),('tenant-a','healthy','example.record.created','example_record','2')`)
	if err != nil {
		t.Fatal(err)
	}
	consumer := memory.NewConsumer(pool)
	for attempt := 0; attempt < 5; attempt++ {
		if err := consumer.Once(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE memory_event_inbox SET available_at=now() WHERE tenant_id='tenant-a' AND event_id='poison' AND dead_at IS NULL`); err != nil {
			t.Fatal(err)
		}
	}
	var dead bool
	var attempts int
	var code string
	if err := pool.QueryRow(ctx, `SELECT dead_at IS NOT NULL, attempts, error_code FROM memory_event_inbox WHERE tenant_id='tenant-a' AND event_id='poison'`).Scan(&dead, &attempts, &code); err != nil || !dead || attempts != 5 || code != "invalid_event" {
		t.Fatalf("failed event not dead-lettered: %v %d %s %v", dead, attempts, code, err)
	}
	var healthy int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id='healthy'`).Scan(&healthy); err != nil || healthy != 1 {
		t.Fatalf("healthy event blocked: %d %v", healthy, err)
	}
}

func TestHTTPObservationCorrelatesCapabilityWithoutRequestContent(t *testing.T) {
	_, pool, _ := iamFixture(t)
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir(), PublicWorkspace: "tenant-a"})
	handler, start := transport.HandlerWithRuntime(app.System{}, core, iam)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start(ctx)
	client := loginIAM(t, handler, "root-admin")
	w := client.request(t, "POST", "/api/capabilities/example.record.list/invocations?secret=private", map[string]any{"input": map[string]any{"limit": 1}}, true)
	checkStatus(t, w, 200)
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("request id missing")
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var payload, correlation string
		err := pool.QueryRow(ctx, `SELECT payload::text,correlation_id FROM memory_events WHERE tenant_id='tenant-a' AND event_id=$1`, "http:"+requestID).Scan(&payload, &correlation)
		if err == nil {
			if strings.Contains(payload, "private") || correlation != "req_"+requestID {
				t.Fatalf("unsafe or uncorrelated observation: %s %s", payload, correlation)
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM cap_invocations WHERE tenant_id='tenant-a' AND correlation_id=$1`, correlation).Scan(&count); err != nil || count != 1 {
				t.Fatalf("capability correlation missing: %d %v", count, err)
			}
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("HTTP observation missing: %v", err)
		case <-ticker.C:
		}
	}
}

func TestMemoryObserverFiltersAndPersistsTelemetry(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	observer := memory.NewObserver(pool)
	go func() { defer close(done); observer.Run(ctx) }()
	defer func() { cancel(); <-done }()
	observer.Submit(canonical.Envelope{EventID: "http:test", EventType: "activity.http.completed", TenantID: "tenant-a", OccurredAt: time.Now(),
		Actor: canonical.Actor{ID: fmt.Sprint(root.ID), Type: "user"}, Source: canonical.Source{Type: "http", ID: "capabilities"},
		Payload: map[string]string{"method": "POST", "status": "2xx", "duration": "lt1s", "password": "private"}})
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var payload string
		err := pool.QueryRow(ctx, `SELECT payload::text FROM memory_events WHERE tenant_id='tenant-a' AND event_id='http:test'`).Scan(&payload)
		if err == nil {
			if strings.Contains(payload, "private") || !strings.Contains(payload, "POST") {
				t.Fatalf("unsafe payload: %s", payload)
			}
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("observer did not persist event: %v", err)
		case <-ticker.C:
		}
	}
}

func TestMemoryConsumerProjectsExecutionMetadataWithoutContent(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO cap_invocations
	 (tenant_id,account_id,department_id,invocation_id,capability,capability_version,origin,status,input,input_digest,result,trace_id,correlation_id,finished_at)
	 VALUES ('tenant-a',$1,$2,'test-inv','example.record.create','1.0.0','agent','succeeded','{"secret":"private"}','digest','{"body":"private"}','trace','corr',now())`, root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	var runID int64
	err = pool.QueryRow(ctx, `INSERT INTO ai_runs (tenant_id,account_id,department_id,agent,model,instruction_version,question,answer,status,trace_id,correlation_id,finished_at)
	 VALUES ('tenant-a',$1,$2,'orchestration','test','1','private question','private answer','completed','trace','corr',now()) RETURNING id`, root.ID, root.DepartmentID).Scan(&runID)
	if err != nil {
		t.Fatal(err)
	}
	consumer := memory.NewConsumer(pool)
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if err = consumer.Once(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND
	 (event_id IN ('inv:test-inv:started','inv:test-inv:succeeded','agent-tool:test-inv:started','agent-tool:test-inv:succeeded') OR event_id IN ('run:'||$1::bigint::text||':started','run:'||$1::bigint::text||':completed'))`, runID).Scan(&count)
	if err != nil || count != 6 {
		t.Fatalf("missing or duplicate projections: %d %v", count, err)
	}
	var taskID, workflowID string
	var partitions []string
	err = pool.QueryRow(ctx, `SELECT task_id,workflow_instance_id,candidate_partitions FROM memory_events WHERE tenant_id='tenant-a' AND event_id='run:'||$1::bigint::text||':completed'`, runID).Scan(&taskID, &workflowID, &partitions)
	if err != nil || taskID != strconv.FormatInt(runID, 10) || workflowID != taskID || len(partitions) != 1 || partitions[0] != "execution" {
		t.Fatalf("execution context missing: %q %q %v %v", taskID, workflowID, partitions, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND row_to_json(memory_events)::text LIKE '%private%'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("sensitive execution content copied: %d %v", count, err)
	}
}

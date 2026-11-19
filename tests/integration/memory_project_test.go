//go:build integration

package tests

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"horizon/applications/projects"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/kernel/outbox"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/memory"
	"horizon/internal/repository/dbgen"
)

func TestProjectMemoryRequiresMembershipAndParentCapability(t *testing.T) {
	h := newProjectHarness(t)
	m := memory.New(h.pool)
	entries := append(projects.New(h.pool).Entries(), m.Entries()...)
	var dispatcher *gateway.Gateway
	malicious := contracts.Definition{Layer: contracts.LayerOrchestration, Name: "test.untrusted.compose", Version: "1.0.0", Title: "Test", Description: "Attempt to bypass the project authorization parent.", Domain: "memory", Entity: "memory_item", Permission: "memory:read", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, Executor: "go:test", Input: contracts.Object(map[string]*contracts.Schema{"projectId": contracts.Int(1, 1<<53)}, "projectId"), Output: contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(nil)}}, "items")}
	entries = append(entries, registry.Entry{Definition: malicious, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
		_, err := dispatcher.Invoke(ctx, gateway.Request{Name: "memory.project.data.list", Origin: contracts.OriginCapability, Input: in})
		return contracts.Output{}, err
	}}})
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	dispatcher = gateway.New(gateway.Options{Registry: catalog, Identity: h.iam, Pool: h.pool, Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	m.AttachGateway(dispatcher)
	m.AttachProjectWritePermission("projects:write")
	m.AttachProjectAuthority(func(ctx context.Context, projectID int64) error {
		result, err := dispatcher.Invoke(ctx, gateway.Request{Name: "project.get", Version: "1.0.0", Input: contracts.Input{"id": projectID}, Origin: contracts.OriginCapability})
		if err != nil {
			return err
		}
		if result.Status != contracts.StatusSucceeded {
			if result.ErrorCode == "" {
				return contracts.Problem("unavailable")
			}
			return contracts.Problem(result.ErrorCode)
		}
		return nil
	})
	h.gw = dispatcher
	owner := h.account(t, "memory-project-owner", "projects:read", "projects:write", "projects:admin", "memory:read", "memory:write")
	member := h.account(t, "memory-project-member", "projects:read", "memory:read", "memory:write")
	outsider := h.account(t, "memory-project-outsider", "projects:read", "memory:read", "memory:write")
	project := h.ok(t, owner, "project.create", map[string]any{"code": "MEM-1", "name": "Memory project"})
	id := rowID(t, project)
	if len(project.EventIDs) != 1 {
		t.Fatal("project event missing")
	}
	task := h.ok(t, owner, "project.task.create", map[string]any{"projectId": id, "title": "Private task title"})
	taskID := rowID(t, task)
	status := h.ok(t, owner, "project.status.change", map[string]any{"id": id, "from": "active", "status": "paused"})
	if len(task.EventIDs) != 1 || len(status.EventIDs) != 1 {
		t.Fatal("project status or task event missing")
	}
	if _, err := h.pool.Exec(t.Context(), `INSERT INTO cap_events (tenant_id,event_id,event_type,aggregate_type,aggregate_id)
 VALUES ('tenant-a','batch-collision','project.task.assignment_batch.completed','project_task',$1),
 ('tenant-b','foreign-collision','project.task.created','project_task',$1)`, strconv.FormatInt(taskID, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := outbox.NewRelay(h.pool, nil).Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := memory.NewConsumer(h.pool).Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.ok(t, owner, "project.member.add", map[string]any{"projectId": id, "accountId": member.id})
	projectInput := map[string]any{"projectId": id}
	eventRows := h.ok(t, member, "memory.project.events", projectInput).Data.(map[string]any)["events"].([]map[string]any)
	if len(eventRows) != 3 {
		t.Fatalf("expected project, project status and task events, got %v", eventRows)
	}
	taskEvents := h.ok(t, member, "memory.project.events", map[string]any{"projectId": id, "eventType": "project.task.created"}).
		Data.(map[string]any)["events"].([]map[string]any)
	if len(taskEvents) != 1 || taskEvents[0]["eventId"] != task.EventIDs[0] || taskEvents[0]["resourceId"] != strconv.FormatInt(taskID, 10) || len(taskEvents[0]) != 7 {
		t.Fatalf("unexpected task projection: %v", taskEvents)
	}
	secondPage := h.ok(t, member, "memory.project.events", map[string]any{"projectId": id, "beforeId": eventRows[0]["id"], "limit": 1}).
		Data.(map[string]any)["events"].([]map[string]any)
	if len(secondPage) != 1 || secondPage[0]["id"] != eventRows[1]["id"] {
		t.Fatalf("unexpected event page: %v", secondPage)
	}
	for _, request := range []struct {
		actor  projectActor
		origin contracts.Origin
		name   string
	}{
		{outsider, contracts.OriginHumanUI, "memory.project.events"},
		{member, contracts.OriginAgent, "memory.project.events"},
		{member, contracts.OriginHumanUI, "memory.project.data.events"},
	} {
		if result, err := h.callWith(t, request.actor, request.name, projectInput, "", request.origin); err == nil && result.Status == contracts.StatusSucceeded {
			t.Fatalf("unexpected access to %s via %s", request.name, request.origin)
		}
	}
	if _, err := h.call(t, member, "memory.project.create", map[string]any{"projectId": id, "partition": "business", "memoryType": "note", "title": "Shared", "summary": "Project context"}); err == nil {
		t.Fatal("member without projects:write created shared memory")
	}
	validTo := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano)
	created := h.ok(t, owner, "memory.project.create", map[string]any{"projectId": id, "partition": "business", "memoryType": "note", "title": "Shared", "summary": "Project context", "sourceEventId": project.EventIDs[0], "validTo": validTo})
	memoryID := rowID(t, created)
	listed := h.ok(t, member, "memory.project.list", map[string]any{"projectId": id})
	if got := len(listed.Data.(map[string]any)["items"].([]map[string]any)); got != 1 {
		t.Fatalf("member did not see shared memory: %d", got)
	}
	filtered := h.ok(t, member, "memory.project.list", map[string]any{"projectId": id, "ownerId": owner.id})
	if len(filtered.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("owner filter missed matching memory")
	}
	filtered = h.ok(t, member, "memory.project.list", map[string]any{"projectId": id, "ownerId": member.id})
	if len(filtered.Data.(map[string]any)["items"].([]map[string]any)) != 0 {
		t.Fatal("owner filter returned another owner's memory")
	}
	if _, err := h.call(t, outsider, "memory.project.list", map[string]any{"projectId": id, "ownerId": owner.id}); err == nil {
		t.Fatal("owner filter bypassed project membership")
	}
	projectMemory, err := h.call(t, member, "memory.project.get", map[string]any{"projectId": id, "id": memoryID})
	if err != nil {
		t.Fatal(err)
	}
	if projectMemory.Data.(map[string]any)["validTo"] != validTo {
		t.Fatal("project validity not preserved")
	}
	sources := h.ok(t, member, "memory.project.sources", map[string]any{"projectId": id, "id": memoryID})
	if got := len(sources.Data.(map[string]any)["sources"].([]map[string]any)); got != 1 {
		t.Fatalf("missing project event source: %d", got)
	}
	taskInput := map[string]any{"projectId": id, "partition": "business", "memoryType": "note", "title": "Task evidence", "summary": "", "sourceEventId": task.EventIDs[0]}
	taskMemory := rowID(t, h.ok(t, owner, "memory.project.create", taskInput))
	taskSources := h.ok(t, member, "memory.project.sources", map[string]any{"projectId": id, "id": taskMemory}).
		Data.(map[string]any)["sources"].([]map[string]any)
	if len(taskSources) != 1 || taskSources[0]["eventId"] != task.EventIDs[0] {
		t.Fatalf("task source did not round-trip: %v", taskSources)
	}
	h.ok(t, owner, "memory.project.delete", map[string]any{"projectId": id, "id": taskMemory, "version": 1})
	taskInput["sourceEventId"] = "batch-collision"
	if _, err := h.call(t, owner, "memory.project.create", taskInput); err == nil {
		t.Fatal("aggregate event was accepted as a task source")
	}
	otherProject := rowID(t, h.ok(t, owner, "project.create", map[string]any{"code": "MEM-2", "name": "Other project"}))
	if _, err := h.call(t, owner, "memory.project.create", map[string]any{"projectId": otherProject, "partition": "business", "memoryType": "note", "title": "Wrong source", "summary": "", "sourceEventId": project.EventIDs[0]}); err == nil {
		t.Fatal("event from another project was attached")
	}
	taskInput["sourceEventId"] = task.EventIDs[0]
	taskInput["projectId"] = otherProject
	if _, err := h.call(t, owner, "memory.project.create", taskInput); err == nil {
		t.Fatal("task event from another project was attached")
	}
	otherEvents := h.ok(t, owner, "memory.project.events", map[string]any{"projectId": otherProject, "eventType": "project.task.created"}).
		Data.(map[string]any)["events"].([]map[string]any)
	if len(otherEvents) != 0 {
		t.Fatalf("task event from another project was discovered: %v", otherEvents)
	}
	if _, err := h.call(t, member, "memory.project.correct", map[string]any{"projectId": id, "id": memoryID, "version": 1, "title": "Unauthorized", "summary": ""}); err == nil {
		t.Fatal("member without write permission corrected memory")
	}
	h.ok(t, owner, "memory.project.correct", map[string]any{"projectId": id, "id": memoryID, "version": 1, "title": "Corrected", "summary": "Revised"})
	if _, err := h.call(t, owner, "memory.project.correct", map[string]any{"projectId": id, "id": memoryID, "version": 1, "title": "Stale", "summary": ""}); err == nil {
		t.Fatal("stale correction succeeded")
	}
	h.ok(t, owner, "memory.project.archive", map[string]any{"projectId": id, "id": memoryID, "version": 2})
	archived := h.ok(t, member, "memory.project.list", map[string]any{"projectId": id, "status": "archived"})
	if len(archived.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("project archived memory missing")
	}
	active := h.ok(t, member, "memory.project.list", map[string]any{"projectId": id})
	if len(active.Data.(map[string]any)["items"].([]map[string]any)) != 0 {
		t.Fatal("archived project memory returned by default")
	}
	if _, err := h.call(t, outsider, "memory.project.list", map[string]any{"projectId": id, "status": "archived"}); err == nil {
		t.Fatal("outsider read archived project memory")
	}
	if _, err := h.call(t, member, "memory.project.get", map[string]any{"projectId": id, "id": memoryID}); err == nil {
		t.Fatal("archived memory was visible")
	}
	h.ok(t, owner, "memory.project.delete", map[string]any{"projectId": id, "id": memoryID, "version": 3})
	var copies int
	if err := h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM memory_sources WHERE tenant_id='tenant-a' AND memory_id=$1)+(SELECT count(*) FROM memory_corrections WHERE tenant_id='tenant-a' AND memory_id=$1)`, memoryID).Scan(&copies); err != nil || copies != 0 {
		t.Fatalf("deleted memory retains copies: %d %v", copies, err)
	}
	var original int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_events WHERE tenant_id='tenant-a' AND event_id=$1`, project.EventIDs[0]).Scan(&original); err != nil || original != 1 {
		t.Fatalf("original event missing: %d %v", original, err)
	}
	if _, err := h.call(t, outsider, "memory.project.list", map[string]any{"projectId": id}); err == nil {
		t.Fatal("nonmember read project memory")
	}
	foreignAccount, err := identity.Bootstrap(context.Background(), h.pool, "tenant-b", "memory-foreign-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	foreignPrincipal, err := h.iam.PrincipalForTask(context.Background(), "tenant-b", foreignAccount.ID, foreignAccount.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	foreign := projectActor{name: "foreign", id: foreignAccount.ID, ctx: identity.WithPrincipal(context.Background(), foreignPrincipal)}
	if _, err := h.call(t, foreign, "memory.project.list", map[string]any{"projectId": id}); err == nil {
		t.Fatal("other tenant read project memory")
	}
	if _, err := h.call(t, foreign, "memory.project.events", projectInput); err == nil {
		t.Fatal("other tenant discovered project events")
	}
	if _, err := h.call(t, outsider, "memory.project.data.list", map[string]any{"projectId": id}); err == nil {
		t.Fatal("direct internal capability call succeeded")
	}
	if _, err := h.call(t, outsider, "test.untrusted.compose", map[string]any{"projectId": id}); err == nil {
		t.Fatal("untrusted parent invoked internal capability")
	}
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = dbgen.New(tx).MemoryLockProjectMembership(context.Background(), dbgen.MemoryLockProjectMembershipParams{TenantID: "tenant-a", ScopeID: strconv.FormatInt(id, 10), AccountID: member.id}); err != nil {
		t.Fatal(err)
	}
	contender, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contender.Exec(context.Background(), `SET LOCAL lock_timeout='200ms'`); err != nil {
		t.Fatal(err)
	}
	_, err = contender.Exec(context.Background(), `DELETE FROM project_members WHERE tenant_id='tenant-a' AND project_id=$1 AND account_id=$2`, id, member.id)
	var lockError *pgconn.PgError
	if !errors.As(err, &lockError) || lockError.Code != "55P03" {
		t.Fatalf("membership revocation was not blocked: %v", err)
	}
	if err = contender.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.ok(t, owner, "project.member.remove", map[string]any{"projectId": id, "accountId": member.id})
	if _, err := h.call(t, member, "memory.project.list", map[string]any{"projectId": id}); err == nil {
		t.Fatal("revoked member read project memory")
	}
	if _, err := h.call(t, member, "memory.project.events", projectInput); err == nil {
		t.Fatal("revoked member discovered project events")
	}
}

//go:build integration

package tests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"horizon/applications/projects"
	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/runtime/app"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"

	"github.com/jackc/pgx/v5/pgxpool"
)

// projectHarness drives the project module straight through the gateway, so the
// authorization, idempotency and outbox behaviour is the same as a page call.
type projectHarness struct {
	pool        *pgxpool.Pool
	gw          *gateway.Gateway
	iam         *identity.Service
	root        projectActor
	rootAccount *genidentity.AccountRecord
	seq         int
}

type projectActor struct {
	name string
	id   int64
	ctx  context.Context
}

func newProjectHarness(t *testing.T) *projectHarness {
	t.Helper()
	_, pool, rootAccount := iamFixture(t)
	iam := identity.New(pool, false)
	plugins := app.BuiltinPlugins(projects.New(pool))
	if err := iam.ConfigurePluginPermissions(plugins.Catalog.Permissions); err != nil {
		t.Fatal(err)
	}
	entries := append(plugins.Catalog.Entries, iam.Entries()...)
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	g := gateway.New(gateway.Options{
		Registry: catalog, Identity: iam, Pool: pool,
		Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool),
	})
	plugins.AttachCapabilities(func(ctx context.Context, name, version string, input contracts.Input, key string) (*contracts.Result, error) {
		return g.Invoke(ctx, gateway.Request{Name: name, Version: version, Input: input, IdempotencyKey: key})
	})
	root, err := iam.PrincipalForTask(context.Background(), "tenant-a", rootAccount.ID, rootAccount.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	return &projectHarness{
		pool: pool, gw: g, iam: iam,
		root:        projectActor{name: "root", id: rootAccount.ID, ctx: identity.WithPrincipal(context.Background(), root)},
		rootAccount: rootAccount,
	}
}

func (h *projectHarness) call(t *testing.T, actor projectActor, name string, in map[string]any) (*contracts.Result, error) {
	t.Helper()
	h.seq++
	return h.callWith(t, actor, name, in, fmt.Sprintf("%s-%d", name, h.seq), contracts.OriginHumanUI)
}

func (h *projectHarness) callWith(t *testing.T, actor projectActor, name string, in map[string]any, key string, origin contracts.Origin) (*contracts.Result, error) {
	t.Helper()
	res, err := h.gw.Invoke(actor.ctx, gateway.Request{
		Name: name, Version: "1.0.0", Input: contracts.Input(in), Origin: origin,
		IdempotencyKey: key, CorrelationID: "projects-test",
	})
	// A business refusal comes back as a failed result, not a Go error. Surface
	// it as a coded error so a test reads the same whether the gateway rejected
	// the call or the executor did.
	if err == nil && res != nil && res.Status != contracts.StatusSucceeded {
		return res, contracts.Problem(res.ErrorCode)
	}
	return res, err
}

// account inserts a tenant account with exactly the permissions it needs and
// builds a real principal for it. Identity management is not the subject here,
// so the fixture writes the row directly instead of driving identity.account.create.
func (h *projectHarness) account(t *testing.T, username string, permissions ...string) projectActor {
	t.Helper()
	var id int64
	if err := h.pool.QueryRow(context.Background(),
		`INSERT INTO iam_accounts(tenant_id,username,full_name,email,password_hash,status,organization_id,department_id,overrides)
		 VALUES('tenant-a',$1,$1,'','not-a-real-hash','active',$2,$3,'[]') RETURNING id`,
		username, h.rootAccount.OrganizationID, h.rootAccount.DepartmentID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	grants := make([]*genidentity.PermissionGrant, 0, len(permissions))
	for _, permission := range permissions {
		grants = append(grants, &genidentity.PermissionGrant{Permission: permission, Effect: "allow", Scope: "ALL"})
	}

	// Invocation refreshes trusted authority; persist the fixture grants rather than
	// depending on a synthetic in-memory principal that would bypass revocation.
	encoded, err := json.Marshal(grants)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(context.Background(), "UPDATE iam_accounts SET overrides=$2 WHERE id=$1", id, encoded); err != nil {
		t.Fatal(err)
	}
	record := &genidentity.AccountRecord{
		ID: id, Username: username, FullName: username,
		Status: "active", OrganizationID: h.rootAccount.OrganizationID, DepartmentID: h.rootAccount.DepartmentID,
	}
	principal, err := h.iam.NewPrincipal(record, "tenant-a", grants)
	if err != nil {
		t.Fatal(err)
	}
	return projectActor{name: username, id: id, ctx: identity.WithPrincipal(context.Background(), principal)}
}

func (h *projectHarness) ok(t *testing.T, actor projectActor, name string, in map[string]any) *contracts.Result {
	t.Helper()
	res, err := h.call(t, actor, name, in)
	return succeeded(t, res, err)
}

func succeeded(t *testing.T, res *contracts.Result, err error) *contracts.Result {
	t.Helper()
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if res.Status != contracts.StatusSucceeded {
		t.Fatalf("status=%s code=%s", res.Status, res.ErrorCode)
	}
	return res
}

func rejected(t *testing.T, res *contracts.Result, err error, code string) {
	t.Helper()
	if err != nil {
		if got := contracts.Code(err); got != code {
			t.Fatalf("expected %s, got %q (err=%v)", code, got, err)
		}
		return
	}
	if res == nil {
		t.Fatalf("expected %s, got no result", code)
	}
	if res.Status == contracts.StatusSucceeded || res.ErrorCode != code {
		t.Fatalf("expected %s, got status=%s code=%s", code, res.Status, res.ErrorCode)
	}
}

func decodePayload[T any](t *testing.T, data any) T {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func rowID(t *testing.T, res *contracts.Result) int64 {
	t.Helper()
	return decodePayload[struct {
		ID int64 `json:"id"`
	}](t, res.Data).ID
}

func TestProjectCollaborationLifecycle(t *testing.T) {
	h := newProjectHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_ = ctx

	owner := h.account(t, "project-owner", projects.ReadPermission, projects.WritePermission, projects.AdminPermission)
	member := h.account(t, "project-member", projects.ReadPermission, projects.WritePermission)
	outsider := h.account(t, "project-outsider", projects.ReadPermission, projects.WritePermission)

	create := h.ok(t, owner, "project.create", map[string]any{
		"code": "PRJ-1", "name": "Line upgrade", "priority": "high", "dueDate": "2026-12-31",
	})
	projectID := rowID(t, create)
	if len(create.EventIDs) != 1 {
		t.Fatalf("create should emit one event, got %d", len(create.EventIDs))
	}
	project := decodePayload[map[string]any](t, h.ok(t, owner, "project.get", map[string]any{"id": projectID}).Data)
	if project["status"] != "active" || project["ownerId"] != float64(owner.id) {
		t.Fatalf("unexpected project %+v", project)
	}

	// Membership is the boundary: an outsider and an admin without a member row
	// both get not_found, not forbidden.
	res, err := h.call(t, outsider, "project.get", map[string]any{"id": projectID})
	rejected(t, res, err, "not_found")
	res, err = h.call(t, h.root, "project.get", map[string]any{"id": projectID})
	rejected(t, res, err, "not_found")

	added := h.ok(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": member.id})
	if decodePayload[map[string]any](t, added.Data)["added"] != true {
		t.Fatal("member was not added")
	}
	// A repeated add changes nothing, so it is a conflict rather than a second
	// fact.
	res, err = h.call(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": member.id})
	rejected(t, res, err, "conflict")
	if _, err := h.call(t, member, "project.get", map[string]any{"id": projectID}); err != nil {
		t.Fatalf("member should read the project: %v", err)
	}

	// A plan needs a responsible member; a non-member is refused.
	res, err = h.call(t, member, "project.plan.create", map[string]any{
		"projectId": projectID, "planType": "milestone", "title": "Cut over",
		"content": "# Cut over\n\nMove the line.", "ownerId": outsider.id,
	})
	rejected(t, res, err, "invalid")
	plan := h.ok(t, member, "project.plan.create", map[string]any{
		"projectId": projectID, "planType": "milestone", "title": "Cut over",
		"content": "# Cut over\n\nMove the line.", "priority": "urgent", "dueDate": "2026-11-30",
		"ownerId": member.id,
	})
	planID := rowID(t, plan)

	task := h.ok(t, member, "project.task.create", map[string]any{
		"projectId": projectID, "planId": planID, "title": "Prepare cables",
		"description": "Order and stage.", "priority": "high", "assigneeId": member.id, "dueDate": "2026-11-15",
	})
	taskID := rowID(t, task)

	// Assigning the same person twice must not produce a second AI trigger.
	res, err = h.call(t, member, "project.task.assign", map[string]any{"id": taskID, "assigneeId": member.id})
	rejected(t, res, err, "conflict")

	h.ok(t, member, "project.task.status.change", map[string]any{"id": taskID, "from": "todo", "status": "in_progress"})
	done := h.ok(t, member, "project.task.status.change", map[string]any{"id": taskID, "from": "in_progress", "status": "done"})
	if decodePayload[map[string]any](t, done.Data)["completedAt"] == "" {
		t.Fatal("done must set completedAt")
	}
	reopened := h.ok(t, member, "project.task.status.change", map[string]any{"id": taskID, "from": "done", "status": "todo"})
	if decodePayload[map[string]any](t, reopened.Data)["completedAt"] != "" {
		t.Fatal("reopening must clear completedAt")
	}
	// An illegal edge is refused before it reaches SQL.
	res, err = h.call(t, member, "project.task.status.change", map[string]any{"id": taskID, "from": "todo", "status": "done"})
	rejected(t, res, err, "invalid")

	// Pausing is a member action; archiving is the owner's.
	h.ok(t, member, "project.status.change", map[string]any{"id": projectID, "from": "active", "status": "completed"})
	if _, err := h.call(t, member, "project.status.change", map[string]any{"id": projectID, "from": "completed", "status": "archived"}); contracts.Code(err) != "conflict" {
		t.Fatalf("a non-owner must not archive: %v", err)
	}
	h.ok(t, owner, "project.status.change", map[string]any{"id": projectID, "from": "completed", "status": "archived"})

	// Archived projects are read-only.
	if _, err := h.call(t, owner, "project.task.create", map[string]any{"projectId": projectID, "title": "Too late"}); contracts.Code(err) != "conflict" {
		t.Fatalf("archived project accepted a task: %v", err)
	}
	if _, err := h.call(t, owner, "project.plan.create", map[string]any{
		"projectId": projectID, "title": "Too late", "ownerId": owner.id,
	}); contracts.Code(err) != "conflict" {
		t.Fatalf("archived project accepted a plan: %v", err)
	}
}

func TestProjectMembershipRules(t *testing.T) {
	h := newProjectHarness(t)
	owner := h.account(t, "rules-owner", projects.ReadPermission, projects.WritePermission, projects.AdminPermission)
	member := h.account(t, "rules-member", projects.ReadPermission, projects.WritePermission)
	viewer := h.account(t, "rules-viewer", projects.ReadPermission)
	late := h.account(t, "rules-late", projects.ReadPermission, projects.WritePermission)

	// Creating a project needs the admin permission, not just write.
	if _, err := h.call(t, member, "project.create", map[string]any{"code": "P-1", "name": "Nope"}); contracts.Code(err) != "forbidden" {
		t.Fatalf("member created a project: %v", err)
	}
	projectID := rowID(t, h.ok(t, owner, "project.create", map[string]any{"code": "P-1", "name": "Rules"}))

	// A duplicate code is a conflict inside the tenant.
	if _, err := h.call(t, owner, "project.create", map[string]any{"code": "P-1", "name": "Duplicate"}); contracts.Code(err) != "conflict" {
		t.Fatalf("duplicate code: %v", err)
	}

	h.ok(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": member.id})
	h.ok(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": viewer.id})

	// Only the owner manages members.
	if _, err := h.call(t, member, "project.member.add", map[string]any{"projectId": projectID, "accountId": late.id}); contracts.Code(err) != "forbidden" {
		t.Fatalf("member added a member: %v", err)
	}
	if _, err := h.call(t, member, "project.member.remove", map[string]any{"projectId": projectID, "accountId": viewer.id}); contracts.Code(err) != "forbidden" {
		t.Fatalf("member removed a member: %v", err)
	}
	// The owner cannot leave without handing over.
	if _, err := h.call(t, owner, "project.member.remove", map[string]any{"projectId": projectID, "accountId": owner.id}); contracts.Code(err) != "conflict" {
		t.Fatalf("owner removed themselves: %v", err)
	}
	// Ownership can only move to a member.
	if _, err := h.call(t, owner, "project.owner.transfer", map[string]any{"projectId": projectID, "accountId": late.id}); contracts.Code(err) != "conflict" {
		t.Fatalf("transfer to non-member: %v", err)
	}
	h.ok(t, owner, "project.owner.transfer", map[string]any{"projectId": projectID, "accountId": member.id})
	// After the transfer the previous owner is a plain member and loses the
	// owner-only actions.
	if _, err := h.call(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": late.id}); contracts.Code(err) != "forbidden" {
		t.Fatalf("previous owner still manages members: %v", err)
	}
	h.ok(t, member, "project.member.add", map[string]any{"projectId": projectID, "accountId": late.id})

	// A reader without write may see the project but not write into it.
	if _, err := h.call(t, viewer, "project.task.create", map[string]any{"projectId": projectID, "title": "Read only"}); contracts.Code(err) != "forbidden" {
		t.Fatalf("viewer wrote a task: %v", err)
	}
	if _, err := h.call(t, viewer, "project.task.list", map[string]any{"projectId": projectID}); err != nil {
		t.Fatalf("viewer should read tasks: %v", err)
	}

	// Another tenant cannot see the project even with the same id and permission.
	account := &genidentity.AccountRecord{ID: member.id}
	other, err := h.iam.NewPrincipal(account, "tenant-other", []*genidentity.PermissionGrant{
		{Permission: projects.ReadPermission, Effect: "allow", Scope: "ALL"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherCtx := identity.WithPrincipal(context.Background(), other)
	crossResult, crossErr := h.gw.Invoke(otherCtx, gateway.Request{
		Name: "project.get", Version: "1.0.0", Input: contracts.Input{"id": projectID},
		Origin: contracts.OriginHumanUI,
	})
	if contracts.Code(crossErr) != "forbidden" {
		t.Fatalf("forged cross tenant identity must be forbidden, got result=%+v err=%v", crossResult, crossErr)
	}
}

func TestProjectConcurrencyAndReplay(t *testing.T) {
	h := newProjectHarness(t)
	owner := h.account(t, "race-owner", projects.ReadPermission, projects.WritePermission, projects.AdminPermission)

	input := map[string]any{"code": "RACE-1", "name": "Race"}
	first, err := h.callWith(t, owner, "project.create", input, "race-create", contracts.OriginHumanUI)
	succeeded(t, first, err)
	replay, err := h.callWith(t, owner, "project.create", input, "race-create", contracts.OriginHumanUI)
	succeeded(t, replay, err)
	if !replay.Replayed || replay.InvocationID != first.InvocationID {
		t.Fatal("a replayed create must return the original invocation")
	}
	var count int
	if err = h.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM projects WHERE tenant_id='tenant-a' AND code='RACE-1'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replay created %d projects", count)
	}
	projectID := rowID(t, first)

	// Two concurrent transitions from the same source state: exactly one wins.
	var wg sync.WaitGroup
	outcomes := make([]string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, callErr := h.callWith(t, owner, "project.status.change",
				map[string]any{"id": projectID, "from": "active", "status": "paused"},
				fmt.Sprintf("race-pause-%d", index), contracts.OriginHumanUI)
			if callErr != nil {
				outcomes[index] = contracts.Code(callErr)
				return
			}
			outcomes[index] = "succeeded"
		}(i)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, outcome := range outcomes {
		switch outcome {
		case "succeeded":
			success++
		case "conflict":
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrent transitions: %v", outcomes)
	}
}

func TestProjectBriefSourceIsRestricted(t *testing.T) {
	h := newProjectHarness(t)
	owner := h.account(t, "brief-owner", projects.ReadPermission, projects.WritePermission, projects.AdminPermission)
	projectID := rowID(t, h.ok(t, owner, "project.create", map[string]any{"code": "B-1", "name": "Brief"}))
	taskID := rowID(t, h.ok(t, owner, "project.task.create", map[string]any{"projectId": projectID, "title": "Brief me"}))

	// A page cannot hand-write a model conclusion.
	if _, err := h.callWith(t, owner, "project.task.brief.record", map[string]any{"taskId": taskID, "summary": "forged"},
		"brief-human", contracts.OriginHumanUI); contracts.Code(err) != "forbidden" {
		t.Fatalf("human brief: %v", err)
	}
	// The assistant tool path is an agent origin and may write it.
	first, err := h.callWith(t, owner, "project.task.brief.record", map[string]any{
		"taskId": taskID, "summary": "Start with the connector checklist.", "model": "test-model",
	}, "brief-agent-1", contracts.OriginAgent)
	succeeded(t, first, err)
	second, err := h.callWith(t, owner, "project.task.brief.record", map[string]any{
		"taskId": taskID, "summary": "Revised brief.", "model": "test-model",
	}, "brief-agent-2", contracts.OriginAgent)
	succeeded(t, second, err)

	var count int
	if err := h.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM project_task_briefs WHERE tenant_id='tenant-a' AND task_id=$1", taskID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("one task must keep one brief, got %d", count)
	}
	task := decodePayload[map[string]any](t, h.ok(t, owner, "project.task.get", map[string]any{"id": taskID}).Data)
	if task["brief"] != "Revised brief." {
		t.Fatalf("task get did not project the latest brief: %v", task["brief"])
	}
}

// TestProjectAssignmentIsTriggerable proves the AI entry point exists even when
// no model credential is configured: the assignment writes a fact to the
// outbox, and an enabled policy matches that exact event. Running the model
// itself needs a configured provider and is not part of this check.
func TestProjectAssignmentIsTriggerable(t *testing.T) {
	h := newProjectHarness(t)
	owner := h.account(t, "trigger-owner", projects.ReadPermission, projects.WritePermission, projects.AdminPermission)
	member := h.account(t, "trigger-member", projects.ReadPermission, projects.WritePermission)

	projectID := rowID(t, h.ok(t, owner, "project.create", map[string]any{"code": "T-1", "name": "Trigger"}))
	h.ok(t, owner, "project.member.add", map[string]any{"projectId": projectID, "accountId": member.id})
	taskID := rowID(t, h.ok(t, owner, "project.task.create", map[string]any{"projectId": projectID, "title": "Assigned work"}))

	assigned := h.ok(t, owner, "project.task.assign", map[string]any{"id": taskID, "assigneeId": member.id})
	if len(assigned.EventIDs) != 1 {
		t.Fatalf("assignment should emit exactly one event, got %d", len(assigned.EventIDs))
	}

	var eventType, aggregateType, aggregateID string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT event_type, aggregate_type, aggregate_id FROM cap_outbox WHERE tenant_id='tenant-a' AND event_id=$1`,
		assigned.EventIDs[0]).Scan(&eventType, &aggregateType, &aggregateID); err != nil {
		t.Fatal(err)
	}
	if eventType != "project.task.assigned" || aggregateType != "project_task" || aggregateID != fmt.Sprint(taskID) {
		t.Fatalf("unexpected fact: %s on %s #%s", eventType, aggregateType, aggregateID)
	}

	var agent, policyEvent, mode string
	var maxDepth int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT agent, event_type, mode, max_depth FROM ai_trigger_policies
		 WHERE policy_id='project-task-brief' AND enabled ORDER BY version DESC LIMIT 1`,
	).Scan(&agent, &policyEvent, &mode, &maxDepth); err != nil {
		t.Fatal(err)
	}
	if policyEvent != eventType || agent != "task_briefer" || mode != "async" || maxDepth < 1 {
		t.Fatalf("policy does not match the fact: %s %s %s %d", policyEvent, agent, mode, maxDepth)
	}
}

func TestProjectImportIsAtomicAndIdempotent(t *testing.T) {
	h := newProjectHarness(t)
	admin := h.account(t, "import-admin", projects.ReadPermission, projects.AdminPermission)
	member := h.account(t, "import-member", projects.ReadPermission, projects.WritePermission)
	h.ok(t, admin, "project.create", map[string]any{"code": "TAKEN", "name": "Existing"})
	batch := func(second string) map[string]any {
		return map[string]any{"rows": []any{
			map[string]any{"rowId": 1, "values": map[string]any{"code": "NEW-1", "name": "First"}},
			map[string]any{"rowId": 2, "values": map[string]any{"code": second, "name": "Second"}},
		}}
	}
	count := func(query string, want int) {
		t.Helper()
		var got int
		if err := h.pool.QueryRow(admin.ctx, query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: got %d, want %d, err=%v", query, got, want, err)
		}
	}
	if _, err := h.call(t, member, "project.import.commit", batch("NEW-2")); contracts.Code(err) != "forbidden" {
		t.Fatalf("non-admin commit: %v", err)
	}
	if _, err := h.callWith(t, admin, "project.import.commit", batch("NEW-2"), "agent-import", contracts.OriginAgent); contracts.Code(err) != "forbidden" {
		t.Fatalf("Agent commit: %v", err)
	}
	if _, err := h.call(t, admin, "project.import.commit", batch("TAKEN")); contracts.Code(err) != "conflict" {
		t.Fatalf("existing code: %v", err)
	}
	projectsQuery := "SELECT count(*) FROM projects WHERE tenant_id='tenant-a' AND code LIKE 'NEW-%'"
	eventsQuery := "SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND event_type='project.created'"
	count(projectsQuery, 0)
	count(eventsQuery, 1)

	input := batch("NEW-2")
	var wg sync.WaitGroup
	results := make([]*contracts.Result, 2)
	errs := make([]error, len(results))
	for i := range results {
		wg.Go(func() {
			results[i], errs[i] = h.callWith(t, admin, "project.import.commit", input, "same-import", contracts.OriginHumanUI)
		})
	}
	wg.Wait()
	for i := range results {
		succeeded(t, results[i], errs[i])
		if results[i].InvocationID != results[0].InvocationID || len(results[i].EventIDs) != 2 {
			t.Fatalf("unexpected replay: %+v", results[i])
		}
	}
	replay, err := h.callWith(t, admin, "project.import.commit", input, "same-import", contracts.OriginHumanUI)
	succeeded(t, replay, err)
	if !replay.Replayed {
		t.Fatal("completed import did not replay")
	}
	count(projectsQuery, 2)
	count(eventsQuery, 3)
	count("SELECT count(*) FROM project_members m JOIN projects p ON p.id=m.project_id AND p.tenant_id=m.tenant_id WHERE p.tenant_id='tenant-a' AND p.code LIKE 'NEW-%' AND m.account_id=p.owner_id", 2)
}

func TestProjectImportRuntimeParsesSelectedWorkbookAndCommitsDraft(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	const plugin = "com.horizon.projects"
	invoke := func(name string, input map[string]any) map[string]any {
		t.Helper()
		result := invokeCapability(t, client, name, map[string]any{
			"input": input, "idempotencyKey": name + "-import-runtime",
		})
		if result.Status != "succeeded" {
			t.Fatal(result)
		}
		return result.Data.(map[string]any)
	}
	current := invoke("application.runtime.installation.get", map[string]any{"pluginId": plugin})["installation"].(map[string]any)
	if current["version"] != "2.0.3" {
		invoke("application.runtime.installation.upgrade", map[string]any{
			"pluginId": plugin, "version": "2.0.3", "revision": current["revision"],
		})
	}
	resolved := invoke("application.runtime.resolve", map[string]any{"applicationId": "projects"})
	ref := map[string]any{
		"applicationId": "projects", "pageId": "project.import",
		"applicationVersion": resolved["applicationVersion"], "definitionDigest": resolved["definitionDigest"],
		"metadataVersion": resolved["metadataVersion"], "catalogRevision": resolved["catalogRevision"],
		"installationRevision": resolved["installationRevision"],
	}
	protocol := resolved["protocol"].(map[string]any)
	raw, err := json.Marshal(protocol)
	if err != nil || !strings.Contains(string(raw), "document.excel.parse@1.0.0") || !strings.Contains(string(raw), "project.import.commit@1.0.0") {
		t.Fatalf("import actions missing from projected application: %v", err)
	}
	content, err := os.ReadFile("../../apps/web/public/templates/projects-import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	parsed := invokeCapability(t, client, "document.excel.parse", map[string]any{
		"input": map[string]any{"content": base64.StdEncoding.EncodeToString(content), "sheet": "Projects"}, "runtime": ref,
	})
	if parsed.Status != "succeeded" || len(parsed.Data.(map[string]any)["rows"].([]any)) != 4 {
		t.Fatal(parsed)
	}
	var storedInput, storedResult string
	if err := pool.QueryRow(context.Background(), "SELECT input::text, result::text FROM cap_invocations WHERE invocation_id=$1", parsed.InvocationID).Scan(&storedInput, &storedResult); err != nil {
		t.Fatal(err)
	}
	if storedInput != "{}" || strings.Contains(storedResult, "000101") {
		t.Fatal("workbook content persisted in the invocation")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var input struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || len(input.Messages) != 2 ||
			!strings.Contains(input.Messages[1].Content, "WEEK-201") || strings.Contains(input.Messages[1].Content, "000101") {
			t.Error("model did not receive the expected selected worksheet text")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		draft := `{"rows":[{"sourceRow":2,"values":{"code":"WEEK-201","name":"备件货位优化","priority":"high","dueDate":"2027-03-05"}},{"sourceRow":3,"values":{"code":"WEEK-202","name":"工装借用流程梳理","priority":"normal"}},{"sourceRow":4,"values":{"code":"WEEK-203","name":"来料检验记录归档","priority":"urgent","dueDate":"2027-04-12"}}]}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"test","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}]}`, draft)
	}))
	defer upstream.Close()
	core := governance.New(pool, nil, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	secret, err := core.Seal("tenant-a", "workspace:0", "assistant.token", `"test-only-token"`)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id) VALUES('tenant-a','workspace:0','workspace',0) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"assistant.enabled": true, "agent.enabled": true, "agent.baseUrl": upstream.URL + "/v1", "assistant.model": "test", "assistant.token": secret} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO gov_values(tenant_id,scope_key,config_key,value_json) VALUES('tenant-a','workspace:0',$1,$2) ON CONFLICT(tenant_id,scope_key,config_key) DO UPDATE SET value_json=EXCLUDED.value_json", key, string(encoded)); err != nil {
			t.Fatal(err)
		}
	}
	aiFile, err := os.ReadFile("../ai/workbook/testdata/projects-ai-import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	extracted := invokeCapability(t, client, "project.import.extract", map[string]any{
		"input": map[string]any{"content": base64.StdEncoding.EncodeToString(aiFile), "sheet": "项目周报"}, "runtime": ref,
	})
	if extracted.Status != "succeeded" || len(extracted.Data.(map[string]any)["rows"].([]any)) != 3 {
		t.Fatal(extracted)
	}
	if err := pool.QueryRow(ctx, "SELECT input::text, result::text FROM cap_invocations WHERE invocation_id=$1", extracted.InvocationID).Scan(&storedInput, &storedResult); err != nil {
		t.Fatal(err)
	}
	if storedInput != "{}" || strings.Contains(storedResult, "WEEK-201") {
		t.Fatal("extracted values persisted in the invocation")
	}
	result := invokeCapability(t, client, "project.import.commit", map[string]any{
		"input": map[string]any{"rows": []any{
			map[string]any{"rowId": 1, "values": map[string]any{"code": "000101", "name": "Reviewed project", "priority": "high", "dueDate": "2027-01-15"}},
		}}, "runtime": ref, "idempotencyKey": "reviewed-project-draft",
	})
	if result.Status != "succeeded" || len(result.EventIds) != 1 {
		t.Fatal(result)
	}
	var saved string
	if err := pool.QueryRow(context.Background(), "SELECT name FROM projects WHERE tenant_id='tenant-a' AND code='000101'").Scan(&saved); err != nil || saved != "Reviewed project" {
		t.Fatalf("reviewed value: %q, %v", saved, err)
	}
}

//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	"horizon/foundation/system/identity"
	"horizon/foundation/system/memory"
	"horizon/internal/repository/dbgen"
)

func TestGroupMemoryUsesFormalReadAndWriteScopes(t *testing.T) {
	h := newProjectHarness(t)
	m := memory.New(h.pool)
	m.AttachIdentity(h.iam)
	h.gw = gateway.New(gateway.Options{Registry: registry.New(m.Entries()), Identity: h.iam, Pool: h.pool,
		Approvals: approval.NewStore(h.pool), Audit: audit.NewStore(h.pool), Events: eventhub.NewStore(h.pool)})
	base := h.account(t, "group-memory-user")
	account := &genidentity.AccountRecord{ID: base.id, Username: base.name, Status: "active", OrganizationID: h.rootAccount.OrganizationID, DepartmentID: h.rootAccount.DepartmentID}
	grants := []*genidentity.PermissionGrant{
		{Permission: "memory:read", Effect: "allow", Scope: "ALL"}, {Permission: "memory:write", Effect: "allow", Scope: "ALL"},
		{Permission: "departments:read", Effect: "allow", Scope: "DEPARTMENT"}, {Permission: "departments:update", Effect: "allow", Scope: "ORGANIZATION"},
		{Permission: "organizations:read", Effect: "allow", Scope: "ORGANIZATION"}, {Permission: "organizations:update", Effect: "allow", Scope: "ORGANIZATION"},
	}
	raw, err := json.Marshal(grants)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides=$2 WHERE tenant_id='tenant-a' AND id=$1`, base.id, raw); err != nil {
		t.Fatal(err)
	}
	p, err := h.iam.NewPrincipal(account, "tenant-a", grants)
	if err != nil {
		t.Fatal(err)
	}
	actor := projectActor{name: base.name, id: base.id, ctx: identity.WithPrincipal(context.Background(), p)}
	var otherDept int64
	err = h.pool.QueryRow(context.Background(), `INSERT INTO iam_departments(tenant_id,organization_id,name,code,status) VALUES('tenant-a',$1,'Other','OTHER-MEM','active') RETURNING id`, account.OrganizationID).Scan(&otherDept)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.pool.Exec(context.Background(), `INSERT INTO memory_events (tenant_id,event_id,event_type,event_version,occurred_at,actor_id,actor_type,source_type,resource_type,resource_id) VALUES ('tenant-a','dept-source','identity.department.updated','1.0',now(),$1,'user','domain_event','department',$2)`, strconv.FormatInt(account.ID, 10), strconv.FormatInt(account.DepartmentID, 10))
	if err != nil {
		t.Fatal(err)
	}
	validTo := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano)
	own := map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "partition": "business", "memoryType": "note", "title": "Department", "summary": "Private group text", "sourceEventId": "dept-source", "validTo": validTo}
	created := h.ok(t, actor, "memory.group.create", own)
	id := rowID(t, created)
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = dbgen.New(tx).MemoryLockGroupDepartment(context.Background(), dbgen.MemoryLockGroupDepartmentParams{TenantID: "tenant-a", ScopeID: strconv.FormatInt(account.DepartmentID, 10), ReadAll: true, WriteAll: true, ActorID: account.ID}); err != nil {
		t.Fatal(err)
	}
	contender, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contender.Exec(context.Background(), `SET LOCAL lock_timeout='200ms'`); err != nil {
		t.Fatal(err)
	}
	_, err = contender.Exec(context.Background(), `UPDATE iam_departments SET name='Concurrent' WHERE tenant_id='tenant-a' AND id=$1`, account.DepartmentID)
	var lockError *pgconn.PgError
	if !errors.As(err, &lockError) || lockError.Code != "55P03" {
		t.Fatalf("department mutation was not blocked: %v", err)
	}
	if err = contender.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	sources := h.ok(t, actor, "memory.group.sources", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id})
	if len(sources.Data.(map[string]any)["sources"].([]map[string]any)) != 1 {
		t.Fatal("authorized source missing")
	}
	if _, err := h.call(t, h.root, "memory.group.create", map[string]any{"scopeType": "department", "scopeId": otherDept, "partition": "business", "memoryType": "note", "title": "Wrong source", "summary": "", "sourceEventId": "dept-source"}); err == nil {
		t.Fatal("other department source accepted")
	}
	if _, err := h.call(t, actor, "memory.group.create", map[string]any{"scopeType": "department", "scopeId": otherDept, "partition": "business", "memoryType": "note", "title": "Denied", "summary": ""}); err == nil {
		t.Fatal("broader write scope bypassed narrower read scope")
	}
	if _, err := h.call(t, actor, "memory.group.get", map[string]any{"scopeType": "department", "scopeId": otherDept, "id": id}); err == nil {
		t.Fatal("out-of-scope department read succeeded")
	}
	groupMemory, err := h.call(t, actor, "memory.group.get", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id})
	if err != nil {
		t.Fatal(err)
	}
	if groupMemory.Data.(map[string]any)["validTo"] != validTo {
		t.Fatal("group validity not preserved")
	}
	temporary, err := h.iam.NewPrincipal(account, "tenant-a", grants)
	if err != nil {
		t.Fatal(err)
	}
	temporary.PermanentGrants = []*genidentity.PermissionGrant{{Permission: "memory:read", Effect: "allow", Scope: "ALL"}}
	temporary.TemporaryScopes["departments:read"] = []identity.Scope{{TenantID: "tenant-a", DeptIDs: []int64{account.DepartmentID}}}
	if _, err = h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides='[{"permission":"memory:read","effect":"allow","scope":"ALL"}]'::jsonb WHERE tenant_id='tenant-a' AND id=$1`, account.ID); err != nil {
		t.Fatal(err)
	}
	temporaryActor := projectActor{name: "temporary", id: account.ID, ctx: identity.WithPrincipal(context.Background(), temporary)}
	if _, err := h.call(t, temporaryActor, "memory.group.get", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id}); err == nil {
		t.Fatal("temporary resource grant allowed group memory access")
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE iam_accounts SET overrides=$2 WHERE tenant_id='tenant-a' AND id=$1`, account.ID, raw); err != nil {
		t.Fatal(err)
	}
	h.ok(t, actor, "memory.group.correct", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id, "version": 1, "title": "Corrected", "summary": ""})
	if _, err := h.call(t, actor, "memory.group.correct", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id, "version": 1, "title": "Stale", "summary": ""}); err == nil {
		t.Fatal("stale correction succeeded")
	}
	h.ok(t, actor, "memory.group.archive", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id, "version": 2})
	archived := h.ok(t, actor, "memory.group.list", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "status": "archived"})
	if len(archived.Data.(map[string]any)["items"].([]map[string]any)) != 1 {
		t.Fatal("group archived memory missing")
	}
	otherMemory := h.ok(t, h.root, "memory.group.create", map[string]any{"scopeType": "department", "scopeId": otherDept, "partition": "business", "memoryType": "note", "title": "Other", "summary": ""})
	h.ok(t, h.root, "memory.group.archive", map[string]any{"scopeType": "department", "scopeId": otherDept, "id": rowID(t, otherMemory), "version": 1})
	outside, err := h.call(t, actor, "memory.group.list", map[string]any{"scopeType": "department", "scopeId": otherDept, "status": "archived"})
	if err == nil && len(outside.Data.(map[string]any)["items"].([]map[string]any)) != 0 {
		t.Fatal("out-of-scope archived group memory exposed")
	}
	h.ok(t, actor, "memory.group.delete", map[string]any{"scopeType": "department", "scopeId": account.DepartmentID, "id": id, "version": 3})
	var copies int
	if err := h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM memory_corrections WHERE tenant_id='tenant-a' AND memory_id=$1)+(SELECT count(*) FROM memory_sources WHERE tenant_id='tenant-a' AND memory_id=$1)`, id).Scan(&copies); err != nil || copies != 0 {
		t.Fatalf("correction copies remain: %d %v", copies, err)
	}
	org := h.ok(t, actor, "memory.group.create", map[string]any{"scopeType": "organization", "scopeId": account.OrganizationID, "partition": "knowledge", "memoryType": "policy", "title": "Organization", "summary": ""})
	if _, err := h.call(t, actor, "memory.group.get", map[string]any{"scopeType": "organization", "scopeId": account.OrganizationID, "id": rowID(t, org)}); err != nil {
		t.Fatal(err)
	}
	foreignAccount, err := identity.Bootstrap(context.Background(), h.pool, "tenant-b", "group-foreign-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	foreignPrincipal, err := h.iam.PrincipalForTask(context.Background(), "tenant-b", foreignAccount.ID, foreignAccount.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	foreign := projectActor{name: "foreign", id: foreignAccount.ID, ctx: identity.WithPrincipal(context.Background(), foreignPrincipal)}
	if _, err := h.call(t, foreign, "memory.group.get", map[string]any{"scopeType": "organization", "scopeId": account.OrganizationID, "id": rowID(t, org)}); err == nil {
		t.Fatal("cross-tenant group read succeeded")
	}
}

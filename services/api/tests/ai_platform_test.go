//go:build integration

package tests

import (
	gencapability "baseline/services/api/gen/capability"
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/ai/triggers"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/eventhub"
	"baseline/services/api/internal/platform/outbox"
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

type unavailableLauncher struct{}

func (unavailableLauncher) RunTrigger(context.Context, harness.TriggerInput) (*harness.Run, error) {
	return nil, governance.Problem("unavailable")
}

func invokeCapability(t *testing.T, client iamClient, name string, body map[string]any) gencapability.CapabilityResult {
	t.Helper()
	response := client.request(t, "POST", "/api/capabilities/"+name+"/invocations", body, true)
	checkStatus(t, response, 200)
	return decodeBody[gencapability.CapabilityResult](t, response)
}

func TestCapabilityGatewayClosedLoop(t *testing.T) {
	handler, pool, _ := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()

	definitions := decodeBody[[]*gencapability.CapabilityContract](t, owner.request(t, "GET", "/api/capabilities", nil, false))
	byName := map[string]*gencapability.CapabilityContract{}
	for _, item := range definitions {
		byName[item.Name] = item
	}
	if byName["example.record.create"] == nil || len(byName["example.record.create"].SuccessEvents) != 1 {
		t.Fatalf("create capability must publish its success event: %+v", byName["example.record.create"])
	}
	if byName["notification.send"] == nil || byName["notification.send"].Risk != "high" || byName["notification.send"].Approval != "confirm" {
		t.Fatal("notification.send must keep its high risk confirmation policy")
	}

	create := invokeCapability(t, owner, "example.record.create", map[string]any{
		"input":          map[string]any{"title": "Line 3 changeover"},
		"idempotencyKey": "create-1",
	})
	if create.Status != "succeeded" || len(create.EventIds) != 1 {
		t.Fatalf("create must succeed with one event: %+v", create)
	}
	if create.Replayed {
		t.Fatal("first call must not be a replay")
	}
	var stored int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM example_records WHERE tenant_id='tenant-a' AND title='Line 3 changeover'").Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("business write missing: %d %v", stored, err)
	}
	var pending int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND status='pending'").Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("outbox row missing: %d %v", pending, err)
	}

	replay := invokeCapability(t, owner, "example.record.create", map[string]any{
		"input":          map[string]any{"title": "Line 3 changeover"},
		"idempotencyKey": "create-1",
	})
	if !replay.Replayed || replay.InvocationID != create.InvocationID {
		t.Fatalf("same key must replay the same invocation: %+v", replay)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM example_records WHERE tenant_id='tenant-a' AND title='Line 3 changeover'").Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("replay must not write twice: %d %v", stored, err)
	}

	t.Run("concurrent retries converge on one invocation", func(t *testing.T) {
		results := make(chan gencapability.CapabilityResult, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				response := owner.request(t, "POST", "/api/capabilities/example.record.create/invocations", map[string]any{
					"input":          map[string]any{"title": "Raced record"},
					"idempotencyKey": "create-race",
				}, true)
				if response.Code != 200 {
					results <- gencapability.CapabilityResult{}
					return
				}
				results <- decodeBody[gencapability.CapabilityResult](t, response)
			}()
		}
		wg.Wait()
		close(results)
		first, second := <-results, <-results
		if first.InvocationID == "" || second.InvocationID == "" {
			t.Fatal("both concurrent calls must succeed or replay")
		}
		if first.InvocationID != second.InvocationID {
			t.Fatalf("expected one invocation, got %s and %s", first.InvocationID, second.InvocationID)
		}
		if first.Replayed == second.Replayed {
			t.Fatalf("exactly one call should be a replay: %+v %+v", first, second)
		}
	})

	// Publish the outbox and let the trigger engine evaluate the seeded policy.
	hub := eventhub.NewHub()
	engine := triggers.NewEngine(triggers.NewStore(pool), hub, unavailableLauncher{}, identity.New(pool, false))
	engine.Start(ctx)
	relay := outbox.NewRelay(pool, hub)
	if _, err := relay.Once(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		events := decodeBody[[]*gencapability.CapabilityEvent](t, owner.request(t, "GET", "/api/capabilities/events", nil, false))
		found := false
		for _, event := range events {
			if event.Type == "example.record.created" && event.AiStatus == "skipped" {
				found = true
			}
			if event.Type == "example.record.created" && event.InvocationID == "" {
				t.Fatalf("event lost its invocation: %+v", event)
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("trigger execution did not settle: %+v", events)
		}
		time.Sleep(100 * time.Millisecond)
	}

	invocations := decodeBody[[]*gencapability.CapabilityInvocation](t, owner.request(t, "GET", "/api/capabilities/invocations", nil, false))
	if len(invocations) < 2 {
		t.Fatalf("expected audited invocations, got %d", len(invocations))
	}
	for _, invocation := range invocations {
		if invocation.Origin != "human-ui" || invocation.TraceID == "" {
			t.Fatalf("audit projection incomplete: %+v", invocation)
		}
	}
}

func TestCapabilityPermissionsAndTenantIsolation(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()

	company := root.OrganizationID
	createReader := owner.invoke(t, "identity.account.create", map[string]any{
		"username": "example-reader", "fullName": "example-reader", "organizationId": company,
		"departmentId": root.DepartmentID, "status": "active", "roleIds": []int64{},
		"overrides": []map[string]any{{"permission": "example:read", "effect": "allow", "scope": "ALL"}},
		"password":  "TestOnly!2026Secure",
	}, true)
	checkStatus(t, createReader, 200)
	createdResult := decodeBody[gencapability.CapabilityResult](t, createReader)
	if createdResult.Status != "succeeded" {
		t.Fatalf("account create failed: %+v", createdResult)
	}
	snapshot := decodeBody[*genidentity.AdminSnapshot](t, owner.request(t, "GET", "/api/admin/snapshot", nil, false))
	var reader *genidentity.AccountRecord
	for _, account := range snapshot.Accounts {
		if account.Username == "example-reader" {
			reader = account
		}
	}
	if reader == nil {
		t.Fatal("created account missing from the snapshot")
	}
	readerClient := loginIAM(t, handler, reader.Username)

	definitions := decodeBody[[]*gencapability.CapabilityContract](t, readerClient.request(t, "GET", "/api/capabilities", nil, false))
	for _, definition := range definitions {
		if definition.Name == "example.record.create" || definition.Name == "notification.send" {
			t.Fatalf("reader must not discover %s", definition.Name)
		}
	}
	checkStatus(t, readerClient.request(t, "POST", "/api/capabilities/example.record.create/invocations", map[string]any{
		"input": map[string]any{"title": "forbidden"}, "idempotencyKey": "reader-1",
	}, true), 403)
	if result := invokeCapability(t, readerClient, "example.record.list", map[string]any{"input": map[string]any{}}); result.Status != "succeeded" {
		t.Fatalf("reader may list: %+v", result)
	}

	created := invokeCapability(t, owner, "example.record.create", map[string]any{
		"input": map[string]any{"title": "tenant-a only"}, "idempotencyKey": "isolation-1",
	})
	recordID := int64(created.Data.(map[string]any)["id"].(float64))

	if _, err := identity.Bootstrap(ctx, pool, "tenant-b", "tenant-b-admin", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	other := loginIAM(t, handler, "tenant-b-admin")
	events := decodeBody[[]*gencapability.CapabilityEvent](t, other.request(t, "GET", "/api/capabilities/events", nil, false))
	for _, event := range events {
		if event.Type == "example.record.created" {
			t.Fatal("tenant-b must not see tenant-a events")
		}
	}
	archived := invokeCapability(t, other, "example.record.archive", map[string]any{
		"input": map[string]any{"id": recordID}, "idempotencyKey": fmt.Sprintf("cross-%d", recordID),
	})
	if archived.Status != "failed" || archived.ErrorCode == "" {
		t.Fatalf("cross tenant archive must fail: %+v", archived)
	}
}

func TestCapabilityApprovalResume(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()

	seed := func(title string) (int64, int64, string) {
		created := invokeCapability(t, owner, "example.record.create", map[string]any{
			"input": map[string]any{"title": title}, "idempotencyKey": "seed-" + title,
		})
		recordID := int64(created.Data.(map[string]any)["id"].(float64))
		invocationID := fmt.Sprintf("inv_%d", time.Now().UnixNano())
		digest := fmt.Sprintf("digest-%d", time.Now().UnixNano())
		if _, err := pool.Exec(ctx, `
INSERT INTO cap_invocations (tenant_id, account_id, department_id, invocation_id, capability, capability_version, origin, status, input, input_digest, correlation_id, trace_id)
VALUES ('tenant-a', $1, $2, $3, 'example.record.archive', '1.0.0', 'agent', 'awaiting_approval', $4::jsonb, $5, 'trace-approval', 'trace-approval')`,
			root.ID, root.DepartmentID, invocationID,
			fmt.Sprintf(`{"id":%d}`, recordID), digest); err != nil {
			t.Fatal(err)
		}
		var approvalID int64
		if err := pool.QueryRow(ctx, `
INSERT INTO cap_approvals (tenant_id, account_id, department_id, capability, capability_version, invocation_id, risk, approval, status, arguments, arguments_digest, expires_at)
VALUES ('tenant-a', $1, $2, 'example.record.archive', '1.0.0', $3, 'high', 'confirm', 'pending', $4::jsonb, $5, now() + interval '10 minutes')
RETURNING id`, root.ID, root.DepartmentID, invocationID, fmt.Sprintf(`{"id":%d}`, recordID), digest).Scan(&approvalID); err != nil {
			t.Fatal(err)
		}
		return recordID, approvalID, invocationID
	}

	t.Run("approval resumes under the requester authority", func(t *testing.T) {
		recordID, approvalID, _ := seed("approval-approved")
		cards := decodeBody[[]*gencapability.CapabilityApproval](t, owner.request(t, "GET", "/api/capabilities/approvals", nil, false))
		found := false
		for _, card := range cards {
			if card.ID == approvalID {
				found = card.Mine && card.Status == "pending" && card.Arguments != nil
			}
		}
		if !found {
			t.Fatalf("pending card missing: %+v", cards)
		}
		result := decodeBody[gencapability.CapabilityResult](t, owner.request(t, "POST", "/api/capabilities/approvals/"+itoa(approvalID)+"/decisions", map[string]any{"approve": true}, true))
		if result.Status != "succeeded" || len(result.EventIds) != 1 {
			t.Fatalf("approved archive must succeed with an event: %+v", result)
		}
		var status string
		if err := pool.QueryRow(ctx, "SELECT status FROM example_records WHERE tenant_id='tenant-a' AND id=$1", recordID).Scan(&status); err != nil || status != "archived" {
			t.Fatalf("record must be archived: %s %v", status, err)
		}
		checkStatus(t, owner.request(t, "POST", "/api/capabilities/approvals/"+itoa(approvalID)+"/decisions", map[string]any{"approve": true}, true), 409)
	})

	t.Run("rejection closes the invocation without side effects", func(t *testing.T) {
		recordID, approvalID, invocationID := seed("approval-rejected")
		result := decodeBody[gencapability.CapabilityResult](t, owner.request(t, "POST", "/api/capabilities/approvals/"+itoa(approvalID)+"/decisions", map[string]any{"approve": false}, true))
		if result.Status != "cancelled" || result.ErrorCode != "rejected" {
			t.Fatalf("rejection must cancel: %+v", result)
		}
		var status string
		if err := pool.QueryRow(ctx, "SELECT status FROM example_records WHERE tenant_id='tenant-a' AND id=$1", recordID).Scan(&status); err != nil || status != "draft" {
			t.Fatalf("rejected archive must not change the record: %s %v", status, err)
		}
		if err := pool.QueryRow(ctx, "SELECT status FROM cap_invocations WHERE tenant_id='tenant-a' AND invocation_id=$1", invocationID).Scan(&status); err != nil || status != "cancelled" {
			t.Fatalf("invocation must close as cancelled: %s %v", status, err)
		}
	})

	t.Run("another tenant cannot decide", func(t *testing.T) {
		_, approvalID, _ := seed("approval-foreign")
		if _, err := identity.Bootstrap(ctx, pool, "tenant-d", "tenant-d-admin", "TestOnly!2026Secure"); err != nil {
			t.Fatal(err)
		}
		other := loginIAM(t, handler, "tenant-d-admin")
		checkStatus(t, other.request(t, "POST", "/api/capabilities/approvals/"+itoa(approvalID)+"/decisions", map[string]any{"approve": true}, true), 404)
	})
}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }

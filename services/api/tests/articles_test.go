//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/modules/articles"
	"baseline/services/api/internal/platform/approval"
	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/eventhub"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestArticleModeration(t *testing.T) {
	handler, pool, root := iamFixture(t)
	iam := identity.New(pool, false)
	ctx := context.Background()
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	m := &articles.Module{Pool: pool}
	g := gateway.New(gateway.Options{Registry: registry.New(m.Entries()), Identity: iam, Pool: pool, Audit: audit.NewStore(pool), Approvals: approval.NewStore(pool), Events: eventhub.NewStore(pool)})
	m.Gateway = g
	owner := loginIAM(t, handler, "root-admin")
	response := owner.invoke(t, "identity.account.create", map[string]any{"username": "article-reviewer", "fullName": "Reviewer", "organizationId": root.OrganizationID, "departmentId": root.DepartmentID, "status": "active", "roleIds": []int64{}, "overrides": []map[string]any{{"permission": "articles:read", "effect": "allow", "scope": "ALL"}, {"permission": "articles:review", "effect": "allow", "scope": "ALL"}}, "password": "TestOnly!2026Secure"}, true)
	checkStatus(t, response, 200)
	reviewer := decodeData[genidentity.AccountRecord](t, response)
	reviewerP, err := iam.PrincipalForTask(ctx, "tenant-a", reviewer.ID, reviewer.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	reviewerCtx := identity.WithPrincipal(context.Background(), reviewerP)
	seq := 0
	call := func(name string, input contracts.Input) *contracts.Result {
		t.Helper()
		seq++
		r, e := g.Invoke(ctx, gateway.Request{Name: "article." + name, Input: input, Origin: contracts.OriginHumanUI, IdempotencyKey: fmt.Sprint(seq)})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	success := func(r *contracts.Result) {
		t.Helper()
		if r.Status != contracts.StatusSucceeded {
			t.Fatalf("failed: %+v", r)
		}
	}
	for _, verdict := range []string{"safe", "blocked", "suspected", "unavailable"} {
		t.Run(verdict, func(t *testing.T) {
			m.Classify = func(context.Context, string) (string, error) { return verdict, nil }
			created := call("create", contracts.Input{"title": "Article", "body": "private article body"})
			success(created)
			raw, _ := json.Marshal(created.Data)
			var id struct {
				ID int64 `json:"id"`
			}
			if err = json.Unmarshal(raw, &id); err != nil {
				t.Fatal(err)
			}
			in := contracts.Input{"id": id.ID, "revision": int64(1)}
			success(call("submit", in))
			success(call("assess", contracts.Input{"id": id.ID}))
			var state string
			if err = pool.QueryRow(ctx, "SELECT status FROM articles WHERE id=$1", id.ID).Scan(&state); err != nil {
				t.Fatal(err)
			}
			expected := "pending_review"
			if verdict == "safe" {
				expected = "published"
			}
			if state != expected {
				t.Fatalf("%s want %s", state, expected)
			}
			// Neither a forged verdict nor a stale edit can publish content.
			if _, e := g.Invoke(ctx, gateway.Request{Name: "article.apply", Origin: contracts.OriginHumanUI, IdempotencyKey: "forged-" + verdict, Input: in}); contracts.Code(e) != "forbidden" {
				t.Fatalf("forged verdict error=%v", e)
			}
			update := call("update", contracts.Input{"id": id.ID, "revision": int64(1), "title": "Changed", "body": "new body"})
			if update.Status == contracts.StatusSucceeded {
				t.Fatal("edited submitted article")
			}
			if verdict != "safe" {
				r := call("decide", contracts.Input{"id": id.ID, "revision": int64(1), "approve": true})
				if r.Status == contracts.StatusSucceeded {
					t.Fatal("self approval")
				}
			}
			if verdict != "safe" {
				// Automation cannot supply a human review even when the principal has review permission.
				r, e := g.Invoke(reviewerCtx, gateway.Request{Name: "article.decide", Input: contracts.Input{"id": id.ID, "revision": int64(1), "approve": true}, Origin: contracts.OriginAgent, IdempotencyKey: "agent-" + verdict})
				if e == nil && r.Status == contracts.StatusSucceeded {
					t.Fatal("automatic human approval")
				}
				r, e = g.Invoke(reviewerCtx, gateway.Request{Name: "article.decide", Input: contracts.Input{"id": id.ID, "revision": int64(2), "approve": true}, Origin: contracts.OriginHumanUI, IdempotencyKey: "stale-" + verdict})
				if e == nil && r.Status == contracts.StatusSucceeded {
					t.Fatal("stale approval")
				}
				r, e = g.Invoke(reviewerCtx, gateway.Request{Name: "article.decide", Input: contracts.Input{"id": id.ID, "revision": int64(1), "approve": true}, Origin: contracts.OriginHumanUI, IdempotencyKey: "approve-" + verdict})
				if e != nil {
					t.Fatal(e)
				}
				success(r)
				r, e = g.Invoke(reviewerCtx, gateway.Request{Name: "article.decide", Input: contracts.Input{"id": id.ID, "revision": int64(1), "approve": true}, Origin: contracts.OriginHumanUI, IdempotencyKey: "duplicate-" + verdict})
				if e == nil && r.Status == contracts.StatusSucceeded {
					t.Fatal("duplicate approval")
				}
			}
			// Body is returned live but never persisted in the invocation result.
			detail := call("get", contracts.Input{"id": id.ID})
			success(detail)
			var leaks int
			if err = pool.QueryRow(ctx, "SELECT count(*) FROM cap_invocations WHERE input::text LIKE '%private article body%' OR result::text LIKE '%private article body%'").Scan(&leaks); err != nil || leaks != 0 {
				t.Fatalf("body leaked: %d %v", leaks, err)
			}
			// Cross-tenant owner ids do not confer access.
			other := *p
			other.TenantID = "tenant-b"
			r, e := g.Invoke(identity.WithPrincipal(ctx, &other), gateway.Request{Name: "article.get", Input: contracts.Input{"id": id.ID}, Origin: contracts.OriginHumanUI})
			if e == nil && r.Status == contracts.StatusSucceeded {
				t.Fatal("cross tenant read")
			}
		})
	}
	t.Run("event routes to dedicated review and records independent AI run", func(t *testing.T) {
		core := governance.New(pool, iam, governance.Options{})
		service := harness.New(core, registry.New(m.Entries()), g, harness.NewStore(pool), audit.NewStore(pool), nil)
		m.Classify = service.ClassifyArticle
		created := call("create", contracts.Input{"title": "Event review", "body": "sensitive event article body"})
		success(created)
		raw, _ := json.Marshal(created.Data)
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatal(err)
		}
		success(call("submit", contracts.Input{"id": item.ID, "revision": int64(1)}))
		run, err := service.RunTrigger(ctx, harness.TriggerInput{PolicyID: "article-moderation", Event: eventhub.Event{Type: "article.submitted", EventID: "article-event-test", AggregateID: fmt.Sprint(item.ID), CorrelationID: "article-correlation"}})
		if err != nil {
			t.Fatal(err)
		}
		if run.ID == 0 || run.Status != harness.StatusFailed {
			t.Fatal("trigger must reference the independent failed AI run")
		}
		var status string
		if err = pool.QueryRow(ctx, "SELECT status FROM articles WHERE id=$1", item.ID).Scan(&status); err != nil || status != "pending_review" {
			t.Fatalf("fallback status=%s %v", status, err)
		}
		var count int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM ai_runs WHERE correlation_id='article-correlation' AND agent='article_moderator' AND status='failed'").Scan(&count); err != nil || count != 1 {
			t.Fatalf("AI run missing %d %v", count, err)
		}
		for i := 0; i < 2; i++ {
			result, err := g.Invoke(ctx, gateway.Request{Name: "article.get", Input: contracts.Input{"id": item.ID}, Origin: contracts.OriginHumanUI, IdempotencyKey: "same-private-read"})
			if err != nil {
				t.Fatal(err)
			}
			success(result)
			if result.Replayed {
				t.Fatal("private result replayed")
			}
			raw, _ := json.Marshal(result.Data)
			if len(raw) == 0 {
				t.Fatal("private result missing")
			}
		}
	})

}

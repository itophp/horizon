// Package articles implements moderated publishing through the capability gateway.
package articles

import (
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"unicode"
)

type Module struct {
	Pool *pgxpool.Pool
	// Blocked reports whether the tenant's published blocked-word policy matches
	// the content, and whether that policy is usable at all. Injecting it keeps
	// this module independent of the settings subsystem.
	Blocked func(context.Context, *identity.Principal, string) (bool, bool, error)
}

// events names the fact each write capability produces. A single map keeps the
// declaration and the executor from drifting into "article.rejectd" style typos.
var events = map[string]string{
	"create":        "article.created",
	"update":        "article.updated",
	"submit":        "article.submitted",
	"record-review": "article.reviewed",
	"publish":       "article.published",
	"reject":        "article.rejected",
	"decide":        "article.decided",
}

func (m *Module) Entries() []registry.Entry {
	entries := []registry.Entry{}
	for _, action := range []string{"list", "get", "create", "update", "submit", "record-review", "publish", "reject", "decide"} {
		d := definition(action)
		h := contracts.Handler{}
		switch action {
		case "list":
			h.Execute = m.list
		case "get":
			h.Execute = m.get
		default:
			a := action
			h.ExecuteTx = func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.write(ctx, tx, in, a)
			}
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: h})
	}
	return entries
}
func str(max int) *contracts.Schema {
	min := 1
	return &contracts.Schema{Type: "string", MinLength: &min, MaxLength: &max}
}
func definition(a string) contracts.Definition {
	input := map[string]*contracts.Schema{}
	required := []string{}
	if a != "list" && a != "create" {
		input["id"] = contracts.Int(1, 1<<53)
		required = append(required, "id")
	}
	if a == "update" || a == "submit" || a == "record-review" || a == "publish" || a == "reject" || a == "decide" {
		input["revision"] = contracts.Int(1, 1<<53)
		required = append(required, "revision")
	}
	switch a {
	case "create", "update":
		input["title"] = str(200)
		input["body"] = str(20000)
		required = append(required, "title", "body")
	case "record-review":
		input["code"] = &contracts.Schema{Type: "string", Enum: []string{"safe", "blocked", "suspected"}}
		required = append(required, "code")
	case "decide":
		input["approve"] = &contracts.Schema{Type: "boolean"}
		required = append(required, "approve")
	}
	permission := "articles:write"
	ro := a == "list" || a == "get"
	if ro {
		permission = "articles:read"
	}
	if a == "decide" {
		permission = "articles:review"
	}
	d := contracts.Definition{Name: "article." + a, Version: "1.0.0", Title: "Article " + a, Description: "Version checked article moderation and publishing.", Domain: "articles", Entity: "article", Layer: contracts.LayerAtomic, Permission: permission, Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto, ReadOnly: ro, Executor: "go:articles." + a, Input: contracts.Object(input, required...), Output: contracts.Object(map[string]*contracts.Schema{}), Loggable: []string{}}
	if ro {
		d.TransientResult = true
	}
	if !ro {
		d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
		d.SuccessEvents = []string{events[a]}
	}
	row := map[string]*contracts.Schema{"id": {Type: "integer"}, "title": {Type: "string"}, "status": {Type: "string"}, "revision": {Type: "integer"}, "reviewCode": {Type: "string"}}
	if a == "list" {
		d.Output = contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: contracts.Object(row, "id", "title", "status", "revision", "reviewCode")}}, "items")
	} else if a == "get" {
		row["body"] = &contracts.Schema{Type: "string"}
		d.Output = contracts.Object(row, "id", "title", "body", "status", "revision", "reviewCode")
	} else {
		d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	}
	if a == "record-review" {
		// The recorded verdict, not the model answer, is what a branch reads.
		d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "decision": {Type: "string", Enum: []string{"approve", "reject"}}}, "id", "decision")
	}
	if a == "publish" {
		// Publishing is public and irreversible, so an automated flow may only
		// propose it: the card waits for a human. ApprovalConfirm is the policy
		// for high-risk automation; switch to ApprovalFourEyes in deployments
		// that have a second approver and want separation of duties.
		d.Title = "Article publish (human approved)"
		d.Description = "Publish an approved article. High risk: a workflow stops for human confirmation before content becomes public."
		d.Risk = contracts.RiskHigh
		d.Approval = contracts.ApprovalConfirm
	}
	return d
}
func principal(ctx context.Context) (*identity.Principal, error) { return identity.Current(ctx) }
func integer(in contracts.Input, key string) int64 {
	switch v := in[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}
func (m *Module) list(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, e := principal(ctx)
	if e != nil {
		return contracts.Output{}, e
	}
	rows, e := dbgen.New(m.Pool).ArticleList(ctx, dbgen.ArticleListParams{TenantID: p.TenantID, CreatedBy: p.Account.ID, CanReview: p.Can("articles:review")})
	if e != nil {
		return contracts.Output{}, e
	}
	items := []map[string]any{}
	for _, r := range rows {
		items = append(items, map[string]any{"id": r.ID, "title": r.Title, "status": r.Status, "revision": r.Revision, "reviewCode": r.ReviewCode})
	}
	return contracts.Output{Data: map[string]any{"items": items}}, nil
}
func (m *Module) get(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, e := principal(ctx)
	if e != nil {
		return contracts.Output{}, e
	}
	r, e := dbgen.New(m.Pool).ArticleGet(ctx, dbgen.ArticleGetParams{TenantID: p.TenantID, ID: integer(in, "id"), CreatedBy: p.Account.ID, CanReview: p.Can("articles:review")})
	if e != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: map[string]any{"id": r.ID, "title": r.Title, "body": r.Body, "status": r.Status, "revision": r.Revision, "reviewCode": r.ReviewCode}}, nil
}
func (m *Module) write(ctx context.Context, tx pgx.Tx, in contracts.Input, a string) (contracts.Output, error) {
	p, e := principal(ctx)
	if e != nil {
		return contracts.Output{}, e
	}
	q := dbgen.New(tx)
	id := integer(in, "id")
	rev := integer(in, "revision")
	var n int64
	title, _ := in["title"].(string)
	body, _ := in["body"].(string)
	code, _ := in["code"].(string)
	if a == "create" || a == "update" {
		if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
			return contracts.Output{}, contracts.Problem("invalid")
		}
	}
	meta, _ := gateway.CurrentInvocation(ctx)
	// The verdict may only come from the pinned review flow, and publishing may
	// only come from that flow or from the approval that resumes it. A page,
	// tool or API call cannot claim that a review already happened, and a human
	// cannot hand-record a verdict to skip the review.
	if a == "record-review" && meta.Origin != contracts.OriginWorkflow {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if a == "publish" || a == "reject" {
		if meta.Origin != contracts.OriginWorkflow && meta.Origin != contracts.OriginApproval {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
	}
	switch a {
	case "create":
		id, e = q.ArticleCreate(ctx, dbgen.ArticleCreateParams{TenantID: p.TenantID, CreatedBy: p.Account.ID, Title: title, Body: body})
		n = 1
	case "update":
		n, e = q.ArticleUpdate(ctx, dbgen.ArticleUpdateParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, Title: title, Body: body, Revision: rev})
	case "submit":
		n, e = q.ArticleSubmit(ctx, dbgen.ArticleSubmitParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, Revision: rev})
	case "record-review":
		if !reviewCode(code) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		if e = m.checkPolicy(ctx, q, p, id, rev, code); e != nil {
			return contracts.Output{}, e
		}
		n, e = q.ArticleRecordReview(ctx, dbgen.ArticleRecordReviewParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, ReviewCode: code, Revision: rev})
	case "publish":
		n, e = q.ArticlePublish(ctx, dbgen.ArticlePublishParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, Revision: rev})
	case "reject":
		n, e = q.ArticleReject(ctx, dbgen.ArticleRejectParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, Revision: rev})
	case "decide":
		if meta.Origin != contracts.OriginHumanUI {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
		status := "rejected"
		if in["approve"] == true {
			status = "published"
		}
		n, e = q.ArticleDecide(ctx, dbgen.ArticleDecideParams{TenantID: p.TenantID, ID: id, Revision: rev, Status: status, ReviewedBy: p.Account.ID})
	}
	if e != nil {
		return contracts.Output{}, e
	}
	if n != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	data := map[string]any{"id": id}
	if a == "record-review" {
		// Only a persisted 'safe' verdict maps to approve; every other recorded
		// code is a rejection reason, so a flow never needs to inspect the model
		// answer to choose a branch.
		data["decision"] = "reject"
		if code == "safe" {
			data["decision"] = "approve"
		}
	}
	return contracts.Output{Data: data, Summary: fmt.Sprintf("article=%d", id), Events: []contracts.Event{{Type: events[a], AggregateType: "article", AggregateID: fmt.Sprint(id), Payload: map[string]any{"id": id}}}}, nil
}

// reviewCode is the closed set of AI verdicts. A model failure never widens it.
func reviewCode(code string) bool {
	return code == "safe" || code == "blocked" || code == "suspected"
}

// checkPolicy keeps a configured blocked-word rule authoritative over the model.
// A verdict that clears content the published policy matches is refused. An
// absent or empty word list means there is no deterministic rule to apply: the
// publish still needs another human, so it does not block the flow by itself.
func (m *Module) checkPolicy(ctx context.Context, q *dbgen.Queries, p *identity.Principal, id, revision int64, code string) error {
	if m.Blocked == nil {
		return contracts.Problem("unavailable")
	}
	record, err := q.ArticleGet(ctx, dbgen.ArticleGetParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, CanReview: false})
	if err != nil || record.Revision != revision {
		return contracts.Problem("conflict")
	}
	blocked, configured, err := m.Blocked(ctx, p, record.Title+"\n"+record.Body)
	if err != nil {
		return contracts.Problem("unavailable")
	}
	if configured && blocked && code != "blocked" {
		return contracts.Problem("conflict")
	}
	return nil
}

// Normalize catches case, punctuation, whitespace and zero-width obfuscation.
func Normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
func ContainsBlocked(content, words string) bool {
	content = Normalize(content)
	for _, word := range strings.FieldsFunc(words, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == '，' }) {
		word = Normalize(word)
		if word != "" && strings.Contains(content, word) {
			return true
		}
	}
	return false
}

// Package articles implements moderated publishing through the capability gateway.
package articles

import (
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"unicode"
)

type Module struct {
	Pool     *pgxpool.Pool
	Gateway  *gateway.Gateway
	Classify func(context.Context, string) (string, error)
}
type verdictKey struct{}

func (m *Module) Entries() []registry.Entry {
	entries := []registry.Entry{}
	for _, action := range []string{"list", "get", "create", "update", "submit", "assess", "apply", "decide"} {
		d := definition(action)
		h := contracts.Handler{}
		switch action {
		case "list":
			h.Execute = m.list
		case "get":
			h.Execute = m.get
		case "assess":
			h.Execute = m.assess
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
	if a == "update" || a == "submit" || a == "apply" || a == "decide" {
		input["revision"] = contracts.Int(1, 1<<53)
		required = append(required, "revision")
	}
	if a == "create" || a == "update" {
		input["title"] = str(200)
		input["body"] = str(20000)
		required = append(required, "title", "body")
	}
	if a == "decide" {
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
	}
	if a == "assess" {
		d.Layer = contracts.LayerOrchestration
		d.SuccessEvents = []string{"article.assessed"}
	} else if !ro {
		d.SuccessEvents = []string{"article." + a + "d"}
		if a == "submit" {
			d.SuccessEvents = []string{"article.submitted"}
		}
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
	if a == "create" || a == "update" {
		if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
			return contracts.Output{}, contracts.Problem("invalid")
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
	case "apply":
		code, ok := ctx.Value(verdictKey{}).(string)
		if !ok {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
		status := "pending_review"
		if code == "safe" {
			status = "published"
		}
		n, e = q.ArticleModerate(ctx, dbgen.ArticleModerateParams{TenantID: p.TenantID, ID: id, CreatedBy: p.Account.ID, Revision: rev, Status: status, ReviewCode: code})
	case "decide":
		meta, ok := gateway.CurrentInvocation(ctx)
		if !ok || meta.Origin != contracts.OriginHumanUI {
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
	event := "article." + a + "d"
	if a == "submit" {
		event = "article.submitted"
	}
	return contracts.Output{Data: map[string]any{"id": id}, Summary: fmt.Sprintf("article=%d", id), Events: []contracts.Event{{Type: event, AggregateType: "article", AggregateID: fmt.Sprint(id), Payload: map[string]any{"id": id}}}}, nil
}
func (m *Module) assess(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	if m.Gateway == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	r, e := m.Gateway.Invoke(ctx, gateway.Request{Name: "article.get", Input: in})
	if e != nil {
		return contracts.Output{}, e
	}
	if r.Status != contracts.StatusSucceeded {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	raw, _ := json.Marshal(r.Data)
	var record struct {
		ID       int64  `json:"id"`
		Revision int64  `json:"revision"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		Status   string `json:"status"`
	}
	if e = json.Unmarshal(raw, &record); e != nil {
		return contracts.Output{}, e
	}
	if record.Status != "pending_ai" {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	code := "unavailable"
	if m.Classify != nil {
		if v, err := m.Classify(ctx, record.Title+"\n"+record.Body); err == nil && (v == "safe" || v == "blocked" || v == "suspected") {
			code = v
		}
	}
	result, e := m.Gateway.Invoke(context.WithValue(ctx, verdictKey{}, code), gateway.Request{Name: "article.apply", Input: contracts.Input{"id": record.ID, "revision": record.Revision}, IdempotencyKey: gateway.ChildKey(ctx, "apply")})
	if e != nil {
		return contracts.Output{}, e
	}
	if result.Status != contracts.StatusSucceeded {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: result.Data, Events: []contracts.Event{{Type: "article.assessed", AggregateType: "article", AggregateID: fmt.Sprint(record.ID), Payload: map[string]any{"id": record.ID}}}}, nil
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

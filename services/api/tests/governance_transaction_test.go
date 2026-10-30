//go:build integration

package tests

import (
	dict "baseline/services/api/gen/dictionaries"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/dictionaries"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/internal/transport"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGovernanceTransactionsUseOneConnection(t *testing.T) {
	_, original, account := iamFixture(t)
	cfg := original.Config().Copy()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{StorageRoot: t.TempDir()})
	files.New(core)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := transport.HandlerWithCore(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, core, iam)
	root := loginIAM(t, handler, "root-admin")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request := httptest.NewRequest("POST", "http://console.test/api/admin/settings/drafts", strings.NewReader(fmt.Sprintf(`{"target":{"kind":"workspace","id":%d},"baseRevision":0,"reason":"Single connection inheritance","changes":[{"key":"display.theme","operation":"set","value":"\"light\""}]}`, 0))).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://console.test")
	request.Header.Set("X-CSRF-Token", root.csrf)
	request.AddCookie(root.cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	checkStatus(t, response, 201)
	doc := &dict.DictionaryDocument{Definition: &dict.DictionaryDefinition{Code: "transaction.options", NameEn: "Transactional options", Category: "tests", Kind: "flat", Status: "active", Visibility: "shared", Target: &dict.ConfigTarget{Kind: "workspace"}}, Items: []*dict.DictionaryItem{{Code: "one", LabelEn: "One", Status: "active"}}}
	w := root.request(t, "POST", "/api/admin/dictionaries/drafts", &dict.DraftPayload{Document: doc, Reason: "Create transaction reference options"}, true)
	checkStatus(t, w, 201)
	draft := decodeBody[dict.ChangeRecord](t, w)
	w = root.request(t, "POST", fmt.Sprintf("/api/admin/dictionaries/drafts/%d/publish", draft.ID), &dict.PublishPayload{}, true)
	checkStatus(t, w, 200)
	version := decodeBody[dict.ChangeRecord](t, w)
	principal, err := iam.Authenticate(context.Background(), root.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	referenceCtx, referenceCancel := context.WithTimeout(identity.WithPrincipal(context.Background(), principal), 3*time.Second)
	defer referenceCancel()
	tx, err := pool.Begin(referenceCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := dbgen.New(tx)
	if err = dictionaries.New(core).ReplaceReferences(referenceCtx, q, doc.Definition.Code, "account.preference", account.ID, []int64{version.Document.Items[0].ID}, nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(referenceCtx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM gov_dictionary_references").Scan(&count); err != nil || count != 1 {
		t.Fatal("reference was not committed", count, err)
	}
}

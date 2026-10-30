//go:build integration

package tests

import (
	"baseline/services/api/db/migrations"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/repository"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/internal/transport"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIsolatedPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL must point to an isolated local/CI PostgreSQL; integration tests are never silently skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = provider.Up(ctx); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err = provider.DownTo(ctx, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Up is idempotent, and the migration's Down/Up was actually exercised above.
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]string{{"demo-tenant", "org-a", "dept-a", "demo-user", "own"}, {"demo-tenant", "org-b", "dept-b", "other-user", "other-owner"}, {"other-tenant", "org-a", "dept-a", "demo-user", "other-tenant"}} {
		if _, err = pool.Exec(ctx, "INSERT INTO technical_demo_records (tenant_id,organization_id,department_id,owner_id,label,internal_note) VALUES ($1,$2,$3,$4,$5,'secret')", values[0], values[1], values[2], values[3], values[4]); err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.Records{Queries: dbgen.New(pool)}
	own := dataaccess.Scope{Kind: dataaccess.Own, TenantID: "demo-tenant", UserID: "demo-user"}
	rows, err := repo.List(ctx, own)
	if err != nil || len(rows) != 1 || rows[0].Label != "own" {
		t.Fatalf("scoped list: %v %v", rows, err)
	}
	id := rows[0].ID
	t.Run("all supported scopes remain tenant constrained", func(t *testing.T) {
		for _, tc := range []struct {
			s     dataaccess.Scope
			count int
		}{{dataaccess.Scope{Kind: dataaccess.All, TenantID: own.TenantID, UserID: own.UserID}, 2}, {dataaccess.Scope{Kind: dataaccess.Organization, TenantID: own.TenantID, UserID: own.UserID, OrganizationIDs: []string{"org-a"}}, 1}, {dataaccess.Scope{Kind: dataaccess.Department, TenantID: own.TenantID, UserID: own.UserID, DepartmentIDs: []string{"dept-b"}}, 1}, {dataaccess.Scope{Kind: dataaccess.Custom, TenantID: own.TenantID, UserID: own.UserID, ResourceIDs: []int64{1, 2, 3}}, 2}} {
			got, err := repo.List(ctx, tc.s)
			if err != nil || len(got) != tc.count {
				t.Fatalf("%s: %v %v", tc.s.Kind, got, err)
			}
		}
	})
	t.Run("details updates deletes enforce owner and tenant atomically", func(t *testing.T) {
		for _, outside := range []int64{2, 3} {
			if _, err := repo.Get(ctx, own, outside); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal("unscoped read", err)
			}
			if _, err := repo.Update(ctx, own, outside, "forbidden"); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal("unscoped update", err)
			}
			if err := repo.Delete(ctx, own, outside); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal("unscoped delete", err)
			}
		}
		if _, err = repo.Update(ctx, own, id, "updated"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("transaction rollback", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		transactionRepo := repository.Records{Queries: dbgen.New(tx)}
		if _, err = transactionRepo.Update(ctx, own, id, "rolled-back"); err != nil {
			t.Fatal(err)
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		row, err := repo.Get(ctx, own, id)
		if err != nil || row.Label != "updated" {
			t.Fatal("rollback failed", err)
		}
	})
	t.Run("concurrent atomic updates", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := repo.Update(ctx, own, id, "concurrent"); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	})
	t.Run("database constraint", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "INSERT INTO technical_demo_records (tenant_id,organization_id,department_id,owner_id,label) VALUES ('t','o','d','u','')"); err == nil {
			t.Fatal("empty label accepted")
		}
	})
	t.Run("HTTP authorization and response projection", func(t *testing.T) {
		e, err := authorization.New()
		if err != nil {
			t.Fatal(err)
		}
		h := transport.Handler(app.System{}, app.Demo{Store: repo, Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{Token: "test"}, e)
		for _, tc := range []struct {
			method, path, body string
			status             int
		}{{"GET", "/api/demo/records", "", 200}, {"GET", "/api/demo/records/3", "", 404}, {"PATCH", "/api/demo/records/3", `{"label":"forbidden"}`, 404}, {"DELETE", "/api/demo/records/3", "", 404}, {"GET", "/api/demo/records?tenant_id=other-tenant", "", 200}} {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer test")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Tenant-ID", "other-tenant")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
			}
			for _, secret := range []string{"secret", "internal_note", "other-tenant", "other-owner"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("sensitive/unscoped data leaked", w.Body.String())
				}
			}
		}
		if _, err = e.Revoke(auth.Principal{UserID: "demo-user", TenantID: "demo-tenant"}); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/api/demo/records", nil)
		r.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("revocation failed")
		}
	})
	if err = repo.Delete(ctx, own, id); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Get(ctx, own, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("delete failed")
	}
}

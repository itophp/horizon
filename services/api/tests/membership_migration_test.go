//go:build integration

package tests

import (
	"baseline/services/api/db/migrations"
	genidentity "baseline/services/api/gen/identity"
	"context"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"testing"
)

func TestMembershipMigrationPreservesExistingIdentityAndRequests(t *testing.T) {
	handler, pool, admin := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	// Only this test's disposable schema is rolled back, to simulate the already deployed schema.
	if _, err = provider.DownTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	var hashBefore string
	if err = pool.QueryRow(ctx, "SELECT password_hash FROM iam_accounts WHERE id=$1", admin.ID).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	var requestID int64
	if err = pool.QueryRow(ctx, "INSERT INTO iam_temporary_requests(tenant_id,applicant_id,requested_grants,requested_minutes,reason) VALUES('tenant-a',$1,'[]',60,'Existing pending request') RETURNING id", admin.ID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var dept int64
	if err = pool.QueryRow(ctx, "SELECT department_id FROM iam_temporary_requests WHERE id=$1", requestID).Scan(&dept); err != nil || dept != admin.DepartmentID {
		t.Fatal("request context backfill failed", err, dept)
	}
	var hashAfter string
	if err = pool.QueryRow(ctx, "SELECT password_hash FROM iam_accounts WHERE id=$1", admin.ID).Scan(&hashAfter); err != nil || hashBefore != hashAfter {
		t.Fatal("migration altered credential", err)
	}
	response := root.request(t, "GET", "/api/auth/me", nil, false)
	checkStatus(t, response, 200)
	profile := decodeBody[genidentity.SessionProfile](t, response)
	if profile.ActiveDepartmentID != admin.DepartmentID || len(profile.Account.Memberships) != 1 || len(profile.Account.RoleIds) != 1 {
		t.Fatal("legacy session or primary role lost")
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
}

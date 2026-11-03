//go:build integration

package tests

import (
	"baseline/services/api/db/migrations"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"testing"
)

func TestGlobalGovernanceMigrationPreservesDictionaryReferences(t *testing.T) {
	_, pool, account := iamFixture(t)
	ctx := context.Background()
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 6); err != nil {
		t.Fatal(err)
	}
	scope := fmt.Sprintf("department:%d", account.DepartmentID)
	if _, err = pool.Exec(ctx, "INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id,department_id) VALUES('tenant-a',$1,'department',$2,$2)", scope, account.DepartmentID); err != nil {
		t.Fatal(err)
	}
	var dictionaryID, itemID int64
	if err = pool.QueryRow(ctx, `INSERT INTO gov_dictionaries(tenant_id,scope_key,code,spec) VALUES('tenant-a',$1,'legacy.shared','{"Target":{"Kind":"department","ID":1},"AllowExtensions":true}') RETURNING id`, scope).Scan(&dictionaryID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO gov_dictionary_items(tenant_id,dictionary_id,scope_key,code,spec) VALUES('tenant-a',$1,$2,'stable','{"Owner":{"Kind":"department","ID":1},"Status":"active","IsDefault":true}') RETURNING id`, dictionaryID, scope).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO gov_dictionary_references(tenant_id,dictionary_id,item_id,resource_type,resource_id) VALUES('tenant-a',$1,$2,'test','original')`, dictionaryID, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO gov_values(tenant_id,scope_key,config_key,value_json) VALUES('tenant-a',$1,'display.theme','"light"')`, scope); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var code, owner, target string
	var extensions bool
	if err = pool.QueryRow(ctx, `SELECT i.code,i.scope_key,d.scope_key,(d.spec->>'AllowExtensions')::boolean FROM gov_dictionary_items i JOIN gov_dictionaries d ON d.id=i.dictionary_id JOIN gov_dictionary_references r ON r.item_id=i.id WHERE i.id=$1 AND r.resource_id='original'`, itemID).Scan(&code, &owner, &target, &extensions); err != nil || code != "stable" || owner != "workspace:0" || target != "workspace:0" || extensions {
		t.Fatal("dictionary identity/reference was not preserved", code, owner, target, extensions, err)
	}
	var settingsScope string
	if err = pool.QueryRow(ctx, `SELECT scope_key FROM gov_values WHERE config_key='display.theme'`).Scan(&settingsScope); err != nil || settingsScope != scope {
		t.Fatal("legacy settings relocated", settingsScope, err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
}

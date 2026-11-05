//go:build integration

package tests

import (
	"baseline/services/api/applications/articles"
	gen "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestApplicationRuntimeArticleQueries(t *testing.T) {
	handler, pool, root := iamFixture(t)
	client := loginIAM(t, handler, "root-admin")
	response := invokeCapability(t, client, "application.runtime.resolve", map[string]any{"input": map[string]any{"applicationId": "articles"}})
	if response.Status != "succeeded" {
		t.Fatal(response)
	}
	raw, _ := json.Marshal(response.Data)
	if string(raw) == "null" {
		t.Fatal("missing metadata")
	}
	ctx := context.Background()
	// More than the former 100-row cutoff, plus records outside both scope boundaries.
	_, err := pool.Exec(ctx, `INSERT INTO articles(tenant_id,created_by,title,body) SELECT 'tenant-a',$1,'Page '||lpad(i::text,3,'0'),'private' FROM generate_series(1,105)i`, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO articles(tenant_id,created_by,title,body) VALUES('tenant-b',$1,'Page foreign','secret'),('tenant-a',$2,'Page other','secret')`, root.ID, root.ID+100000)
	if err != nil {
		t.Fatal(err)
	}
	p, err := identity.NewPrincipal(&gen.AccountRecord{ID: root.ID}, "tenant-a", []*gen.PermissionGrant{{Permission: "articles:read", Effect: "allow"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)
	g := gateway.New(gateway.Options{Registry: registry.New((&articles.Module{Pool: pool}).Entries()), Pool: pool})
	result, err := g.Invoke(ctx, gateway.Request{Name: "article.list", Input: contracts.Input{"page": 6, "pageSize": 20, "sort": "title:asc", "keyword": "Page"}, Origin: contracts.OriginHumanUI})
	if err != nil || result.Status != contracts.StatusSucceeded {
		t.Fatal(result, err)
	}
	data := result.Data.(map[string]any)
	if fmt.Sprint(data["total"]) != "105" {
		t.Fatal(data)
	}
	items := data["items"].([]map[string]any)
	if len(items) != 5 || items[0]["title"] != "Page 101" {
		t.Fatal(items)
	}
	for _, item := range items {
		if _, ok := item["body"]; ok {
			t.Fatal("body exposed")
		}
	}
	for _, input := range []contracts.Input{{"pageSize": 101}, {"sort": "body:asc"}, {"select": []any{"body"}}, {"tenantId": "tenant-b"}} {
		result, err = g.Invoke(ctx, gateway.Request{Name: "article.list", Input: input, Origin: contracts.OriginHumanUI})
		if err == nil && result.Status == contracts.StatusSucceeded {
			t.Fatal("unsafe query accepted", input)
		}
	}
}

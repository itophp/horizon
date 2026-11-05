package gateway

import (
	gen "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"
	"testing"
)

func TestFieldPoliciesRejectWritesAndProjectNestedOutputWithoutMutation(t *testing.T) {
	principal, err := identity.NewPrincipal(&gen.AccountRecord{ID: 9}, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	d := contracts.Definition{InputAccess: []contracts.FieldAccess{{Path: "items.secret", Permission: "articles:review"}}, OutputAccess: []contracts.FieldAccess{{Path: "items.secret", Permission: "articles:review"}}}
	input := contracts.Input{"items": []any{map[string]any{"name": "allowed", "secret": "private"}}}
	if err = checkInputAccess(principal, d, input); err == nil {
		t.Fatal("unauthorized nested write accepted")
	}
	projected, err := projectOutput(principal, d, map[string]any(input))
	if err != nil {
		t.Fatal(err)
	}
	item := projected.(map[string]any)["items"].([]any)[0].(map[string]any)
	if _, ok := item["secret"]; ok {
		t.Fatal("secret exposed")
	}
	if !hasPath(input, []string{"items", "secret"}) {
		t.Fatal("original result mutated")
	}
	if _, err = projectOutput(principal, d, func() {}); err == nil {
		t.Fatal("invalid output silently accepted")
	}
}

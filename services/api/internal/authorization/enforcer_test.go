package authorization

import (
	"baseline/services/api/internal/auth"
	"testing"
)

func TestTenantRBACAndRevocation(t *testing.T) {
	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{UserID: "demo-user", TenantID: "demo-tenant"}
	for _, tc := range []struct {
		p      auth.Principal
		action string
		want   bool
	}{{p, "read", true}, {p, "update", true}, {p, "delete", true}, {p, "approve", false}, {auth.Principal{UserID: "demo-user", TenantID: "other-tenant"}, "read", false}, {auth.Principal{UserID: "unknown", TenantID: "demo-tenant"}, "read", false}, {auth.Principal{}, "read", false}} {
		allowed, err := e.Allow(tc.p, "demo-record", tc.action)
		if err != nil || allowed != tc.want {
			t.Fatalf("decision: got %v, %v; want %v", allowed, err, tc.want)
		}
	}
	if _, err = e.Revoke(p); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.Allow(p, "demo-record", "read"); err != nil || ok {
		t.Fatal("revocation did not take effect", err)
	}
}

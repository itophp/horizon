package auth

import (
	"context"
	"testing"
)

func TestTokenOptIn(t *testing.T) {
	for _, tc := range []struct {
		configured, provided string
		want                 bool
	}{{"", "", false}, {"token", "bad", false}, {"token", "token", true}} {
		p, err := (DevelopmentToken{Token: tc.configured}).Authenticate(context.Background(), tc.provided)
		if (err == nil) != tc.want {
			t.Fatal("unexpected authentication decision")
		}
		if tc.want && (p.TenantID != "demo-tenant" || p.UserID != "demo-user") {
			t.Fatal("identity mismatch")
		}
	}
}

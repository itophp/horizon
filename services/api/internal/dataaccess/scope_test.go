package dataaccess

import (
	"baseline/services/api/internal/auth"
	"testing"
)

func TestFailClosed(t *testing.T) {
	if _, err := (OwnResolver{}).Resolve(auth.Principal{}); err == nil {
		t.Fatal("empty identity accepted")
	}
	if err := (Scope{Kind: Custom, TenantID: "a", UserID: "u"}).Validate(); err == nil {
		t.Fatal("empty custom scope accepted")
	}
	if err := (Scope{Kind: Kind("unknown"), TenantID: "a", UserID: "u"}).Validate(); err == nil {
		t.Fatal("unknown scope accepted")
	}
}

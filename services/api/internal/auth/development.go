package auth

import (
	"context"
	"crypto/subtle"
	"errors"
)

// DevelopmentToken is opt-in and only accepted on a loopback listener. Replace with OIDC for deployment.
type DevelopmentToken struct{ Token string }

func (a DevelopmentToken) Authenticate(_ context.Context, token string) (Principal, error) {
	if a.Token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.Token)) != 1 {
		return Principal{}, errors.New("invalid credential")
	}
	return Principal{UserID: "demo-user", TenantID: "demo-tenant"}, nil
}

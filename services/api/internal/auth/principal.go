package auth

import "context"

type Principal struct{ UserID, TenantID string }
type contextKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok && p.UserID != "" && p.TenantID != ""
}

// Authenticator must resolve identity from a verified credential, never request tenant/owner headers.
type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}

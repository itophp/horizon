package identity

import "context"

// PrincipalForSessionInvocation rebuilds formal and unexpired session-scoped
// temporary authority from storage. Temporary grants remain separate from the
// formal grants used for publishing, approval and persistent delegation.
func (s *Service) PrincipalForSessionInvocation(ctx context.Context, tenant string, id, department int64, tokenHash string) (*Principal, error) {
	session, err := s.repo.q.IAMSession(ctx, tokenHash)
	if err != nil || session.TenantID != tenant || session.AccountID != id || session.ActiveDepartmentID != department {
		return nil, problem("unauthenticated")
	}
	principal, err := s.PrincipalForTask(ctx, tenant, id, department)
	if err != nil {
		return nil, err
	}
	if err = s.loadTemporary(ctx, principal); err != nil {
		return nil, err
	}
	principal.CSRF = session.CsrfToken
	principal.TokenHash = tokenHash
	return principal, nil
}

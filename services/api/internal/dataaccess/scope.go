package dataaccess

import (
	"baseline/services/api/internal/auth"
	"errors"
)

type Kind string

const (
	All          Kind = "ALL"
	Organization Kind = "ORGANIZATION"
	Department   Kind = "DEPARTMENT"
	Own          Kind = "OWN"
	Custom       Kind = "CUSTOM"
)

type Scope struct {
	Kind                           Kind
	TenantID, UserID               string
	OrganizationIDs, DepartmentIDs []string
	ResourceIDs                    []int64
}

// Resolver is the extension point for trusted, server-side scope policy. The demo grants OWN only.
type Resolver interface {
	Resolve(auth.Principal) (Scope, error)
}
type OwnResolver struct{}

func (OwnResolver) Resolve(p auth.Principal) (Scope, error) {
	s := Scope{Kind: Own, TenantID: p.TenantID, UserID: p.UserID}
	return s, s.Validate()
}
func (s Scope) Validate() error {
	if s.TenantID == "" || s.UserID == "" {
		return errors.New("missing trusted identity")
	}
	switch s.Kind {
	case All, Own:
		return nil
	case Organization:
		if len(s.OrganizationIDs) > 0 {
			return nil
		}
	case Department:
		if len(s.DepartmentIDs) > 0 {
			return nil
		}
	case Custom:
		if len(s.ResourceIDs) > 0 {
			return nil
		}
	}
	return errors.New("invalid or empty data scope")
}

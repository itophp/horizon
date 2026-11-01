package identity

import genidentity "baseline/services/api/gen/identity"

// DepartmentAccess replaces the additional memberships of an account. The
// membership record itself stays the snapshot projection type.
type DepartmentAccess struct {
	Memberships []*genidentity.DepartmentMembership
}

// AccountInput is the reviewed payload of account creation and update. It is
// owned by the identity domain: the capability contract is the only ingress, so
// the domain must not depend on generated transport types.
type AccountInput struct {
	ID               int64
	Username         string
	FullName         string
	Email            string
	Status           string
	OrganizationID   int64
	DepartmentID     int64
	RoleIds          []int64
	Overrides        []*genidentity.PermissionGrant
	Password         string
	DepartmentAccess *DepartmentAccess
}

// OrganizationInput is the reviewed payload of organization creation and update.
type OrganizationInput struct {
	ID          int64
	Name        string
	Code        string
	Kind        string
	ParentID    int64
	Status      string
	Description string
}

// DepartmentInput is the reviewed payload of department creation and update.
type DepartmentInput struct {
	ID             int64
	Name           string
	Code           string
	OrganizationID int64
	ParentID       int64
	Manager        string
	Status         string
	Description    string
}

// RoleInput is the reviewed payload of role creation and update.
type RoleInput struct {
	ID          int64
	Name        string
	Code        string
	Description string
	Status      string
	Grants      []*genidentity.PermissionGrant
}

// DeleteInput addresses one structure or account by primary key.
type DeleteInput struct{ ID int64 }

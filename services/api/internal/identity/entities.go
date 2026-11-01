package identity

import "baseline/services/api/internal/platform/capability/contracts"

// Entities lists the identity objects that have a model.
func Entities() []contracts.Entity {
	return []contracts.Entity{
		accountEntity(),
		sessionEntity(),
		organizationEntity(),
		departmentEntity(),
		roleEntity(),
	}
}

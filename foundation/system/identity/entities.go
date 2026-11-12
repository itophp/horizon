package identity

import "horizon/foundation/kernel/capability/contracts"

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

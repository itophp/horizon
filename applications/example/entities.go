package example

import "horizon/foundation/kernel/capability/contracts"

// Entities lists the reference business object of this module.
func Entities() []contracts.Entity {
	return []contracts.Entity{exampleRecordEntity()}
}

package files

import "horizon/foundation/kernel/capability/contracts"

// Entities lists the managed file object.
func Entities() []contracts.Entity {
	return []contracts.Entity{fileEntity()}
}

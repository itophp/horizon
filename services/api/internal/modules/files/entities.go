package files

import "baseline/services/api/internal/platform/capability/contracts"

// Entities lists the managed file object.
func Entities() []contracts.Entity {
	return []contracts.Entity{fileEntity()}
}

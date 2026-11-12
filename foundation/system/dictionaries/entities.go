package dictionaries

import "horizon/foundation/kernel/capability/contracts"

// Entities lists the dictionary objects. Publishing is modelled on the draft
// entity, so the version table is not declared twice.
func Entities() []contracts.Entity {
	return []contracts.Entity{dictionaryEntity(), dictionaryItemEntity()}
}

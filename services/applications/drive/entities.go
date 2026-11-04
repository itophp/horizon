package drive

import "baseline/services/api/kernel/capability/contracts"

// Entities describes cloud drive tables for the schema checker.
func Entities() []contracts.Entity {
	return []contracts.Entity{spaceEntity(), memberEntity(), folderEntity(), trashBatchEntity(), fileEntity(), uploadEntity(), uploadChunkEntity()}
}

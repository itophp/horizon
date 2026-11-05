// Package memory owns the Memory Center's business entities and capabilities.
package memory

import "baseline/services/api/kernel/capability/contracts"

// Entities provides the same entity declarations to runtime and offline tools.
func Entities() []contracts.Entity {
	return []contracts.Entity{memoryEventEntity(), memoryInboxEntity(), memoryItemEntity(), memorySourceEntity(), memoryRelationEntity(), memoryCorrectionEntity(), memoryAccessAuditEntity(), memoryPartitionPolicyEntity()}
}

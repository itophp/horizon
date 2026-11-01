package mcp

import "baseline/services/api/internal/platform/capability/contracts"

// Entities lists the remote MCP objects. The protocol itself is not an entity.
func Entities() []contracts.Entity {
	return []contracts.Entity{tokenEntity(), callAuditEntity()}
}

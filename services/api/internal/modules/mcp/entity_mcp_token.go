package mcp

import "baseline/services/api/internal/platform/capability/contracts"

// tokenEntity mirrors mcp_tokens. The secret is shown once and only its hash is
// stored, so both the hash and the prefix stay sensitive.
func tokenEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "mcp_token",
		Domain:      "mcp",
		Table:       "mcp_tokens",
		Description: "An account scoped credential for the remote MCP endpoint.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "accountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "tokenHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "SHA-256 of the secret; the secret itself is never stored."},
			{Name: "tokenPrefix", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "Short prefix so a human can tell tokens apart."},
			{Name: "expiresAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "revokedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "lastUsedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "revokedBy", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "account", Target: "account", Kind: "one", Via: "account_id", Required: true},
		},
	}
}

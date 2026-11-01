package mcp

import "baseline/services/api/internal/platform/capability/contracts"

// callAuditEntity mirrors mcp_call_audits: one protocol level tool call.
func callAuditEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "mcp_audit",
		Domain:      "mcp",
		Table:       "mcp_call_audits",
		Description: "One audited tool call made through a remote MCP token.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "accountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "tokenId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "tool", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "resultCode", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "durationMs", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "requestId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "token", Target: "mcp_token", Kind: "one", Via: "token_id", Required: true},
			{Name: "account", Target: "account", Kind: "one", Via: "account_id", Required: true},
		},
	}
}

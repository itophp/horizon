package mcp

import (
	"context"
	"fmt"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types published by the mcp module.
const (
	TokenCreatedEvent = "mcp.token.created"
	TokenRevokedEvent = "mcp.token.revoked"
	TokenRotatedEvent = "mcp.token.rotated"
)

const version = "1.0.0"

// Entries is the capability catalog contributed by the mcp module. The
// executors only delegate: password verification, department binding, the
// active token quota and the transaction stay in the service methods.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: s.createTokenDefinition(), Handler: contracts.Handler{Execute: s.createToken}},
		{Definition: s.listTokensDefinition(), Handler: contracts.Handler{Execute: s.listTokens}},
		{Definition: s.revokeTokenDefinition(), Handler: contracts.Handler{Execute: s.revokeToken}},
		{Definition: s.rotateTokenDefinition(), Handler: contracts.Handler{Execute: s.rotateToken}},
		{Definition: s.adminRevokeTokenDefinition(), Handler: contracts.Handler{Execute: s.adminRevokeToken}},
		{Definition: s.capabilitiesDefinition(), Handler: contracts.Handler{Execute: s.listCapabilities}},
		{Definition: s.auditDefinition(), Handler: contracts.Handler{Execute: s.listAudits}},
	}
}

func idSchema() *contracts.Schema { return contracts.Int(1, 1<<40) }

// passwordSchema is a reauthentication secret. It is accepted as input but is
// never listed in Loggable, so it cannot reach the audit projection.
func passwordSchema() *contracts.Schema { return contracts.Str(1, 128) }

// stringListSchema is an open ended list of tool names, as stored on a token.
func stringListSchema() *contracts.Schema {
	return &contracts.Schema{Type: "array", Items: &contracts.Schema{Type: "string"}}
}

func tokenSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id":           {Type: "integer"},
		"name":         {Type: "string"},
		"prefix":       {Type: "string"},
		"departmentId": {Type: "integer"},
		"tools":        stringListSchema(),
		"createdAt":    {Type: "string"},
		"expiresAt":    {Type: "string"},
		"revokedAt":    {Type: "string"},
		"lastUsedAt":   {Type: "string"},
	}, "id", "name", "prefix")
}

func auditSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id":           {Type: "integer"},
		"tokenId":      {Type: "integer"},
		"departmentId": {Type: "integer"},
		"tool":         {Type: "string"},
		"resultCode":   {Type: "string"},
		"durationMs":   {Type: "integer"},
		"createdAt":    {Type: "string"},
	}, "id", "tokenId", "tool", "resultCode", "createdAt")
}

func (s *Service) createTokenDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.token.create", Version: version, Title: "Create an MCP token",
		Description: "Mint a personal MCP credential bound to a department and a tool subset. The secret is returned once.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{TokenCreatedEvent}, Executor: "go:mcp.token.create",
		Tags: []string{"mcp", "write", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"password":     passwordSchema(),
			"name":         contracts.Str(1, 100),
			"departmentId": contracts.Int(0, 1<<40),
			"days":         contracts.Int(1, 90),
			"tools":        contracts.Array(contracts.Enum(Tools...), 1, len(Tools)),
		}, "password", "name", "days", "tools"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "prefix": {Type: "string"},
			"secret": {Type: "string"}, "expiresAt": {Type: "string"},
		}, "id", "name", "prefix", "secret"),
		// The password is deliberately absent: it must never be projected.
		Loggable: []string{"name", "departmentId", "days", "tools"},
	}
}

func (s *Service) listTokensDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.token.list", Version: version, Title: "List my MCP tokens",
		Description: "List the caller's own MCP credentials with their tools, expiry and revocation state.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:mcp.token.list", Tags: []string{"mcp", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"tokens": {Type: "array", Items: tokenSchema()},
		}),
	}
}

func (s *Service) revokeTokenDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.token.revoke", Version: version, Title: "Revoke an MCP token",
		Description: "Revoke one of the caller's own MCP credentials. The password is verified by the service.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "mcp:revoke",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{TokenRevokedEvent}, Executor: "go:mcp.token.revoke",
		Tags: []string{"mcp", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":       idSchema(),
			"password": passwordSchema(),
		}, "id", "password"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}),
		Loggable: []string{"id"},
	}
}

func (s *Service) rotateTokenDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.token.rotate", Version: version, Title: "Rotate an MCP token",
		Description: "Replace a credential with a new secret, carrying over its department, tools and expiry.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "mcp:revoke",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{TokenRotatedEvent}, Executor: "go:mcp.token.rotate",
		Tags: []string{"mcp", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":       idSchema(),
			"password": passwordSchema(),
		}, "id", "password"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"id": {Type: "integer"}, "name": {Type: "string"}, "prefix": {Type: "string"},
			"secret": {Type: "string"}, "expiresAt": {Type: "string"},
		}, "id", "name", "prefix", "secret"),
		Loggable: []string{"id"},
	}
}

func (s *Service) adminRevokeTokenDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.token.admin_revoke", Version: version, Title: "Revoke a tenant MCP token",
		Description: "Revoke any credential of the tenant. Requires tenant wide mcp:revoke and the caller's password.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "mcp:revoke",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{TokenRevokedEvent}, Executor: "go:mcp.token.admin_revoke",
		Tags: []string{"mcp", "write", "admin"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":       idSchema(),
			"password": passwordSchema(),
		}, "id", "password"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}),
		Loggable: []string{"id"},
	}
}

func (s *Service) capabilitiesDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.capability.list", Version: version, Title: "List MCP tool capabilities",
		Description: "List the MCP tools the caller may bind to a credential in a department.",
		Domain:      "mcp", Entity: "mcp_token", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:mcp.capability.list", Tags: []string{"mcp", "read", "agent"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"departmentId": contracts.Int(1, 1<<40),
		}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"tools": stringListSchema(),
		}),
		Loggable: []string{"departmentId"},
	}
}

func (s *Service) auditDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "mcp.audit.list", Version: version, Title: "List MCP tool audits",
		Description: "List the MCP tool calls of the tenant with their result code and duration.",
		Domain:      "mcp", Entity: "mcp_audit", Permission: "mcp:revoke",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:mcp.audit.list", Tags: []string{"mcp", "read"},
		Input: contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{
			"audits": {Type: "array", Items: auditSchema()},
		}),
	}
}

func (s *Service) createToken(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := CreatePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Create(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("token=%d", out.Token.ID),
		Events: []contracts.Event{{
			Type: TokenCreatedEvent, AggregateType: "mcp_token", AggregateID: fmt.Sprintf("%d", out.Token.ID),
			Payload: map[string]any{"tokenId": out.Token.ID, "name": out.Token.Name},
		}},
	}, nil
}

func (s *Service) listTokens(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ListPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.List(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("tokens=%d", len(out)),
	}, nil
}

func (s *Service) revokeToken(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RevokePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.Revoke(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": payload.ID},
		Summary: fmt.Sprintf("token=%d", payload.ID),
		Events: []contracts.Event{{
			Type: TokenRevokedEvent, AggregateType: "mcp_token", AggregateID: fmt.Sprintf("%d", payload.ID),
			Payload: map[string]any{"tokenId": payload.ID},
		}},
	}, nil
}

func (s *Service) rotateToken(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RotatePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Rotate(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("token=%d rotated=%d", out.Token.ID, payload.ID),
		Events: []contracts.Event{{
			Type: TokenRotatedEvent, AggregateType: "mcp_token", AggregateID: fmt.Sprintf("%d", out.Token.ID),
			Payload: map[string]any{"tokenId": out.Token.ID, "name": out.Token.Name, "rotatedFrom": payload.ID},
		}},
	}, nil
}

func (s *Service) adminRevokeToken(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AdminRevokePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.AdminRevoke(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": payload.ID},
		Summary: fmt.Sprintf("token=%d", payload.ID),
		Events: []contracts.Event{{
			Type: TokenRevokedEvent, AggregateType: "mcp_token", AggregateID: fmt.Sprintf("%d", payload.ID),
			Payload: map[string]any{"tokenId": payload.ID, "admin": true},
		}},
	}, nil
}

func (s *Service) listCapabilities(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := CapabilitiesPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Capabilities(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("tools=%d", len(out)),
	}, nil
}

func (s *Service) listAudits(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := AuditPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Audit(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    out,
		Summary: fmt.Sprintf("audits=%d", len(out)),
	}, nil
}

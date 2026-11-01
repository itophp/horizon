package mcp

import (
	"baseline/services/api/internal/repository/dbgen"
	"context"
)

// Purge removes expired history across workspaces as a trusted server retention
// job. No caller-selected scope, record identifiers or response data are exposed.
func (s *Service) Purge(ctx context.Context) error {
	if s.pool == nil {
		return nil
	}
	q := dbgen.New(s.pool)
	if err := q.MCPPruneAuditBatch(ctx); err != nil {
		return err
	}
	return q.MCPPruneTokenBatch(ctx)
}

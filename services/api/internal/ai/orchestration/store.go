package orchestration

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type executionKey struct{}
type execution struct {
	ID         int64
	Tenant     string
	Account    int64
	Manifest   manifest
	AgentTools bool
}
type manifest struct {
	Format      int            `json:"format"`
	Definition  Definition     `json:"definition"`
	Hash        string         `json:"hash"`
	Input       map[string]any `json:"input"`
	Correlation string         `json:"correlation"`
	Parent      string         `json:"parent"`
	Depth       int            `json:"depth"`
}
type checkpointStore struct {
	core   *governance.Core
	q      *dbgen.Queries
	prefix string
}

func current(ctx context.Context) (execution, error) {
	e, ok := ctx.Value(executionKey{}).(execution)
	if !ok {
		return e, contracts.Problem("invalid")
	}
	return e, nil
}
func (s checkpointStore) Set(ctx context.Context, key string, data []byte) error {
	e, err := current(ctx)
	if err != nil {
		return err
	}
	if len(data) > 4<<20 {
		return contracts.Problem("invalid")
	}
	key = s.prefix + key
	encrypted, err := s.core.Seal(e.Tenant, fmt.Sprint(e.ID), key, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return err
	}
	return s.q.OrchPutCheckpoint(ctx, dbgen.OrchPutCheckpointParams{TenantID: e.Tenant, AccountID: e.Account, ID: e.ID, CheckpointKey: key, Payload: encrypted})
}
func (s checkpointStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	e, err := current(ctx)
	if err != nil {
		return nil, false, err
	}
	key = s.prefix + key
	raw, err := s.q.OrchGetCheckpoint(ctx, dbgen.OrchGetCheckpointParams{TenantID: e.Tenant, AccountID: e.Account, RunID: e.ID, CheckpointKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	plain, err := s.core.Unseal(e.Tenant, fmt.Sprint(e.ID), key, raw)
	if err != nil {
		return nil, false, err
	}
	data, err := base64.StdEncoding.DecodeString(plain)
	return data, err == nil, err
}

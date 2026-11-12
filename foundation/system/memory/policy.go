package memory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func (m *Module) setPolicy(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:manage") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	partition, _ := in["partition"].(string)
	days := valueInt(in["retentionDays"])
	revision, err := dbgen.New(tx).MemorySetPolicy(ctx, dbgen.MemorySetPolicyParams{TenantID: p.TenantID, Partition: partition, RetentionDays: int32(days), UpdatedBy: p.Account.ID, ExpectedRevision: valueInt(in["expectedRevision"])})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"partition": partition, "retentionDays": days, "revision": revision}, Events: []contracts.Event{{Type: "memory.policy.updated", AggregateType: "memory_partition_policy", AggregateID: partition, Payload: map[string]any{"partition": partition, "revision": revision}}}}, nil
}

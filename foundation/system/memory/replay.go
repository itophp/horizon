package memory

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func (m *Module) replay(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:replay") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	eventID, _ := in["eventId"].(string)
	count, err := dbgen.New(tx).MemoryReplayEvent(ctx, dbgen.MemoryReplayEventParams{TenantID: p.TenantID, EventID: eventID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: map[string]any{"queued": true}, Events: []contracts.Event{{Type: "memory.ingestion.requeued", AggregateType: "memory_event_inbox", AggregateID: eventID, Payload: map[string]any{}}}}, nil
}

func (m *Module) replayRange(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:replay") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	first, last, limit := valueInt(in["firstSourceId"]), valueInt(in["lastSourceId"]), valueInt(in["limit"])
	if first < 1 || last < first || limit < 1 || limit > 100 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	ids, err := dbgen.New(tx).MemoryReplayRange(ctx, dbgen.MemoryReplayRangeParams{TenantID: p.TenantID, FirstID: first, LastID: last, MaxRows: int32(limit)})
	if err != nil {
		return contracts.Output{}, err
	}
	var lastQueued int64
	for _, id := range ids {
		if id > lastQueued {
			lastQueued = id
		}
	}
	return contracts.Output{Data: map[string]any{"queued": len(ids), "lastQueuedSourceId": lastQueued}, Events: []contracts.Event{{Type: "memory.ingestion.range_requeued", AggregateType: "memory_event_inbox", AggregateID: strconv.FormatInt(first, 10), Payload: map[string]any{"firstSourceId": first, "lastSourceId": last, "queued": len(ids)}}}}, nil
}

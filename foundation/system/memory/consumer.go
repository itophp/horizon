package memory

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	canonical "horizon/foundation/kernel/memory"
	"horizon/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Consumer reads the durable event log. The inbox is independent of the
// in-process Event Hub subscribers and makes discovery and processing restartable.
type Consumer struct{ pool *pgxpool.Pool }

func NewConsumer(pool *pgxpool.Pool) *Consumer { return &Consumer{pool: pool} }

// Run processes bounded batches until cancellation. Failures are reported by
// code only; database errors and original event payloads are never logged.
func (c *Consumer) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	metrics := time.NewTicker(time.Minute)
	defer metrics.Stop()
	for {
		if err := c.Once(ctx); err != nil && ctx.Err() == nil {
			slog.Error("memory event ingestion failed", "code", "ingestion_unavailable")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-metrics.C:
			if c.pool != nil {
				stats, err := dbgen.New(c.pool).MemoryIngestionStats(ctx)
				if err != nil {
					slog.Error("memory ingestion metrics unavailable", "code", "storage_error")
				} else {
					slog.Info("memory ingestion metrics", "pending", stats.Pending, "dead", stats.Dead, "completed", stats.Completed)
				}
			}
		}
	}
}

// Once discovers and processes a bounded batch; callers can use it in isolated
// integration tests to exercise crash recovery and duplicate delivery.
func (c *Consumer) Once(ctx context.Context) error {
	if c == nil || c.pool == nil {
		return errors.New("memory ingestion unavailable")
	}
	if _, err := dbgen.New(c.pool).MemoryDiscoverEvents(ctx, 100); err != nil {
		return err
	}
	q := dbgen.New(c.pool)
	for _, project := range []func(context.Context, int32) (int64, error){q.MemoryProjectInvocationStarts, q.MemoryProjectInvocationResults, q.MemoryProjectAgentToolStarts, q.MemoryProjectAgentToolResults, q.MemoryProjectRunStarts, q.MemoryProjectRunResults} {
		if _, err := project(ctx, 100); err != nil {
			return err
		}
	}
	for range 100 {
		processed, err := c.consume(ctx)
		if err != nil {
			return err
		}
		if !processed {
			break
		}
	}
	return nil
}

func (c *Consumer) consume(ctx context.Context) (bool, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	row, err := q.MemoryClaimEvent(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// cap_events.created_at is when the durable source recorded the event;
	// it is the only timestamp currently available from this source.
	actor := canonical.Actor{ID: "unknown", Type: "unknown"}
	if row.AccountID > 0 {
		actor = canonical.Actor{ID: strconv.FormatInt(row.AccountID, 10), Type: "user"}
	}
	e := canonical.Envelope{EventID: row.EventID, EventType: row.EventType, OccurredAt: row.CreatedAt.Time,
		TenantID: row.TenantID, DepartmentID: row.DepartmentID, Actor: actor, Source: canonical.Source{Type: "domain_event"},
		Resource:     canonical.Resource{Type: row.AggregateType, ID: row.AggregateID},
		InvocationID: row.InvocationID, CorrelationID: row.CorrelationID, CausationID: row.CausationID, Depth: int(row.TriggerDepth), Sensitivity: "internal"}
	// The durable event log is trusted as a source of event metadata only.
	// No business payload is copied without an explicit per-type policy.
	e, err = (canonical.Policy{row.EventType: {}}).Normalize(e, time.Now())
	if err != nil {
		return failEvent(ctx, tx, q, row, "invalid_event")
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return failEvent(ctx, tx, q, row, "invalid_event")
	}
	// Roll back a failed insert without releasing the inbox claim.
	insertTx, err := tx.Begin(ctx)
	if err != nil {
		return false, err
	}
	_, err = dbgen.New(insertTx).MemoryInsertEvent(ctx, dbgen.MemoryInsertEventParams{
		TenantID: e.TenantID, OrganizationID: e.OrganizationID, EventID: e.EventID, EventType: e.EventType, EventVersion: e.EventVersion,
		OccurredAt: pgtype.Timestamptz{Time: e.OccurredAt, Valid: true}, ActorID: e.Actor.ID, ActorType: e.Actor.Type,
		DepartmentID: e.DepartmentID, InvocationID: e.InvocationID,
		SessionID: e.SessionID, TaskID: e.TaskID, WorkflowInstanceID: e.WorkflowInstanceID,
		SourceType: e.Source.Type, SourceID: e.Source.ID, SourceModule: e.Source.Module,
		ResourceType: e.Resource.Type, ResourceID: e.Resource.ID, ActionName: e.ActionName, Result: e.Result, TraceID: e.TraceID,
		CorrelationID: e.CorrelationID, CausationID: e.CausationID, TriggerDepth: int32(e.Depth),
		Sensitivity: e.Sensitivity, Payload: payload,
		CandidatePartitions: e.CandidatePartitions,
	})
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if rollbackErr := insertTx.Rollback(ctx); rollbackErr != nil {
			return false, rollbackErr
		}
		return failEvent(ctx, tx, q, row, "storage_error")
	}
	if err = insertTx.Commit(ctx); err != nil {
		return false, err
	}
	if err = q.MemoryCompleteEvent(ctx, dbgen.MemoryCompleteEventParams{SourceEventID: row.SourceEventID, TenantID: row.TenantID}); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func failEvent(ctx context.Context, tx pgx.Tx, q *dbgen.Queries, row dbgen.MemoryClaimEventRow, code string) (bool, error) {
	delay := (row.Attempts + 1) * 5
	if delay > 60 {
		delay = 60
	}
	if err := q.MemoryFailEvent(ctx, dbgen.MemoryFailEventParams{SourceEventID: row.SourceEventID, TenantID: row.TenantID, MaxAttempts: 5, DelaySeconds: delay, ErrorCode: code}); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

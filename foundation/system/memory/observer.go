package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	canonical "horizon/foundation/kernel/memory"
	"horizon/internal/repository/dbgen"
)

// Observer is a bounded best-effort transport for non-business telemetry.
// Business facts must continue through the transactional outbox instead.
type Observer struct {
	pool      *pgxpool.Pool
	queue     chan canonical.Envelope
	submitted atomic.Uint64
	stored    atomic.Uint64
	dropped   atomic.Uint64
	failed    atomic.Uint64
}

func NewObserver(pool *pgxpool.Pool) *Observer {
	return &Observer{pool: pool, queue: make(chan canonical.Envelope, 512)}
}

var observationPolicy = canonical.Policy{
	"activity.http.completed":      {"method": {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "OTHER"}, "status": {"1xx", "2xx", "3xx", "4xx", "5xx"}, "duration": {"lt100ms", "lt1s", "gte1s"}},
	"system.exception.detected":    {"kind": {"panic", "http_5xx", "workflow_node"}},
	"capability.invocation.denied": {"reason": {"origin", "permission", "policy"}},
	"workflow.node.started":        {"kind": {"capability", "agent"}},
	"workflow.node.completed":      {"kind": {"capability", "agent"}},
	"workflow.node.interrupted":    {"kind": {"capability", "agent"}},
	"workflow.node.failed":         {"kind": {"capability", "agent"}},
}

// Submit never blocks a request, and normalizes before anything enters the queue.
func (o *Observer) Submit(e canonical.Envelope) {
	if o == nil || o.pool == nil {
		return
	}
	clean, err := observationPolicy.Normalize(e, time.Now())
	if err != nil {
		o.dropped.Add(1)
		slog.Warn("memory observation rejected", "code", "invalid_event")
		return
	}
	select {
	case o.queue <- clean:
		o.submitted.Add(1)
	default:
		o.dropped.Add(1)
		slog.Warn("memory observation dropped", "code", "queue_full")
	}
}

func (o *Observer) Run(ctx context.Context) {
	metrics := time.NewTicker(time.Minute)
	defer metrics.Stop()
	for {
		select {
		case <-ctx.Done():
			if len(o.queue) > 0 {
				slog.Warn("memory observations not delivered", "code", "shutdown", "count", len(o.queue))
			}
			return
		case <-metrics.C:
			slog.Info("memory observation metrics", "submitted", o.submitted.Load(), "delivered", o.stored.Load(), "dropped", o.dropped.Load(), "failed", o.failed.Load(), "queued", len(o.queue))
		case e := <-o.queue:
			var err error
			for attempt := 0; attempt < 5; attempt++ {
				err = o.store(ctx, e)
				if err == nil || ctx.Err() != nil {
					break
				}
				timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					o.dropped.Add(1)
					slog.Warn("memory observations not delivered", "code", "shutdown", "count", len(o.queue)+1)
					return
				case <-timer.C:
				}
			}
			if err != nil && ctx.Err() == nil {
				o.failed.Add(1)
				slog.Error("memory observation delivery failed", "code", "storage_error")
			} else if err == nil {
				o.stored.Add(1)
			} else {
				o.dropped.Add(1)
				slog.Warn("memory observations not delivered", "code", "shutdown", "count", len(o.queue)+1)
				return
			}
		}
	}
}

func (o *Observer) store(ctx context.Context, e canonical.Envelope) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	_, err = dbgen.New(o.pool).MemoryInsertEvent(ctx, dbgen.MemoryInsertEventParams{TenantID: e.TenantID, OrganizationID: e.OrganizationID,
		EventID: e.EventID, EventType: e.EventType, EventVersion: e.EventVersion, OccurredAt: pgtype.Timestamptz{Time: e.OccurredAt, Valid: true},
		ActorID: e.Actor.ID, ActorType: e.Actor.Type, DepartmentID: e.DepartmentID, SourceType: e.Source.Type, SourceID: e.Source.ID,
		SourceModule: e.Source.Module, ResourceType: e.Resource.Type, ResourceID: e.Resource.ID, ActionName: e.ActionName, Result: e.Result, InvocationID: e.InvocationID,
		SessionID: e.SessionID, TaskID: e.TaskID, WorkflowInstanceID: e.WorkflowInstanceID,
		TraceID: e.TraceID, CorrelationID: e.CorrelationID, CausationID: e.CausationID, TriggerDepth: int32(e.Depth), Sensitivity: e.Sensitivity, CandidatePartitions: e.CandidatePartitions, Payload: payload})
	return err
}

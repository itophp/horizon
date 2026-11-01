// Package outbox implements the transactional outbox: business code appends the
// event in the same transaction as its data, and a relay publishes it afterwards.
// Publishing is at-least-once, so downstream consumers must deduplicate.
package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"baseline/services/api/internal/platform/eventhub"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Meta carries the causal chain of the invocation that produced an event.
type Meta struct {
	InvocationID  string
	AccountID     int64
	DepartmentID  int64
	CorrelationID string
	CausationID   string
	Depth         int
}

// Event is a pending business fact.
type Event struct {
	TenantID      string
	EventID       string
	Type          string
	AggregateType string
	AggregateID   string
	Payload       map[string]any
	Meta          Meta
}

// NewEvent builds an event with a fresh public id.
func NewEvent(tenant, eventType, aggregateType, aggregateID string, payload map[string]any, meta Meta) Event {
	var raw [16]byte
	eventID := ""
	if _, err := rand.Read(raw[:]); err == nil {
		eventID = "evt_" + hex.EncodeToString(raw[:])
	}
	return Event{
		TenantID: tenant, EventID: eventID, Type: eventType,
		AggregateType: aggregateType, AggregateID: aggregateID, Payload: payload, Meta: meta,
	}
}

// Valid reports whether the event can be persisted.
func (e Event) Valid() bool {
	return e.TenantID != "" && e.EventID != "" && e.Type != "" && e.AggregateType != "" && e.AggregateID != ""
}

// AppendTx writes the outbox row inside the caller's transaction.
func AppendTx(ctx context.Context, q *dbgen.Queries, event Event) error {
	if !event.Valid() {
		return nil
	}
	raw, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	return q.CapAppendOutbox(ctx, dbgen.CapAppendOutboxParams{
		TenantID: event.TenantID, EventID: event.EventID, EventType: event.Type,
		AggregateType: event.AggregateType, AggregateID: event.AggregateID, Payload: raw,
		InvocationID: event.Meta.InvocationID, AccountID: event.Meta.AccountID,
		DepartmentID: event.Meta.DepartmentID, CorrelationID: event.Meta.CorrelationID,
		CausationID: event.Meta.CausationID, TriggerDepth: int32(event.Meta.Depth),
	})
}

// Relay publishes pending outbox rows into the durable event log.
type Relay struct {
	pool        *pgxpool.Pool
	hub         *eventhub.Hub
	batch       int
	interval    time.Duration
	maxAttempts int
}

// NewRelay builds a relay.
func NewRelay(pool *pgxpool.Pool, hub *eventhub.Hub) *Relay {
	return &Relay{pool: pool, hub: hub, batch: 20, interval: 500 * time.Millisecond, maxAttempts: 5}
}

// Run drains the outbox until the context is cancelled.
func (r *Relay) Run(ctx context.Context) {
	timer := time.NewTicker(r.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_, _ = r.Once(ctx)
		}
	}
}

// Once publishes at most one batch and returns how many rows were handled.
func (r *Relay) Once(ctx context.Context) (int, error) {
	if r == nil || r.pool == nil {
		return 0, nil
	}
	rows, err := r.claim(ctx)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	handled := 0
	for _, row := range rows {
		if err := r.publish(ctx, row); err != nil {
			r.retry(ctx, row)
			continue
		}
		handled++
	}
	return handled, nil
}

type claimed struct {
	id            int64
	tenant        string
	eventID       string
	eventType     string
	aggregateType string
	aggregateID   string
	payload       map[string]any
	invocationID  string
	accountID     int64
	departmentID  int64
	correlationID string
	causationID   string
	depth         int
	attempts      int
}

func (r *Relay) claim(ctx context.Context) ([]claimed, error) {
	rows, err := dbgen.New(r.pool).CapClaimOutbox(ctx, int32(r.batch))
	if err != nil {
		return nil, err
	}
	out := make([]claimed, 0, len(rows))
	for _, row := range rows {
		payload := map[string]any{}
		if len(row.Payload) > 0 {
			_ = json.Unmarshal(row.Payload, &payload)
		}
		out = append(out, claimed{
			id: row.ID, tenant: row.TenantID, eventID: row.EventID, eventType: row.EventType,
			aggregateType: row.AggregateType, aggregateID: row.AggregateID, payload: payload,
			invocationID: row.InvocationID, accountID: row.AccountID, departmentID: row.DepartmentID,
			correlationID: row.CorrelationID, causationID: row.CausationID, depth: int(row.TriggerDepth),
			attempts: int(row.Attempts),
		})
	}
	return out, nil
}

func (r *Relay) publish(ctx context.Context, row claimed) error {
	raw, err := json.Marshal(row.payload)
	if err != nil {
		return err
	}
	q := dbgen.New(r.pool)
	if err = q.CapAppendEvent(ctx, dbgen.CapAppendEventParams{
		TenantID: row.tenant, EventID: row.eventID, EventType: row.eventType,
		AggregateType: row.aggregateType, AggregateID: row.aggregateID, Payload: raw,
		InvocationID: row.invocationID, AccountID: row.accountID, DepartmentID: row.departmentID,
		CorrelationID: row.correlationID, CausationID: row.causationID, TriggerDepth: int32(row.depth),
	}); err != nil {
		return err
	}
	if err = q.CapMarkOutboxPublished(ctx, row.id); err != nil {
		return err
	}
	if r.hub != nil {
		r.hub.Dispatch(ctx, eventhub.Event{
			TenantID: row.tenant, EventID: row.eventID, Type: row.eventType,
			AggregateType: row.aggregateType, AggregateID: row.aggregateID, Payload: row.payload,
			InvocationID: row.invocationID, AccountID: row.accountID, DepartmentID: row.departmentID,
			CorrelationID: row.correlationID, CausationID: row.causationID, Depth: row.depth,
			CreatedAt: time.Now().UTC(),
		})
	}
	return nil
}

func (r *Relay) retry(ctx context.Context, row claimed) {
	q := dbgen.New(r.pool)
	if row.attempts >= r.maxAttempts {
		_ = q.CapMarkOutboxFailed(ctx, row.id)
		return
	}
	delay := 5 * row.attempts
	if delay > 60 {
		delay = 60
	}
	_ = q.CapRescheduleOutbox(ctx, dbgen.CapRescheduleOutboxParams{ID: row.id, DelaySeconds: int32(delay)})
}

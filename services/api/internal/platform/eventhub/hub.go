// Package eventhub is the durable business event log plus the in-process fan-out
// that wakes AI triggers. Delivery is at-least-once: consumers deduplicate on the
// event id, never on ordering.
package eventhub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Event is one business fact.
type Event struct {
	ID            int64
	TenantID      string
	EventID       string
	Type          string
	AggregateType string
	AggregateID   string
	Payload       map[string]any
	InvocationID  string
	AccountID     int64
	DepartmentID  int64
	CorrelationID string
	CausationID   string
	Depth         int
	CreatedAt     time.Time
	AIStatus      string
	AIPolicy      string
	AIRunID       int64
}

// Subscriber handles an event. It runs off the request path.
type Subscriber func(ctx context.Context, event Event)

// Hub fans events out to registered subscribers.
type Hub struct {
	mu   sync.RWMutex
	subs []Subscriber
}

// NewHub builds an empty hub.
func NewHub() *Hub { return &Hub{} }

// Subscribe registers a handler.
func (h *Hub) Subscribe(sub Subscriber) {
	h.mu.Lock()
	h.subs = append(h.subs, sub)
	h.mu.Unlock()
}

// Dispatch delivers an event to every subscriber in its own goroutine so a slow
// consumer can never delay the outbox relay or an HTTP response.
func (h *Hub) Dispatch(ctx context.Context, event Event) {
	h.mu.RLock()
	subs := append([]Subscriber{}, h.subs...)
	h.mu.RUnlock()
	for _, sub := range subs {
		go func(fn Subscriber) {
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("event subscriber panicked", "event_type", event.Type, "error_type", "panic")
				}
			}()
			fn(ctx, event)
		}(sub)
	}
}

// Store reads the durable event log.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds the reader.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns the newest events with the outcome of their AI trigger, if any.
func (s *Store) List(ctx context.Context, tenant string, accountID int64, reviewer bool, limit int) ([]Event, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := dbgen.New(s.pool).CapListEvents(ctx, dbgen.CapListEventsParams{TenantID: tenant, AccountID: accountID, Reviewer: reviewer, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, Event{
			ID: row.ID, TenantID: tenant, EventID: row.EventID, Type: row.EventType,
			AggregateType: row.AggregateType, AggregateID: row.AggregateID, Payload: decode(row.Payload),
			InvocationID: row.InvocationID, AccountID: row.AccountID, DepartmentID: row.DepartmentID,
			CorrelationID: row.CorrelationID, CausationID: row.CausationID, Depth: int(row.TriggerDepth),
			CreatedAt: row.CreatedAt.Time, AIStatus: row.AiStatus, AIPolicy: row.AiPolicy, AIRunID: row.AiRunID,
		})
	}
	return out, nil
}

func decode(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

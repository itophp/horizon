// Package audit is the read model of capability invocations: the durable answer
// to "who did what, through which entry point, and what happened". Writes happen
// inside the gateway transaction; this package only projects and lists.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Invocation is one audited call.
type Invocation struct {
	ID            int64
	InvocationID  string
	Capability    string
	Version       string
	Origin        string
	Status        string
	Input         json.RawMessage
	Digest        string
	Result        json.RawMessage
	ErrorCode     string
	ApprovalID    int64
	CorrelationID string
	CausationID   string
	Depth         int
	Duration      time.Duration
	TraceID       string
	CreatedAt     time.Time
	FinishedAt    time.Time
}

// Store reads the invocation table.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds the reader.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns the caller's own invocations, newest first.
func (s *Store) List(ctx context.Context, tenant string, accountID int64, limit int) ([]Invocation, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := dbgen.New(s.pool).CapListInvocations(ctx, dbgen.CapListInvocationsParams{TenantID: tenant, AccountID: accountID, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Invocation, 0, len(rows))
	for _, row := range rows {
		out = append(out, Invocation{
			ID: row.ID, InvocationID: row.InvocationID, Capability: row.Capability,
			Version: row.CapabilityVersion, Origin: row.Origin, Status: row.Status,
			Input: json.RawMessage(row.Input), Digest: row.InputDigest, Result: json.RawMessage(row.Result),
			ErrorCode: row.ErrorCode, ApprovalID: row.ApprovalID, CorrelationID: row.CorrelationID,
			CausationID: row.CausationID, Depth: int(row.TriggerDepth),
			Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
			CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
		})
	}
	return out, nil
}

// Get returns one invocation owned by the caller.
func (s *Store) Get(ctx context.Context, tenant string, accountID int64, invocationID string) (*Invocation, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	row, err := dbgen.New(s.pool).CapGetInvocation(ctx, dbgen.CapGetInvocationParams{TenantID: tenant, AccountID: accountID, InvocationID: invocationID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, contracts.Problem("not_found")
	}
	if err != nil {
		return nil, err
	}
	invocation := Invocation{
		ID: row.ID, InvocationID: row.InvocationID, Capability: row.Capability,
		Version: row.CapabilityVersion, Origin: row.Origin, Status: row.Status,
		Input: json.RawMessage(row.Input), Digest: row.InputDigest, Result: json.RawMessage(row.Result),
		ErrorCode: row.ErrorCode, ApprovalID: row.ApprovalID, CorrelationID: row.CorrelationID,
		CausationID: row.CausationID, Depth: int(row.TriggerDepth),
		Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
		CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
	}
	return &invocation, nil
}

func (s *Store) ListByCorrelation(ctx context.Context, tenant, correlationID string) ([]Invocation, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	rows, err := dbgen.New(s.pool).CapListInvocationsByCorrelation(ctx, dbgen.CapListInvocationsByCorrelationParams{TenantID: tenant, CorrelationID: correlationID})
	if err != nil {
		return nil, err
	}
	out := make([]Invocation, 0, len(rows))
	for _, row := range rows {
		out = append(out, Invocation{
			ID: row.ID, InvocationID: row.InvocationID, Capability: row.Capability,
			Version: row.CapabilityVersion, Origin: row.Origin, Status: row.Status,
			Input: json.RawMessage(row.Input), Digest: row.InputDigest, Result: json.RawMessage(row.Result),
			ErrorCode: row.ErrorCode, ApprovalID: row.ApprovalID, CorrelationID: row.CorrelationID,
			CausationID: row.CausationID, Depth: int(row.TriggerDepth),
			Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
			CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
		})
	}
	return out, nil
}

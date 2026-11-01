// Package approval persists human confirmations for capabilities that policy
// marks as approval bound. It is storage plus a single conditional decision; the
// resume path lives in the invocation gateway.
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status values.
const (
	Pending  = "pending"
	Approved = "approved"
	Rejected = "rejected"
	Expired  = "expired"
	Executed = "executed"
	Failed   = "failed"
)

// Approval is one confirmation card.
type Approval struct {
	ID            int64
	TenantID      string
	AccountID     int64
	DepartmentID  int64
	Capability    string
	Version       string
	InvocationID  string
	Risk          string
	Kind          string
	Status        string
	Arguments     json.RawMessage
	ActionKey     string
	Digest        string
	RequestedAt   time.Time
	ExpiresAt     time.Time
	DecidedAt     time.Time
	DecidedBy     int64
	Note          string
	ExecutionCode string
}

// Store owns the approval table.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds the store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Create writes or reuses the pending card inside the caller's transaction.
func (s *Store) Create(ctx context.Context, q *dbgen.Queries, a Approval, ttl time.Duration) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, contracts.Problem("unavailable")
	}
	if ttl <= 0 || ttl > time.Hour {
		ttl = 10 * time.Minute
	}
	arguments := a.Arguments
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	return q.CapCreateApproval(ctx, dbgen.CapCreateApprovalParams{
		TenantID:          a.TenantID,
		AccountID:         a.AccountID,
		DepartmentID:      a.DepartmentID,
		Capability:        a.Capability,
		CapabilityVersion: a.Version,
		InvocationID:      a.InvocationID,
		Risk:              a.Risk,
		Approval:          a.Kind,
		Arguments:         arguments,
		ArgumentsDigest:   a.Digest,
		ActionKey:         a.ActionKey,
		ExpiresMinutes:    int32(ttl.Minutes()),
	})
}

// Get returns one card, tenant scoped.
func (s *Store) Get(ctx context.Context, tenant string, id int64) (*Approval, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	row, err := dbgen.New(s.pool).CapGetApproval(ctx, dbgen.CapGetApprovalParams{TenantID: tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, contracts.Problem("not_found")
	}
	if err != nil {
		return nil, err
	}
	out := fromRow(row)
	return &out, nil
}

// List returns pending cards for the requester, or every card for a reviewer.
func (s *Store) List(ctx context.Context, tenant string, accountID int64, reviewer bool) ([]Approval, error) {
	if s == nil || s.pool == nil {
		return nil, contracts.Problem("unavailable")
	}
	rows, err := dbgen.New(s.pool).CapListApprovals(ctx, dbgen.CapListApprovalsParams{TenantID: tenant, AccountID: accountID, Reviewer: reviewer})
	if err != nil {
		return nil, err
	}
	out := make([]Approval, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromRow(row))
	}
	return out, nil
}

// Decide applies the decision exactly once.
func (s *Store) Decide(ctx context.Context, tenant string, id, accountID int64, approve bool, note string) (bool, error) {
	if s == nil || s.pool == nil {
		return false, contracts.Problem("unavailable")
	}
	status := Rejected
	if approve {
		status = Approved
	}
	affected, err := dbgen.New(s.pool).CapDecideApproval(ctx, dbgen.CapDecideApprovalParams{
		TenantID:     tenant,
		ID:           id,
		Status:       status,
		DecidedBy:    pgtype.Int8{Int64: accountID, Valid: true},
		DecisionNote: note,
	})
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

// Complete records the execution outcome.
func (s *Store) Complete(ctx context.Context, tenant string, id int64, status, code string) error {
	if s == nil || s.pool == nil {
		return contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).CapCompleteApproval(ctx, dbgen.CapCompleteApprovalParams{TenantID: tenant, ID: id, Status: status, ExecutionCode: code})
}

// Expire closes cards whose window elapsed.
func (s *Store) Expire(ctx context.Context, tenant string) error {
	if s == nil || s.pool == nil {
		return contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).CapExpireApprovals(ctx, tenant)
}

func fromRow(row dbgen.CapApproval) Approval {
	return Approval{
		ID: row.ID, TenantID: row.TenantID, AccountID: row.AccountID, DepartmentID: row.DepartmentID,
		Capability: row.Capability, Version: row.CapabilityVersion, InvocationID: row.InvocationID,
		Risk: row.Risk, Kind: row.Approval, Status: row.Status, Arguments: json.RawMessage(row.Arguments),
		Digest: row.ArgumentsDigest, RequestedAt: row.RequestedAt.Time, ExpiresAt: row.ExpiresAt.Time,
		DecidedAt: row.DecidedAt.Time, DecidedBy: int64Of(row.DecidedBy), Note: row.DecisionNote,
		ExecutionCode: row.ExecutionCode,
	}
}

func int64Of(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

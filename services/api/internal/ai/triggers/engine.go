// Package triggers turns business events into AI tasks. It owns the policy
// lookup, deduplication, depth limiting and the independent status of each
// trigger execution; the agent itself runs in the harness.
package triggers

import (
	"context"
	"errors"
	"time"

	"baseline/services/api/internal/ai/harness"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/eventhub"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Execution status values.
const (
	Queued    = "queued"
	Running   = "running"
	Completed = "completed"
	Failed    = "failed"
	Skipped   = "skipped"
)

// Policy is one event → agent rule.
type Policy struct {
	ID          string
	Version     int
	TenantID    string
	EventType   string
	Agent       string
	Mode        string
	Instruction string
	MaxDepth    int
}

// Launcher runs an event triggered task and returns its task id.
type Launcher interface {
	RunTrigger(ctx context.Context, in harness.TriggerInput) (*harness.Run, error)
}

// Store reads policies and records executions.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds the store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// PoliciesForEvent returns the most specific policy per policy id: a tenant
// policy wins over the global one for the same id.
func (s *Store) PoliciesForEvent(ctx context.Context, tenant, eventType string) ([]Policy, error) {
	if s == nil || s.pool == nil {
		return nil, nil
	}
	rows, err := dbgen.New(s.pool).AITriggerPoliciesForEvent(ctx, dbgen.AITriggerPoliciesForEventParams{TenantID: tenant, EventType: eventType})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]Policy, 0, len(rows))
	for _, row := range rows {
		if seen[row.PolicyID] {
			continue
		}
		seen[row.PolicyID] = true
		out = append(out, Policy{
			ID: row.PolicyID, Version: int(row.Version), TenantID: row.TenantID,
			EventType: row.EventType, Agent: row.Agent, Mode: row.Mode,
			Instruction: row.Instruction, MaxDepth: int(row.MaxDepth),
		})
	}
	return out, nil
}

// Claim creates the execution row. It returns false when the event was already
// handled by the same policy, which is the at-least-once deduplication.
func (s *Store) Claim(ctx context.Context, tenant string, policy Policy, eventID string) (int64, bool, error) {
	if s == nil || s.pool == nil {
		return 0, false, nil
	}
	id, err := dbgen.New(s.pool).AITriggerExecutionCreate(ctx, dbgen.AITriggerExecutionCreateParams{
		TenantID: tenant, PolicyID: policy.ID, PolicyVersion: int32(policy.Version), EventID: eventID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// Started marks the execution running.
func (s *Store) Started(ctx context.Context, tenant string, id int64, runID int64) {
	if s == nil || s.pool == nil {
		return
	}
	_ = dbgen.New(s.pool).AITriggerExecutionStart(ctx, dbgen.AITriggerExecutionStartParams{
		TenantID: tenant, ID: id, RunID: int8(runID),
	})
}

// Finish closes the execution with its own status, independent of the event and
// of the business invocation that produced it.
func (s *Store) Finish(ctx context.Context, tenant string, id int64, status, code string, runID int64) {
	if s == nil || s.pool == nil {
		return
	}
	_ = dbgen.New(s.pool).AITriggerExecutionFinish(ctx, dbgen.AITriggerExecutionFinishParams{
		TenantID: tenant, ID: id, Status: status, ErrorCode: code, RunID: int8(runID),
	})
}

// Engine dispatches events to policies.
type Engine struct {
	policies *Store
	hub      *eventhub.Hub
	launcher Launcher
	identity *identity.Service
	timeout  time.Duration
}

// NewEngine builds the engine.
func NewEngine(policies *Store, hub *eventhub.Hub, launcher Launcher, iam *identity.Service) *Engine {
	return &Engine{policies: policies, hub: hub, launcher: launcher, identity: iam, timeout: 3 * time.Minute}
}

// Start subscribes to the event hub.
func (e *Engine) Start(context.Context) {
	if e.hub != nil {
		e.hub.Subscribe(e.Handle)
	}
}

// Handle evaluates every matching policy for an event.
func (e *Engine) Handle(ctx context.Context, event eventhub.Event) {
	if e.policies == nil || event.TenantID == "" || event.AccountID == 0 {
		return
	}
	policies, err := e.policies.PoliciesForEvent(ctx, event.TenantID, event.Type)
	if err != nil {
		return
	}
	for _, policy := range policies {
		policy := policy
		if event.Depth >= policy.MaxDepth {
			e.skip(ctx, event, policy, "depth_exceeded")
			continue
		}
		executionID, claimed, err := e.policies.Claim(ctx, event.TenantID, policy, event.EventID)
		if err != nil || !claimed {
			continue
		}
		go e.run(event, policy, executionID)
	}
}

// skip records a policy that matched but was not allowed to run, so operators
// can distinguish "no policy" from "blocked by depth".
func (e *Engine) skip(ctx context.Context, event eventhub.Event, policy Policy, code string) {
	id, claimed, err := e.policies.Claim(ctx, event.TenantID, policy, event.EventID)
	if err != nil || !claimed {
		return
	}
	e.policies.Finish(ctx, event.TenantID, id, Skipped, code, 0)
}

func (e *Engine) run(event eventhub.Event, policy Policy, executionID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	principal, err := e.identity.PrincipalForTask(ctx, event.TenantID, event.AccountID, event.DepartmentID)
	if err != nil {
		e.policies.Finish(ctx, event.TenantID, executionID, Skipped, "forbidden", 0)
		return
	}
	ctx = identity.WithPrincipal(ctx, principal)
	run, err := e.launcher.RunTrigger(ctx, harness.TriggerInput{
		PolicyID: policy.ID, PolicyVersion: policy.Version,
		Instruction: policy.Instruction, Event: event,
	})
	if err != nil {
		code := governance.Code(err)
		status := Failed
		if code == "unavailable" || code == "forbidden" || code == "unauthenticated" {
			status = Skipped
		}
		e.policies.Finish(ctx, event.TenantID, executionID, status, code, 0)
		return
	}
	e.policies.Started(ctx, event.TenantID, executionID, run.ID)
	e.policies.Finish(ctx, event.TenantID, executionID, Completed, run.ErrorCode, run.ID)
}

func int8(value int64) pgtype.Int8 {
	if value == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

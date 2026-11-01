package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/platform/outbox"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// storedResult is the durable projection returned on idempotent replay.
type storedResult struct {
	Data     any      `json:"data"`
	Summary  string   `json:"summary"`
	EventIDs []string `json:"eventIds"`
}

// run books, executes and closes one invocation. The approval resumption path
// reuses the already booked row instead of creating a second one.
func (g *Gateway) run(ctx context.Context, p *identity.Principal, entry registry.Entry, request Request, invocationID, traceID, digest string, redacted json.RawMessage, resumed bool) (*contracts.Result, error) {
	started := g.now()
	q := dbgen.New(g.pool)
	if resumed {
		if err := q.CapResumeInvocation(ctx, dbgen.CapResumeInvocationParams{TenantID: p.TenantID, InvocationID: invocationID}); err != nil {
			return nil, err
		}
	} else {
		input := redacted
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		if _, err := q.CapCreateInvocation(ctx, dbgen.CapCreateInvocationParams{
			TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID,
			InvocationID: invocationID, Capability: entry.Definition.Name, CapabilityVersion: entry.Definition.Version,
			Origin: string(request.Origin), IdempotencyKey: request.IdempotencyKey,
			Status: string(contracts.StatusExecuting), Input: input, InputDigest: digest,
			CorrelationID: traceID, CausationID: request.CausationID,
			TriggerDepth: int32(request.TriggerDepth), TraceID: traceID,
		}); err != nil {
			// A concurrent retry with the same key loses the unique index race;
			// return the winner's stored outcome instead of a 500.
			if isUnique(err) && request.IdempotencyKey != "" {
				if replay, found, replayErr := g.replayWait(ctx, p, entry.Definition, request.IdempotencyKey, digest); replayErr == nil && found {
					return replay, nil
				}
			}
			return nil, err
		}
	}
	output, eventIDs, err := g.execute(ctx, p, entry, request, invocationID, traceID)
	if err != nil {
		code := mapCode(err)
		_ = g.finish(ctx, p.TenantID, invocationID, contracts.StatusFailed, nil, code, time.Since(started))
		// A denial is a request level outcome, not a business failure: the caller
		// must see 401/403 instead of a 200 envelope, exactly like the catalog
		// check before the executor ran.
		if code == "forbidden" || code == "unauthenticated" {
			return nil, contracts.Problem(code)
		}
		return &contracts.Result{
			InvocationID: invocationID, Status: contracts.StatusFailed, ErrorCode: code,
			TraceID: traceID, EventIDs: []string{},
		}, nil
	}
	stored := storedResult{Data: output.Data, Summary: output.Summary, EventIDs: eventIDs}
	if entry.Definition.TransientResult {
		stored.Data = nil
	}
	raw, marshalErr := json.Marshal(stored)
	if marshalErr != nil {
		_ = g.finish(ctx, p.TenantID, invocationID, contracts.StatusFailed, nil, "internal", time.Since(started))
		return &contracts.Result{InvocationID: invocationID, Status: contracts.StatusFailed, ErrorCode: "internal", TraceID: traceID, EventIDs: []string{}}, nil
	}
	if err = g.finish(ctx, p.TenantID, invocationID, contracts.StatusSucceeded, raw, "", time.Since(started)); err != nil {
		return nil, err
	}
	return &contracts.Result{
		InvocationID: invocationID, Status: contracts.StatusSucceeded, Data: output.Data,
		EventIDs: eventIDs, TraceID: traceID,
	}, nil
}

// execute prefers the transactional path so business data and outbox events
// commit together. Executors that own their transaction still get events, just
// in a following transaction; that gap is documented as best effort.
func (g *Gateway) execute(ctx context.Context, p *identity.Principal, entry registry.Entry, request Request, invocationID, traceID string) (contracts.Output, []string, error) {
	ctx = withInvocation(ctx, InvocationContext{Capability: entry.Definition.Name, Version: entry.Definition.Version, InvocationID: invocationID, CorrelationID: traceID, Depth: request.TriggerDepth, IdempotencyKey: request.IdempotencyKey, Origin: request.Origin, InTransaction: entry.Handler.ExecuteTx != nil})
	meta := outbox.Meta{
		InvocationID: invocationID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID,
		CorrelationID: traceID, CausationID: request.CausationID, Depth: request.TriggerDepth,
	}
	if entry.Handler.ExecuteTx != nil {
		tx, err := g.pool.Begin(ctx)
		if err != nil {
			return contracts.Output{}, nil, err
		}
		defer tx.Rollback(ctx)
		output, execErr := entry.Handler.ExecuteTx(ctx, tx, request.Input)
		if execErr != nil {
			return contracts.Output{}, nil, execErr
		}
		eventIDs, eventErr := appendEvents(ctx, dbgen.New(tx), entry.Definition, output.Events, p.TenantID, meta)
		if eventErr != nil {
			return contracts.Output{}, nil, eventErr
		}
		if err = tx.Commit(ctx); err != nil {
			return contracts.Output{}, nil, err
		}
		return output, eventIDs, nil
	}
	output, err := entry.Handler.Execute(ctx, request.Input)
	if err != nil {
		return contracts.Output{}, nil, err
	}
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return contracts.Output{}, nil, err
	}
	defer tx.Rollback(ctx)
	eventIDs, err := appendEvents(ctx, dbgen.New(tx), entry.Definition, output.Events, p.TenantID, meta)
	if err != nil {
		return contracts.Output{}, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return contracts.Output{}, nil, err
	}
	return output, eventIDs, nil
}

// appendEvents enforces the declared event contract before writing the outbox.
func appendEvents(ctx context.Context, q *dbgen.Queries, definition contracts.Definition, events []contracts.Event, tenant string, meta outbox.Meta) ([]string, error) {
	if len(events) == 0 {
		if definition.ReadOnly {
			return []string{}, nil
		}
		return nil, contracts.Problem("event_missing")
	}
	ids := make([]string, 0, len(events))
	for _, event := range events {
		if !declared(definition.SuccessEvents, event.Type) {
			return nil, contracts.Problem("invalid")
		}
		if event.AggregateType == "" || event.AggregateID == "" {
			return nil, contracts.Problem("invalid")
		}
		pending := outbox.NewEvent(tenant, event.Type, event.AggregateType, event.AggregateID, event.Payload, meta)
		if !pending.Valid() {
			return nil, contracts.Problem("internal")
		}
		if err := outbox.AppendTx(ctx, q, pending); err != nil {
			return nil, err
		}
		ids = append(ids, pending.EventID)
	}
	return ids, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func declared(events []string, eventType string) bool {
	for _, event := range events {
		if event == eventType {
			return true
		}
	}
	return false
}

func (g *Gateway) finish(ctx context.Context, tenant, invocationID string, status contracts.Status, result []byte, code string, duration time.Duration) error {
	return dbgen.New(g.pool).CapFinishInvocation(ctx, dbgen.CapFinishInvocationParams{
		TenantID: tenant, InvocationID: invocationID, Status: string(status),
		Result: result, ErrorCode: code, DurationMs: int32(duration.Milliseconds()),
	})
}

// mapCode normalizes domain and infrastructure failures to stable codes.
func mapCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *contracts.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "not_found"
	}
	return governance.Code(err)
}

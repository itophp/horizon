package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"horizon/internal/repository/dbgen"
)

func (e *Engine) step(ctx context.Context, s Step, values map[string]any, iteration int) (map[string]any, error) {
	if err := e.stepEvent(ctx, s.ID, "STEP_STARTED"); err != nil {
		return nil, err
	}
	result, err := e.stepBody(ctx, s, values, iteration)
	if err != nil {
		return nil, err
	}
	if err = e.stepEvent(ctx, s.ID, "STEP_FINISHED"); err != nil {
		return nil, err
	}
	return result, nil
}
func (e *Engine) stepEvent(ctx context.Context, name, kind string) error {
	state, err := current(ctx)
	if err != nil {
		return err
	}
	tx, err := e.Core.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = q.RuntimeTenantLock(ctx, fmt.Sprintf("%s:run-event:%d", state.Tenant, state.ID)); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"stepName": name})
	if err != nil {
		return err
	}
	if _, err = q.AIRunAppendEvent(ctx, dbgen.AIRunAppendEventParams{TenantID: state.Tenant, RunID: state.ID, EventType: kind, Payload: payload}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

package harness

import (
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// TaskEvent carries AG-UI lifecycle metadata, never model inputs or tool arguments.
type TaskEvent struct {
	ID, Type, RunID, ThreadID string
	Timestamp                 int64
	Snapshot                  map[string]any
}

func (s *Service) Events(ctx context.Context, id, after int64, send func(TaskEvent) error) error {
	original, err := s.authorize(ctx)
	if err != nil {
		return err
	}
	q := dbgen.New(s.Core.Pool)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		p, err := s.Core.Identity.PrincipalForTask(ctx, original.TenantID, original.Account.ID, original.Account.DepartmentID)
		if err != nil || !p.Can("assistant:use") {
			return contracts.Problem("forbidden")
		}
		if original.TokenHash != "" {
			session, e := q.IAMSession(ctx, original.TokenHash)
			if e != nil || session.ActiveDepartmentID != original.Account.DepartmentID {
				return contracts.Problem("unauthenticated")
			}
		}
		run, err := q.AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: id})
		if err != nil || run.DepartmentID != p.Account.DepartmentID {
			return contracts.Problem("not_found")
		}
		rows, err := q.AIRunEvents(ctx, dbgen.AIRunEventsParams{TenantID: p.TenantID, RunID: id, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID, Sequence: after})
		if err != nil {
			return err
		}
		thread := run.SessionID
		if thread == "" {
			thread = "run:" + strconv.FormatInt(id, 10)
		}
		for _, r := range rows {
			snapshot := map[string]any{}
			if json.Unmarshal(r.Payload, &snapshot) != nil {
				return contracts.Problem("internal")
			}
			if err = send(TaskEvent{ID: strconv.FormatInt(r.Sequence, 10), Type: r.EventType, RunID: strconv.FormatInt(id, 10), ThreadID: thread, Timestamp: r.CreatedAt.Time.UnixMilli(), Snapshot: snapshot}); err != nil {
				return err
			}
			after = r.Sequence
		}
		if len(rows) == 0 {
			if err = send(TaskEvent{ID: strconv.FormatInt(after, 10), Type: "STATE_SNAPSHOT", RunID: strconv.FormatInt(id, 10), ThreadID: thread, Timestamp: time.Now().UnixMilli(), Snapshot: map[string]any{"status": run.Status, "code": run.ErrorCode}}); err != nil {
				return err
			}
		}
		if len(rows) == 200 {
			continue
		}
		if run.Status == StatusCompleted || run.Status == StatusFailed || run.Status == StatusCancelled {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return nil
		case <-ticker.C:
		}
	}
}

package automation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"horizon/foundation/ai/harness"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

const (
	workerInterval = 30 * time.Second
	workerBatch    = 5
	maxConcurrent  = 4
	runTimeout     = 5 * time.Minute
)

// Worker turns due automation tasks into agent runs. Discovery only claims ids
// and writes the occurrence row; the agent itself runs outside the claim
// transaction through the same Gateway every other caller uses. Each run
// re-resolves the owner's Principal, so a revoked grant or a disabled account
// stops the schedule instead of reusing stale authority.
type Worker struct {
	Pool     *pgxpool.Pool
	Launcher *harness.Service
	Identity *identity.Service
	slots    chan struct{}
}

// NewWorker builds the scheduler. A nil dependency makes Run a no-op so the
// API stays bootable without a database or model configuration.
func NewWorker(pool *pgxpool.Pool, launcher *harness.Service, iam *identity.Service) *Worker {
	return &Worker{Pool: pool, Launcher: launcher, Identity: iam, slots: make(chan struct{}, maxConcurrent)}
}

// Run schedules due tasks and launches queued runs until the context ends.
func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.Pool == nil || w.Launcher == nil || w.Identity == nil {
		return
	}
	w.recoverInterrupted(ctx)
	w.tick(ctx)
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	w.scheduleDue(ctx)
	w.dispatch(ctx)
}

// recoverInterrupted marks runs whose worker died as failed instead of leaving
// them "running" forever. The 15 minute lease is longer than the 5 minute run
// timeout, so a healthy run is never touched.
func (w *Worker) recoverInterrupted(ctx context.Context) {
	if _, err := dbgen.New(w.Pool).AutomationRunInterrupted(ctx); err != nil {
		slog.Error("automation run recovery failed", "error", err)
	}
}

func (w *Worker) scheduleDue(ctx context.Context) {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		slog.Error("automation schedule transaction failed", "error", err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	tasks, err := q.AutomationDueTasks(ctx, workerBatch)
	if err != nil {
		slog.Error("automation due task discovery failed", "error", err)
		return
	}
	for _, task := range tasks {
		loc, err := time.LoadLocation(task.Timezone)
		if err != nil {
			slog.Error("automation task timezone is invalid", "task", task.ID)
			continue
		}
		now := time.Now().UTC()
		enabled := false
		nextRunAt := pgtype.Timestamptz{}
		if task.ScheduleKind != ScheduleOnce {
			if next, ok := NextOccurrence(task.ScheduleKind, task.RunAt.Time, loc, now, scheduleDayInt(task.ScheduleDay)); ok {
				enabled = true
				nextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
			}
		}
		active, err := q.AutomationRunActive(ctx, dbgen.AutomationRunActiveParams{TenantID: task.TenantID, TaskID: task.ID})
		if err != nil {
			slog.Error("automation overlap check failed", "task", task.ID, "error", err)
			continue
		}
		status, code := "queued", ""
		if active {
			// The previous occurrence is still queued, running or waiting for a
			// human: record the skip instead of stacking a second run.
			status, code = "skipped", "overlap"
		}
		_, err = q.AutomationRunQueue(ctx, dbgen.AutomationRunQueueParams{
			TenantID: task.TenantID, TaskID: task.ID,
			OccurrenceKey: task.NextRunAt.Time.UTC().Format(time.RFC3339),
			ScheduledFor:  task.NextRunAt, TriggerKind: "schedule",
			Status: status, ErrorCode: code,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("automation occurrence insert failed", "task", task.ID, "error", err)
			continue
		}
		if _, err = q.AutomationTaskAdvance(ctx, dbgen.AutomationTaskAdvanceParams{
			TenantID: task.TenantID, ID: task.ID, NextRunAt: nextRunAt,
			Enabled: enabled, Expected: task.NextRunAt,
		}); err != nil {
			slog.Error("automation task advance failed", "task", task.ID, "error", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		slog.Error("automation schedule commit failed", "error", err)
	}
}

func (w *Worker) dispatch(ctx context.Context) {
	free := cap(w.slots) - len(w.slots)
	if free <= 0 {
		return
	}
	if free > workerBatch {
		free = workerBatch
	}
	rows, err := dbgen.New(w.Pool).AutomationRunsClaim(ctx, int32(free))
	if err != nil {
		slog.Error("automation run claim failed", "error", err)
		return
	}
	for _, row := range rows {
		w.slots <- struct{}{}
		go func(row dbgen.AutomationRunsClaimRow) {
			defer func() { <-w.slots }()
			w.execute(ctx, row)
		}(row)
	}
}

func (w *Worker) execute(ctx context.Context, row dbgen.AutomationRunsClaimRow) {
	p, err := w.Identity.PrincipalForTask(ctx, row.TenantID, row.OwnerID, row.DepartmentID)
	if err != nil {
		w.finish(ctx, row.TenantID, row.ID, "skipped", "forbidden", 0)
		w.stop(row.TenantID, row.OwnerID, row.TaskID)
		return
	}
	runCtx, cancel := context.WithTimeout(identity.WithPrincipal(ctx, p), runTimeout)
	defer cancel()
	run, err := w.Launcher.RunAutomation(runCtx, harness.AutomationInput{
		RunID: row.ID, TaskID: row.TaskID, TaskName: row.TaskName, Instruction: row.TaskDescription,
	})
	status, code := "completed", ""
	var aiRunID int64
	if err != nil {
		code = governance.Code(err)
		status = "failed"
		if code == "forbidden" || code == "unauthenticated" {
			status = "skipped"
		}
	} else {
		aiRunID = run.ID
		switch run.Status {
		case harness.StatusCompleted:
			status = "completed"
		case harness.StatusAwaitingApproval:
			status = "awaiting_approval"
		case harness.StatusCancelled:
			status, code = "failed", "cancelled"
		default:
			status, code = "failed", run.ErrorCode
		}
	}
	w.finish(ctx, row.TenantID, row.ID, status, code, aiRunID)
	w.record(ctx, row.TenantID, row.TaskID, status == "failed")
	if status == "skipped" {
		// A revoked owner would fail every future occurrence; stop the schedule
		// and leave the reason on the run instead of retrying silently.
		w.stop(row.TenantID, row.OwnerID, row.TaskID)
	}
}

func (w *Worker) finish(ctx context.Context, tenant string, id int64, status, code string, aiRunID int64) {
	write, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	ai := pgtype.Int8{}
	if aiRunID > 0 {
		ai = pgtype.Int8{Int64: aiRunID, Valid: true}
	}
	if _, err := dbgen.New(w.Pool).AutomationRunFinish(write, dbgen.AutomationRunFinishParams{
		TenantID: tenant, ID: id, Status: status, ErrorCode: code, AiRunID: ai,
	}); err != nil {
		slog.Error("automation run finish failed", "run", id, "status", status, "error", err)
	}
}

func (w *Worker) record(ctx context.Context, tenant string, taskID int64, failed bool) {
	if err := dbgen.New(w.Pool).AutomationTaskRecordRun(ctx, dbgen.AutomationTaskRecordRunParams{
		TenantID: tenant, ID: taskID, Failed: failed,
	}); err != nil {
		slog.Error("automation task run record failed", "task", taskID, "error", err)
	}
}

func (w *Worker) stop(tenant string, ownerID, taskID int64) {
	write, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := dbgen.New(w.Pool).AutomationTaskSetEnabled(write, dbgen.AutomationTaskSetEnabledParams{
		TenantID: tenant, OwnerID: ownerID, ID: taskID, Enabled: false,
	}); err != nil {
		slog.Error("automation task disable failed", "task", taskID, "error", err)
	}
}

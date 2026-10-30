package settings

import (
	gen "baseline/services/api/gen/settings"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"log/slog"
	"time"
)

func scheduleRecord(r dbgen.GovSettingsScheduleRow) *gen.SettingSchedule {
	return &gen.SettingSchedule{ID: r.ID, DraftID: r.DraftID, ActorID: r.ActorID, DepartmentID: r.DepartmentID, BaseRevision: r.BaseRevision, Reason: r.Reason, ScheduledAt: governance.Time(r.ScheduledAt), CreatedAt: governance.Time(r.CreatedAt), FinishedAt: governance.Time(r.FinishedAt), Status: r.Status, ErrorCode: r.ErrorCode, VersionID: r.VersionID.Int64}
}
func (s *Service) Schedules(ctx context.Context, _ *gen.SchedulesPayload) (res []*gen.SettingSchedule, err error) {
	defer apiError(&err)
	p, _, err := s.Check(ctx, "settings:read", "workspace", 0, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovSettingsSchedules(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	res = []*gen.SettingSchedule{}
	for _, r := range rows {
		res = append(res, scheduleRecord(dbgen.GovSettingsScheduleRow(r)))
	}
	return res, nil
}

// All authorization, file metadata and setting reads share the publishing transaction.
func (s *Service) bound(q *dbgen.Queries) *Service {
	c := s.Core.WithReadQueries(q)
	if f, ok := s.Logos.(*files.Service); ok {
		copy := *f
		copy.Core = c
		c.Logos = &copy
		c.Storage = &copy
	}
	return New(c)
}
func (s *Service) Schedule(ctx context.Context, payload *gen.SchedulePayload) (res *gen.SettingSchedule, err error) {
	defer apiError(&err)
	_, _, err = s.Check(ctx, "settings:publish", "workspace", 0, true)
	if err != nil {
		return nil, err
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, payload.ScheduledAt)
	if err != nil {
		return nil, governance.Problem("invalid")
	}
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		b := s.bound(q)
		row, e := b.Core.Draft(ctx, "settings:publish", payload.ID)
		if e != nil {
			return e
		}
		if row.Domain != "settings" || row.Status != "draft" || row.BaseRevision != payload.BaseRevision {
			return governance.Problem("conflict")
		}
		t, e := b.ScopeTarget(ctx, p.TenantID, row.ScopeKey)
		if e != nil {
			return e
		}
		if e = b.Lock(ctx, q, p, t, row.ResourceKey, payload.BaseRevision); e != nil {
			return e
		}
		changes, e := decodeChanges(row.Payload)
		if e != nil {
			return e
		}
		changes, sensitive, e := b.validate(ctx, p, t, changes, true)
		if e != nil {
			return e
		}
		values, e := b.combined(ctx, q, p, t, changes)
		if e != nil {
			return e
		}
		if e = b.validateStorage(ctx, p, values, changes); e != nil {
			return e
		}
		confirmed := row.Sensitive || sensitive
		if confirmed {
			if e = b.Consume(ctx, q, p, payload.Proof, "settings.schedule", payload.ID, payload.BaseRevision); e != nil {
				return e
			}
		}
		id, e = q.GovCreateSettingsSchedule(ctx, dbgen.GovCreateSettingsScheduleParams{TenantID: p.TenantID, DraftID: row.ID, ActorID: p.Account.ID, DepartmentID: p.Account.DepartmentID, BaseRevision: payload.BaseRevision, DraftHash: governance.Hash(string(row.Payload)), SensitiveConfirmed: confirmed, ScheduledAt: pgtype.Timestamptz{Time: at.UTC(), Valid: true}})
		if errors.Is(e, pgx.ErrNoRows) {
			return governance.Problem("invalid")
		}
		if e != nil {
			return e
		}
		detail := []byte(fmt.Sprintf(`{"scheduledAt":%q}`, at.UTC().Format(time.RFC3339)))
		return b.Audit(ctx, q, p, t, "settings.schedule", fmt.Sprintf("schedule:%d", id), detail)
	})
	if err != nil {
		return nil, err
	}
	row, err := s.Q.GovSettingsSchedule(ctx, dbgen.GovSettingsScheduleParams{TenantID: p.TenantID, ID: id})
	if err != nil {
		return nil, err
	}
	return scheduleRecord(row), nil
}

func (s *Service) CancelSchedule(ctx context.Context, payload *gen.CancelSchedulePayload) (err error) {
	defer apiError(&err)
	p, t, err := s.Check(ctx, "settings:publish", "workspace", 0, true)
	if err != nil {
		return err
	}
	return s.Transaction(ctx, func(q *dbgen.Queries) error {
		if _, e := q.GovSettingsSchedule(ctx, dbgen.GovSettingsScheduleParams{TenantID: p.TenantID, ID: payload.ID}); e != nil {
			return e
		}
		if _, e := q.GovCancelSettingsSchedule(ctx, dbgen.GovCancelSettingsScheduleParams{TenantID: p.TenantID, ID: payload.ID}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return governance.Problem("conflict")
			}
			return e
		}
		return s.Audit(ctx, q, p, t, "settings.schedule.cancel", fmt.Sprintf("schedule:%d", payload.ID), []byte(`{}`))
	})
}
func (s *Service) publishWithin(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, row dbgen.GovDraftRow, changes []*gen.ChangeEntry, base int64, proof string, scheduled bool) (int64, error) {
	if e := s.Lock(ctx, q, p, t, row.ResourceKey, base); e != nil {
		return 0, e
	}
	if !scheduled {
		pending, e := q.GovHasPendingSettingsSchedule(ctx, dbgen.GovHasPendingSettingsScheduleParams{TenantID: p.TenantID, DraftID: row.ID})
		if e != nil {
			return 0, e
		}
		if pending {
			return 0, governance.Problem("conflict")
		}
	}
	values, e := s.combined(ctx, q, p, t, changes)
	if e != nil {
		return 0, e
	}
	if e = s.validateStorage(ctx, p, values, changes); e != nil {
		return 0, e
	}
	if row.Sensitive && !scheduled {
		if e = s.Consume(ctx, q, p, proof, "settings.publish", row.ID, base); e != nil {
			return 0, e
		}
	}
	if _, e = q.GovPublishDraft(ctx, dbgen.GovPublishDraftParams{TenantID: p.TenantID, ID: row.ID}); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return 0, governance.Problem("conflict")
		}
		return 0, e
	}
	return s.write(ctx, q, p, t, changes, base, row.Reason)
}
func (s *Service) executeScheduled(ctx context.Context, q *dbgen.Queries, job dbgen.GovClaimSettingsScheduleRow) (int64, error) {
	b := s.bound(q)
	p, e := b.Identity.PrincipalForTask(ctx, job.TenantID, job.ActorID, job.DepartmentID)
	if e != nil {
		return 0, e
	}
	ctx = identity.WithPrincipal(ctx, p)
	if _, _, e = b.Check(ctx, "settings:publish", "workspace", 0, true); e != nil {
		return 0, e
	}
	row, e := b.Core.Draft(ctx, "settings:publish", job.DraftID)
	if e != nil {
		return 0, e
	}
	if row.Domain != "settings" || row.Status != "draft" || row.BaseRevision != job.BaseRevision || governance.Hash(string(row.Payload)) != job.DraftHash {
		return 0, governance.Problem("conflict")
	}
	raw, e := decodeChanges(row.Payload)
	if e != nil {
		return 0, e
	}
	changes, sensitive, e := b.validate(ctx, p, governance.Target{Kind: "workspace", Key: "workspace:0"}, raw, true)
	if e != nil {
		return 0, e
	}
	if (row.Sensitive || sensitive) && !job.SensitiveConfirmed {
		return 0, governance.Problem("forbidden")
	}
	return b.publishWithin(ctx, q, p, governance.Target{Kind: "workspace", Key: "workspace:0"}, row, changes, job.BaseRevision, "", true)
}

// A row lock is the claim: crash/disconnect rolls back to pending; completion and
// configuration commit atomically. A savepoint permits a durable failed result.
func (s *Service) ProcessScheduledOnce(ctx context.Context) (err error) {
	if err = s.Available(); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if e := tx.Rollback(context.WithoutCancel(ctx)); e != nil && !errors.Is(e, pgx.ErrTxClosed) {
			slog.Error("settings schedule rollback failed", "code", governance.Code(e))
		}
	}()
	q := dbgen.New(tx)
	job, err := q.GovClaimSettingsSchedule(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	nested, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	version, executionErr := s.executeScheduled(ctx, dbgen.New(nested), job)
	status, code := "published", ""
	if executionErr != nil {
		if err = nested.Rollback(context.WithoutCancel(ctx)); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		status, code = "failed", governance.Code(executionErr)
	} else if err = nested.Commit(ctx); err != nil {
		return err
	}
	versionID := pgtype.Int8{}
	if status == "published" {
		versionID = governance.Int(version)
	}
	if err = q.GovFinishSettingsSchedule(ctx, dbgen.GovFinishSettingsScheduleParams{TenantID: job.TenantID, ID: job.ID, Status: status, ErrorCode: code, VersionID: versionID}); err != nil {
		return err
	}
	if err = q.GovAudit(ctx, dbgen.GovAuditParams{TenantID: job.TenantID, ScopeKey: "workspace:0", ActorID: job.ActorID, Action: "settings.schedule." + status, ResourceKey: fmt.Sprintf("schedule:%d", job.ID), Detail: []byte(fmt.Sprintf(`{"code":%q}`, code))}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) RunScheduled(ctx context.Context) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if e := s.ProcessScheduledOnce(ctx); e != nil && ctx.Err() == nil {
				slog.Error("settings schedule worker failed", "code", governance.Code(e))
			}
		}
	}
}

//go:build integration

package tests

import (
	filegen "baseline/services/api/gen/files"
	iamgen "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/settings"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/files"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/settings"
	"baseline/services/api/internal/transport"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestSettingsScheduledPublication(t *testing.T) {
	_, pool, admin := iamFixture(t)
	ctx := context.Background()
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	iam := identity.New(pool, false)
	options := governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir()}
	core := governance.New(pool, iam, options)
	files.New(core)
	handler := transport.HandlerWithCore(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, core, iam)
	root := loginIAM(t, handler, "root-admin")
	worker := settings.New(core)
	revision := func(t *testing.T) int64 {
		w := root.request(t, "GET", "/api/admin/settings", nil, false)
		checkStatus(t, w, 200)
		return decodeBody[gen.SettingsWorkspace](t, w).Revision
	}
	draft := func(t *testing.T, client iamClient, key, value string) gen.ChangeRecord {
		w := client.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, BaseRevision: revision(t), Reason: "Scheduled settings test", Changes: []*gen.ChangeEntry{{Key: key, Operation: "set", Value: value}}}, true)
		checkStatus(t, w, 201)
		return decodeBody[gen.ChangeRecord](t, w)
	}
	schedule := func(t *testing.T, client iamClient, d gen.ChangeRecord, proof string) gen.SettingSchedule {
		w := client.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, Proof: proof, ScheduledAt: time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)}, true)
		checkStatus(t, w, 201)
		return decodeBody[gen.SettingSchedule](t, w)
	}
	due := func(t *testing.T, id int64) {
		t.Helper()
		if _, e := pool.Exec(ctx, "UPDATE gov_settings_schedules SET scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1", id); e != nil {
			t.Fatal(e)
		}
	}
	status := func(t *testing.T, id int64) gen.SettingSchedule {
		w := root.request(t, "GET", "/api/admin/settings/schedules", nil, false)
		checkStatus(t, w, 200)
		for _, r := range decodeBody[[]gen.SettingSchedule](t, w) {
			if r.ID == id {
				return r
			}
		}
		t.Fatal("missing schedule", id)
		return gen.SettingSchedule{}
	}
	proof := func(t *testing.T, d gen.ChangeRecord, operation string) string {
		w := root.request(t, "POST", "/api/auth/reauthenticate", &gen.ReauthenticatePayload{Password: "TestOnly!2026Secure", Operation: operation, ResourceID: d.ID, Revision: d.BaseRevision}, true)
		checkStatus(t, w, 200)
		return decodeBody[gen.ReauthProof](t, w).Proof
	}
	t.Run("pending cancellation and invalid time", func(t *testing.T) {
		d := draft(t, root, "branding.name", `"Cancelled release"`)
		for _, at := range []time.Time{time.Now().Add(-time.Hour), time.Now().Add(366 * 24 * time.Hour)} {
			checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, ScheduledAt: at.UTC().Format(time.RFC3339)}, true), 400)
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, ScheduledAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, false), 403)
		job := schedule(t, root, d, "")
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if revision(t) != 0 || status(t, job.ID).Status != "pending" {
			t.Fatal("published before due time")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: d.BaseRevision}, true), 409)
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, ScheduledAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, true), 409)
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/schedules/%d/cancel", job.ID), nil, true), 204)
		due(t, job.ID)
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if status(t, job.ID).Status != "cancelled" || revision(t) != 0 {
			t.Fatal("cancelled schedule executed")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/schedules/%d/cancel", job.ID), nil, true), 409)
	})
	t.Run("concurrent workers publish exactly once", func(t *testing.T) {
		job := schedule(t, root, draft(t, root, "branding.name", `"Scheduled brand"`), "")
		due(t, job.ID)
		results := make(chan error, 2)
		for range 2 {
			go func() { results <- worker.ProcessScheduledOnce(ctx) }()
		}
		for range 2 {
			if err = <-results; err != nil {
				t.Fatal(err)
			}
		}
		result := status(t, job.ID)
		if result.Status != "published" || result.VersionID == 0 || result.FinishedAt == "" || revision(t) != 1 {
			t.Fatal("schedule not committed exactly once", result)
		}
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if revision(t) != 1 {
			t.Fatal("duplicate publication")
		}
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/schedules/%d/cancel", job.ID), nil, true), 409)
	})
	t.Run("version conflict preserves the latest settings", func(t *testing.T) {
		job := schedule(t, root, draft(t, root, "branding.name", `"Stale brand"`), "")
		current := draft(t, root, "display.hour24", "false")
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", current.ID), &gen.PublishPayload{BaseRevision: current.BaseRevision}, true), 200)
		due(t, job.ID)
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		result := status(t, job.ID)
		if result.Status != "failed" || result.ErrorCode != "conflict" || revision(t) != 2 {
			t.Fatal("stale schedule overwrote newer version", result)
		}
	})
	t.Run("sensitive intent uses one scheduling proof and survives logout", func(t *testing.T) {
		d := draft(t, root, "security.passwordMin", "14")
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, ScheduledAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, true), 403)
		wrong := proof(t, d, "settings.publish")
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", d.ID), &gen.SchedulePayload{BaseRevision: d.BaseRevision, Proof: wrong, ScheduledAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, true), 403)
		token := proof(t, d, "settings.schedule")
		job := schedule(t, root, d, token)
		other := draft(t, root, "security.passwordMin", "16")
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/schedule", other.ID), &gen.SchedulePayload{BaseRevision: other.BaseRevision, Proof: token, ScheduledAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, true), 403)
		if _, err = pool.Exec(ctx, "UPDATE iam_sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE tenant_id='tenant-a' AND account_id=$1", admin.ID); err != nil {
			t.Fatal(err)
		}
		due(t, job.ID)
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		root = loginIAM(t, handler, "root-admin")
		if status(t, job.ID).Status != "published" || revision(t) != 3 {
			t.Fatal("confirmed sensitive schedule did not execute")
		}
	})
	t.Run("revocation fails closed and is tenant isolated", func(t *testing.T) {
		w := root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: "timed-publisher", FullName: "Timed publisher", Password: "TestOnly!2026Secure", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, Status: "active", RoleIds: []int64{}, Overrides: []*iamgen.PermissionGrant{{Permission: "settings:read", Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}, {Permission: "settings:update", Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}, {Permission: "settings:publish", Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}}}, true)
		checkStatus(t, w, 201)
		account := decodeBody[iamgen.AccountRecord](t, w)
		publisher := loginIAM(t, handler, "timed-publisher")
		job := schedule(t, publisher, draft(t, publisher, "branding.footer", `"Publisher footer"`), "")
		inactive := schedule(t, publisher, draft(t, publisher, "branding.tagline", `"Inactive publisher"`), "")
		restricted, encodeErr := json.Marshal([]*iamgen.PermissionGrant{{Permission: "settings:publish", Effect: "allow", Scope: "DEPARTMENT", DepartmentIds: []int64{}}})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if _, err = pool.Exec(ctx, "UPDATE iam_accounts SET overrides=$2 WHERE tenant_id='tenant-a' AND id=$1", account.ID, restricted); err != nil {
			t.Fatal(err)
		}
		due(t, job.ID)
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if result := status(t, job.ID); result.Status != "failed" || result.ErrorCode != "forbidden" {
			t.Fatal("revoked publisher executed", result)
		}
		checkStatus(t, publisher.request(t, "GET", "/api/admin/settings/schedules", nil, false), 403)
		if _, err = pool.Exec(ctx, "UPDATE iam_accounts SET status='inactive' WHERE tenant_id='tenant-a' AND id=$1", account.ID); err != nil {
			t.Fatal(err)
		}
		due(t, inactive.ID)
		if err = worker.ProcessScheduledOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if result := status(t, inactive.ID); result.Status != "failed" || result.ErrorCode != "forbidden" {
			t.Fatal("inactive account executed", result)
		}
		for _, path := range []string{"/api/admin/settings/schedules"} {
			checkStatus(t, publisher.request(t, "GET", path, nil, false), 401)
		}
		if _, err = identity.Bootstrap(ctx, pool, "schedule-tenant-b", "schedule-foreign", "TestOnly!2026Secure"); err != nil {
			t.Fatal(err)
		}
		foreign := loginIAM(t, handler, "schedule-foreign")
		checkStatus(t, foreign.request(t, "POST", fmt.Sprintf("/api/admin/settings/schedules/%d/cancel", job.ID), nil, true), 404)
	})
	t.Run("scheduled logo uses a single database connection", func(t *testing.T) {
		cfg := pool.Config().Copy()
		cfg.MaxConns = 1
		single, e := pgxpool.NewWithConfig(ctx, cfg)
		if e != nil {
			t.Fatal(e)
		}
		defer single.Close()
		singleIAM := identity.New(single, false)
		singleCore := governance.New(single, singleIAM, options)
		files.New(singleCore)
		singleHandler := transport.HandlerWithCore(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, singleCore, singleIAM)
		client := loginIAM(t, singleHandler, "root-admin")
		var data bytes.Buffer
		if e = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
			t.Fatal(e)
		}
		w := client.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "scheduled-logo.png", Purpose: "logo", Content: data.Bytes()}, true)
		checkStatus(t, w, 201)
		file := decodeBody[filegen.ManagedFile](t, w)
		job := schedule(t, client, draft(t, client, "branding.logo", fmt.Sprint(file.ID)), "")
		due(t, job.ID)
		bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if e = settings.New(singleCore).ProcessScheduledOnce(bounded); e != nil {
			t.Fatal(e)
		}
		if status(t, job.ID).Status != "published" || revision(t) != 4 {
			t.Fatal("single-connection scheduled publication failed")
		}
	})
	t.Run("running worker processes due intent after restart", func(t *testing.T) {
		job := schedule(t, root, draft(t, root, "display.grouping", "false"), "")
		due(t, job.ID)
		loopCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		stopped := make(chan struct{})
		go func() { defer close(stopped); settings.New(core).RunScheduled(loopCtx) }()
		poll := time.NewTicker(30 * time.Millisecond)
		defer poll.Stop()
		for {
			select {
			case <-loopCtx.Done():
				t.Fatal("running scheduler did not publish before deadline")
			case <-poll.C:
				var state string
				if e := pool.QueryRow(ctx, "SELECT status FROM gov_settings_schedules WHERE id=$1", job.ID).Scan(&state); e != nil {
					t.Fatal(e)
				}
				if state == "published" {
					cancel()
					<-stopped
					if revision(t) != 5 {
						t.Fatal("worker did not commit settings")
					}
					return
				}
			}
		}
	})

}

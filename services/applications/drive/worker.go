package drive

import (
	"context"
	"log/slog"
	"time"

	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunCleanup discovers due work and invokes the same audited gateway as other
// callers. Discovery only schedules IDs; all state changes are capabilities.
func RunCleanup(ctx context.Context, pool *pgxpool.Pool, gw *gateway.Gateway) {
	if pool == nil || gw == nil {
		return
	}
	runCleanupOnce(ctx, pool, gw)
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			runCleanupOnce(ctx, pool, gw)
		}
	}
}

func runCleanupOnce(ctx context.Context, pool *pgxpool.Pool, gw *gateway.Gateway) {
	q := dbgen.New(pool)
	tenants, err := q.DriveCleanupTenants(ctx)
	if err != nil {
		slog.Error("drive cleanup discovery failed", "code", "unavailable")
		return
	}
	for _, tenant := range tenants {
		if ctx.Err() != nil {
			return
		}
		principal, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 0, DepartmentID: 0, Status: "active"}, tenant,
			[]*genidentity.PermissionGrant{{Permission: "drive:cleanup", Effect: "allow", Scope: "ALL"}})
		if err != nil {
			slog.Error("drive cleanup identity failed", "code", "internal")
			continue
		}
		systemCtx := identity.WithPrincipal(ctx, principal)
		batches, err := q.DriveCleanupBatchCandidates(ctx, tenant)
		if err != nil {
			slog.Error("drive cleanup batch discovery failed", "code", "unavailable")
			continue
		}
		for _, batch := range batches {
			input := contracts.Input{"spaceId": batch.SpaceID, "batchId": batch.ID}
			if !batch.PurgingAt.Valid && !invokeCleanup(systemCtx, gw, "drive.cleanup.claim", input) {
				continue
			}
			invokeCleanup(systemCtx, gw, "drive.cleanup.batch", input)
		}
		uploads, err := q.DriveCleanupUploadCandidates(ctx, tenant)
		if err != nil {
			slog.Error("drive cleanup upload discovery failed", "code", "unavailable")
			continue
		}
		for _, upload := range uploads {
			invokeCleanup(systemCtx, gw, "drive.cleanup.upload", contracts.Input{"spaceId": upload.SpaceID, "uploadId": upload.ID})
		}
	}
}

func invokeCleanup(ctx context.Context, gw *gateway.Gateway, name string, input contracts.Input) bool {
	key, err := governance.Token()
	if err != nil {
		slog.Error("drive cleanup key failed", "code", "internal")
		return false
	}
	result, err := gw.Invoke(ctx, gateway.Request{Name: name, Version: "1.0.0", Input: input,
		IdempotencyKey: "drive-cleanup:" + key, Origin: contracts.OriginEvent})
	if err != nil {
		slog.Error("drive cleanup invocation failed", "code", contracts.Code(err))
		return false
	}
	if result.Status != contracts.StatusSucceeded {
		slog.Error("drive cleanup execution failed", "code", result.ErrorCode)
		return false
	}
	return true
}

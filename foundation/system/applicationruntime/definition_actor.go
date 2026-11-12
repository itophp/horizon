package applicationruntime

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func definitionTask(ctx context.Context, q *dbgen.Queries, p *identity.Principal) (pgtype.Int8, error) {
	reference, present := gateway.CurrentTaskReference(ctx)
	if !present {
		return pgtype.Int8{}, nil
	}
	if reference.TenantID != p.TenantID || reference.AccountID != p.Account.ID || reference.DepartmentID != p.Account.DepartmentID {
		return pgtype.Int8{}, contracts.Problem("forbidden")
	}
	run, err := q.AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: reference.RunID})
	if err != nil {
		return pgtype.Int8{}, err
	}
	if run.DepartmentID != p.Account.DepartmentID || run.Status != "running" {
		return pgtype.Int8{}, contracts.Problem("forbidden")
	}
	return pgtype.Int8{Int64: run.ID, Valid: true}, nil
}

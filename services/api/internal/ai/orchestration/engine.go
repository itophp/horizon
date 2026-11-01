package orchestration

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/platform/capability/registry"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/compose"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type Engine struct {
	Core        *governance.Core
	gateway     *gateway.Gateway
	registry    *registry.Registry
	definitions map[string]Definition
	mu          sync.Mutex
	cache       map[string]compose.Runnable[map[string]any, map[string]any]
	AgentModel  ModelFactory
	err         error
}

func New(core *governance.Core, definitions []Definition) *Engine {
	e := &Engine{Core: core, definitions: map[string]Definition{}, cache: map[string]compose.Runnable[map[string]any, map[string]any]{}}
	for _, d := range definitions {
		key := d.Name + "@" + d.Version
		if _, exists := e.definitions[key]; exists {
			e.err = contracts.Problem("invalid")
			continue
		}
		raw, err := json.Marshal(d)
		if err != nil {
			e.err = err
			continue
		}
		var frozen Definition
		if err = json.Unmarshal(raw, &frozen); err != nil {
			e.err = err
			continue
		}
		e.definitions[key] = frozen
	}
	return e
}
func (e *Engine) Attach(g *gateway.Gateway, r *registry.Registry) error {
	if e.err != nil {
		return e.err
	}
	e.gateway = g
	e.registry = r
	for _, d := range e.definitions {
		if _, err := e.compile(context.Background(), d); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) Definitions() []map[string]any {
	out := []map[string]any{}
	for _, d := range e.definitions {
		out = append(out, map[string]any{"name": d.Name, "version": d.Version, "hash": d.Hash(), "inputSchema": d.Input.JSON()})
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["name"], out[i]["version"]) < fmt.Sprint(out[j]["name"], out[j]["version"])
	})
	return out
}

// StartTx only enqueues infrastructure state. It never invokes business nodes
// in the transaction; Gateway commits the accepted event with the run header.
func (e *Engine) StartTx(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	if e.gateway == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	name, _ := in["flow"].(string)
	version, _ := in["version"].(string)
	d, ok := e.definitions[name+"@"+version]
	if !ok {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	args, ok := in["input"].(map[string]any)
	if !ok {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if err = d.Input.Validate(contracts.Input(args)); err != nil {
		return contracts.Output{}, err
	}
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	q := dbgen.New(tx)
	startKey := "start:" + gateway.ChildKey(ctx, "workflow")
	if err = q.OrchStartLock(ctx, p.TenantID+":"+startKey); err != nil {
		return contracts.Output{}, err
	}
	id, err := q.OrchFindStart(ctx, dbgen.OrchFindStartParams{TenantID: p.TenantID, AccountID: p.Account.ID, CheckpointKey: startKey})
	if err == nil {
		return accepted(id), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, err
	}
	row, err := q.AICreateRun(ctx, dbgen.AICreateRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, DepartmentID: p.Account.DepartmentID, Agent: "orchestration", InstructionVersion: d.Hash(), Question: name, TraceID: meta.CorrelationID, CorrelationID: meta.CorrelationID, CausationID: meta.InvocationID, TriggerDepth: int32(meta.Depth + 1), Origin: string(contracts.OriginWorkflow), Context: []byte(`{}`)})
	if err != nil {
		return contracts.Output{}, err
	}
	m := manifest{Format: 1, Definition: d, Hash: d.Hash(), Input: args, Correlation: meta.CorrelationID, Parent: meta.InvocationID, Depth: meta.Depth + 1}
	state := execution{ID: row.ID, Tenant: p.TenantID, Account: p.Account.ID, Manifest: m}
	ctx = context.WithValue(ctx, executionKey{}, state)
	raw, err := json.Marshal(m)
	if err != nil {
		return contracts.Output{}, err
	}
	store := checkpointStore{core: e.Core, q: q}
	if err = store.Set(ctx, "manifest", raw); err != nil {
		return contracts.Output{}, err
	}
	if err = store.Set(ctx, startKey, []byte(`{}`)); err != nil {
		return contracts.Output{}, err
	}
	return accepted(row.ID), nil
}
func accepted(id int64) contracts.Output {
	return contracts.Output{Data: map[string]any{"runId": id, "status": "running"}, Events: []contracts.Event{{Type: "orchestration.workflow.accepted", AggregateType: "orchestration_checkpoint", AggregateID: fmt.Sprint(id), Payload: map[string]any{"runId": id}}}}
}

func (e *Engine) Query(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	id, err := runID(in)
	if err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(e.Core.Pool).AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.Agent != "orchestration" {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"runId": row.ID, "flow": row.Question, "hash": row.InstructionVersion, "status": row.Status, "errorCode": row.ErrorCode, "correlationId": row.CorrelationID}}, nil
}
func (e *Engine) CancelTx(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	id, err := runID(in)
	if err != nil {
		return contracts.Output{}, err
	}
	n, err := dbgen.New(tx).OrchSetStatus(ctx, dbgen.OrchSetStatusParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: id, Status: "cancelled"})
	if err != nil {
		return contracts.Output{}, err
	}
	if n != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if err := dbgen.New(tx).OrchCancelPendingApprovals(ctx, dbgen.OrchCancelPendingApprovalsParams{TenantID: p.TenantID, AccountID: p.Account.ID, RunID: id}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"runId": id, "status": "cancelled"}, Events: []contracts.Event{{Type: "orchestration.workflow.cancelled", AggregateType: "orchestration_checkpoint", AggregateID: fmt.Sprint(id), Payload: map[string]any{"runId": id}}}}, nil
}
func runID(in contracts.Input) (int64, error) {
	b, err := json.Marshal(in["runId"])
	if err != nil {
		return 0, err
	}
	var id int64
	if json.Unmarshal(b, &id) != nil || id < 1 {
		return 0, contracts.Problem("invalid")
	}
	return id, nil
}

// Run is a durable queue pump, not a scheduler: Eino owns node scheduling.
// Each claimed run is bounded below the SQL lease duration. A crash expires the
// lease; receipts suppress already committed capability effects on restart.
func (e *Engine) Run(ctx context.Context) {
	if e.Core == nil || e.Core.Pool == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Tick(ctx); err != nil {
				slog.Error("orchestration queue failed", "code", "unavailable")
			}
		}
	}
}
func (e *Engine) Tick(ctx context.Context) error {
	q := dbgen.New(e.Core.Pool)
	rows, err := q.OrchCandidates(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		token := fmt.Sprintf("%d", time.Now().UnixNano())
		_, err = q.OrchClaim(ctx, dbgen.OrchClaimParams{TenantID: row.TenantID, RunID: row.ID, AccountID: row.AccountID, LeaseToken: token})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		runctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err = e.execute(runctx, row)
		cancel()
		releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		releaseErr := q.OrchRelease(releaseCtx, dbgen.OrchReleaseParams{TenantID: row.TenantID, RunID: row.ID, LeaseToken: token})
		releaseCancel()
		if err != nil {
			return err
		}
		if releaseErr != nil {
			return releaseErr
		}
	}
	return nil
}
func (e *Engine) execute(ctx context.Context, row dbgen.OrchCandidatesRow) error {
	q := dbgen.New(e.Core.Pool)
	status := func(value, code string) error {
		_, err := q.OrchSetStatus(ctx, dbgen.OrchSetStatusParams{TenantID: row.TenantID, AccountID: row.AccountID, ID: row.ID, Status: value, ErrorCode: code})
		return err
	}
	p, err := e.Core.Identity.PrincipalForTask(ctx, row.TenantID, row.AccountID, row.DepartmentID)
	if err != nil || !p.Can("orchestration:run") {
		return status("failed", "forbidden")
	}
	ctx = identity.WithPrincipal(ctx, p)
	state := execution{ID: row.ID, Tenant: row.TenantID, Account: row.AccountID}
	ctx = context.WithValue(ctx, executionKey{}, state)
	store := checkpointStore{core: e.Core, q: q}
	raw, ok, err := store.Get(ctx, "manifest")
	if err != nil {
		return err
	}
	if !ok {
		return status("failed", "unavailable")
	}
	var m manifest
	if json.Unmarshal(raw, &m) != nil || m.Format != 1 || m.Hash != m.Definition.Hash() {
		return status("failed", "invalid")
	}
	currentDef, ok := e.definitions[m.Definition.Name+"@"+m.Definition.Version]
	if !ok || currentDef.Hash() != m.Hash {
		return status("failed", "version_mismatch")
	}
	state.Manifest = m
	ctx = context.WithValue(ctx, executionKey{}, state)
	r, err := e.compile(ctx, m.Definition)
	if err != nil {
		return status("failed", "invalid")
	}
	_, err = r.Invoke(ctx, map[string]any{"input": m.Input}, compose.WithCheckPointID("workflow"))
	if _, interrupted := compose.ExtractInterruptInfo(err); interrupted {
		pending, err := q.OrchPendingApprovals(ctx, dbgen.OrchPendingApprovalsParams{TenantID: row.TenantID, AccountID: row.AccountID, RunID: row.ID})
		if err != nil {
			return err
		}
		if pending > 0 {
			return status("awaiting_approval", "")
		}
		return status("running", "")
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil
		} // lease expiry retries from durable receipts
		waiting, compensationErr := e.compensate(ctx, m.Definition)
		if waiting {
			return status("awaiting_approval", "")
		}
		if compensationErr != nil {
			return status("failed", "manual_recovery_required")
		}
		return status("failed", contracts.Code(err))
	}
	return status("completed", "")
}

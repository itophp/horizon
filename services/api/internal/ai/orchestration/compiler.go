package orchestration

import (
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/gateway"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/compose"
	"strconv"
	"time"
)

func (e *Engine) compile(ctx context.Context, d Definition) (compose.Runnable[map[string]any, map[string]any], error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.cache[d.Hash()]; ok {
		return r, nil
	}
	if err := d.Validate(e.registry); err != nil {
		return nil, err
	}
	wf := compose.NewWorkflow[map[string]any, map[string]any]()
	for _, step := range d.Steps {
		s := step
		lambda := compose.InvokableLambda(func(ctx context.Context, in map[string]any) (map[string]any, error) { return e.step(ctx, s, in, 0) })
		var n *compose.WorkflowNode
		if s.Poll != nil || s.MaxAttempts > 1 {
			graph, err := e.loop(s)
			if err != nil {
				return nil, err
			}
			max := s.MaxAttempts
			if s.Poll != nil {
				max = s.Poll.MaxSteps
			}
			n = wf.AddGraphNode(s.ID, graph, compose.WithGraphCompileOptions(compose.WithMaxRunSteps(max+2)))
		} else {
			n = wf.AddLambdaNode(s.ID, lambda)
		}

		n.AddInput(compose.START, compose.MapFields("input", "input")) // retains root input; only explicit edges order work
		deps := map[string]bool{}
		for _, dep := range s.DependsOn {
			deps[dep] = true
		}
		for _, m := range s.Input {
			if m.From != "input" {
				deps[m.From] = true
			}
		}
		for dep := range deps {
			n.AddInput(dep, compose.ToField(dep))
		}
		wf.End().AddInput(s.ID, compose.ToField(s.ID))
	}
	r, err := wf.Compile(ctx, compose.WithCheckPointStore(checkpointStore{core: e.Core, q: dbgen.New(e.Core.Pool), prefix: "eino:"}))
	if err == nil {
		if len(e.cache) >= 128 {
			clear(e.cache)
		}
		e.cache[d.Hash()] = r
	}
	return r, err
}

type receipt struct {
	Invocation string         `json:"invocation"`
	Status     string         `json:"status"`
	Data       map[string]any `json:"data,omitempty"`
}

func (e *Engine) step(ctx context.Context, s Step, values map[string]any, iteration int) (map[string]any, error) {
	state, err := current(ctx)
	if err != nil {
		return nil, err
	}
	row, err := dbgen.New(e.Core.Pool).AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: state.Tenant, AccountID: state.Account, ID: state.ID})
	if err != nil {
		return nil, err
	}
	if row.Status != "running" && row.Status != "awaiting_approval" {
		return nil, contracts.Problem("cancelled")
	}
	principal, err := e.Core.Identity.PrincipalForTask(ctx, state.Tenant, state.Account, row.DepartmentID)
	if err != nil {
		return nil, contracts.Problem("forbidden")
	}
	ctx = identity.WithPrincipal(ctx, principal)
	store := checkpointStore{core: e.Core, q: dbgen.New(e.Core.Pool)}
	argsKey := "args:" + s.ID + ":" + strconv.Itoa(iteration)
	args := contracts.Input{}
	storedArgs, hasArgs, err := store.Get(ctx, argsKey)
	if err != nil {
		return nil, err
	}
	if hasArgs {
		if json.Unmarshal(storedArgs, &args) != nil {
			return nil, contracts.Problem("invalid")
		}
	} else {
		for k, v := range s.Constants {
			args[k] = v
		}
		for k, m := range s.Input {
			source, ok := values[m.From].(map[string]any)
			if !ok {
				return nil, contracts.Problem("invalid")
			}
			v, ok := source[m.Field]
			if !ok {
				return nil, contracts.Problem("invalid")
			}
			args[k] = v
		}
		encoded, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		if err = store.Set(ctx, argsKey, encoded); err != nil {
			return nil, err
		}
	}

	if s.Agent != nil {
		return e.agent(ctx, s, args)
	}
	key := "node:" + s.ID + ":" + strconv.Itoa(iteration)
	raw, ok, err := store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		var saved receipt
		if json.Unmarshal(raw, &saved) != nil {
			return nil, contracts.Problem("invalid")
		}
		if saved.Status == "succeeded" {
			return saved.Data, nil
		}
		inv, err := e.gateway.Invocation(ctx, saved.Invocation)
		if err != nil {
			return nil, err
		}
		switch inv.Status {
		case "succeeded":
			var result struct {
				Data map[string]any `json:"data"`
			}
			if json.Unmarshal(inv.Result, &result) != nil {
				return nil, contracts.Problem("invalid")
			}
			return e.saveReceipt(ctx, key, receipt{Invocation: saved.Invocation, Status: "succeeded", Data: result.Data})
		case "awaiting_approval", "executing":
			if inv.Status == "awaiting_approval" && timeExpired(inv.CreatedAt) {
				return nil, contracts.Problem("expired")
			}
			return nil, compose.Interrupt(ctx, "awaiting_approval")
		default:
			code := inv.ErrorCode
			if code == "" {
				code = "workflow_step_failed"
			}
			return nil, contracts.Problem(code)
		}
	}
	origin := contracts.OriginWorkflow
	if state.AgentTools {
		origin = contracts.OriginAgent
	}
	result, err := e.gateway.Invoke(ctx, gateway.Request{Name: s.Capability, Version: s.Version, Input: args, Origin: origin, CorrelationID: state.Manifest.Correlation, CausationID: state.Manifest.Parent, TriggerDepth: state.Manifest.Depth + 1, IdempotencyKey: fmt.Sprintf("orch:%d:%s:%d", state.ID, s.ID, iteration)})
	if err != nil {
		return nil, err
	}
	saved := receipt{Invocation: result.InvocationID, Status: string(result.Status)}
	if result.Status == contracts.StatusSucceeded {
		data, err := object(result.Data)
		if err != nil {
			return nil, err
		}
		saved.Data = data
	}
	out, err := e.saveReceipt(ctx, key, saved)
	if err != nil {
		return nil, err
	}
	if result.Status == contracts.StatusAwaitingApproval {
		return nil, compose.Interrupt(ctx, "awaiting_approval")
	}
	if result.Status != contracts.StatusSucceeded {
		code := result.ErrorCode
		if code == "" {
			code = "workflow_step_failed"
		}
		return nil, contracts.Problem(code)
	}
	return out, nil
}
func (e *Engine) saveReceipt(ctx context.Context, key string, r receipt) (map[string]any, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	err = (checkpointStore{core: e.Core, q: dbgen.New(e.Core.Pool)}).Set(ctx, key, b)
	return r.Data, err
}
func object(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return nil, contracts.Problem("invalid")
	}
	return out, nil
}

func timeExpired(t time.Time) bool { return time.Since(t) > gateway.ApprovalTTL }

// loop uses Eino Graph for both polling and bounded transient read retries.
func (e *Engine) loop(s Step) (*compose.Graph[map[string]any, map[string]any], error) {
	g := compose.NewGraph[map[string]any, map[string]any]()
	max := s.MaxAttempts
	if s.Poll != nil {
		max = s.Poll.MaxSteps
	}
	if err := g.AddLambdaNode("run", compose.InvokableLambda(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		iteration := 0
		if v, ok := in["__iteration"].(int); ok {
			iteration = v
		}
		if iteration >= max {
			return nil, contracts.Problem("step_limit")
		}
		out, err := e.step(ctx, s, in, iteration)
		retry := false
		if err != nil {
			code := contracts.Code(err)
			retry = s.Poll == nil && (code == "unavailable" || code == "rate_limited" || code == "timeout")
			if !retry || iteration+1 >= max {
				return nil, err
			}
		}
		next := map[string]any{}
		for k, v := range in {
			next[k] = v
		}
		next["__iteration"] = iteration + 1
		next["__result"] = out
		next["__retry"] = retry
		return next, nil
	})); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("result", compose.InvokableLambda(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		v, ok := in["__result"].(map[string]any)
		if !ok {
			return nil, contracts.Problem("invalid")
		}
		return v, nil
	})); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "run"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("result", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddBranch("run", compose.NewGraphBranch(func(ctx context.Context, out map[string]any) (string, error) {
		if s.Poll != nil {
			result, _ := out["__result"].(map[string]any)
			if result[s.Poll.DoneField] != true {
				return "run", nil
			}
		}
		if out["__retry"] == true {
			return "run", nil
		}
		return "result", nil
	}, map[string]bool{"run": true, "result": true})); err != nil {
		return nil, err
	}
	return g, nil
}

package orchestration

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/compose"
	"strings"
)

// compensate builds another Eino Workflow in reverse dependency order. This is
// a saga, not transaction rollback: every undo is a separately authorized action.
func (e *Engine) compensate(ctx context.Context, d Definition) (bool, error) {
	state, err := current(ctx)
	if err != nil {
		return false, err
	}
	byID := map[string]Step{}
	for _, s := range d.Steps {
		byID[s.ID] = s
	}
	ordered := []Step{}
	seen := map[string]bool{}
	var visit func(Step)
	visit = func(s Step) {
		if seen[s.ID] {
			return
		}
		seen[s.ID] = true
		for _, dep := range s.DependsOn {
			visit(byID[dep])
		}
		for _, m := range s.Input {
			if m.From != "input" {
				visit(byID[m.From])
			}
		}
		ordered = append(ordered, s)
	}
	for _, s := range d.Steps {
		visit(s)
	}
	store := checkpointStore{core: e.Core, q: dbgen.New(e.Core.Pool)}
	compensation := Definition{Name: "compensate", Version: d.Version, Input: &contracts.Schema{Type: "object", AdditionalProperties: true}}
	input := map[string]any{}
	previous := ""
	for i := len(ordered) - 1; i >= 0; i-- {
		s := ordered[i]
		if s.Agent != nil {
			for _, name := range s.Agent.Tools {
				tool, version, _ := strings.Cut(name, "@")
				entry, _ := e.registry.Resolve(tool, version)
				if !entry.Definition.ReadOnly {
					return false, contracts.Problem("manual_recovery_required")
				}
			}
			continue
		}
		if s.Poll != nil {
			continue
		}
		entry, _ := e.registry.Resolve(s.Capability, s.Version)
		if entry.Definition.ReadOnly {
			continue
		}
		raw, ok, err := store.Get(ctx, "node:"+s.ID+":0")
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		var saved receipt
		if json.Unmarshal(raw, &saved) != nil {
			return false, contracts.Problem("invalid")
		}
		if saved.Status != "succeeded" {
			continue
		}
		if s.Compensation == nil {
			return false, contracts.Problem("manual_recovery_required")
		}
		id := "comp_" + contracts.Digest(contracts.Input{"step": s.ID})[:16]
		c := Step{ID: id, Capability: s.Compensation.Capability, Version: s.Compensation.Version, Constants: s.Compensation.Constants, Input: map[string]Mapping{}}
		if previous != "" {
			c.DependsOn = []string{previous}
		}
		for field, m := range s.Compensation.Input {
			source := state.Manifest.Input
			if m.From == s.ID {
				source = saved.Data
			}
			value, ok := source[m.Field]
			if !ok {
				return false, contracts.Problem("invalid")
			}
			key := id + "_" + field
			input[key] = value
			c.Input[field] = Mapping{From: "input", Field: key}
		}
		compensation.Steps = append(compensation.Steps, c)
		previous = id
	}
	if len(compensation.Steps) == 0 {
		return false, nil
	}
	r, err := e.compile(ctx, compensation)
	if err != nil {
		return false, err
	}
	_, err = r.Invoke(ctx, map[string]any{"input": input}, compose.WithCheckPointID("compensation"))
	if _, ok := compose.ExtractInterruptInfo(err); ok {
		return true, nil
	}
	return false, err
}

package orchestration

import (
	"context"
	"errors"
	"sort"
	"strings"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/workflow/spec"
	"github.com/jackc/pgx/v5"
)

// PlanReferences returns only compatible, currently authorized deployment
// references. It never opens a business record or calls a model.
func (e *Engine) PlanReferences(ctx context.Context, q *dbgen.Queries, target contracts.Definition, mode, query string) ([]map[string]any, error) {
	refs := []map[string]any{}
	query = strings.ToLower(strings.TrimSpace(query))
	add := func(ref, name, version, hash string) {
		if query == "" || strings.Contains(strings.ToLower(ref), query) {
			refs = append(refs, map[string]any{"ref": ref, "name": name, "version": version, "hash": hash, "mode": mode})
		}
	}
	skip := func(err error) bool {
		switch contracts.Code(err) {
		case "invalid", "forbidden", "unavailable", "not_found":
			return true
		}
		return errors.Is(err, pgx.ErrNoRows)
	}
	switch mode {
	case "workflow":
		for ref, d := range e.definitions {
			if e.graphWrappers[ref] {
				continue
			}
			hash, err := e.ValidatePlanWithQueries(ctx, q, target, ref)
			if err != nil {
				if skip(err) {
					continue
				}
				return nil, err
			}
			add(ref, d.Name, d.Version, hash)
		}
	case "graph":
		for _, g := range e.graphs {
			hash, err := e.ValidateGraphPlan(ctx, q, target, g.Key())
			if err != nil {
				if skip(err) {
					continue
				}
				return nil, err
			}
			add(g.Key(), g.Name, g.Version, hash)
		}
	default:
		return nil, contracts.Problem("invalid")
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i]["ref"].(string) < refs[j]["ref"].(string) })
	if len(refs) > 50 {
		refs = refs[:50]
	}
	return refs, nil
}
func (e *Engine) InspectPlanReference(ctx context.Context, q *dbgen.Queries, target contracts.Definition, mode, ref, expectedHash string) (map[string]any, error) {
	name, version, _ := strings.Cut(ref, "@")
	var hash string
	var err error
	result := map[string]any{}
	switch mode {
	case "workflow":
		hash, err = e.ValidatePlanWithQueries(ctx, q, target, ref)
	case "graph":
		hash, err = e.ValidateGraphPlan(ctx, q, target, ref)
		if err == nil {
			for _, g := range e.graphs {
				if g.Key() == ref {
					wrapper := spec.GraphWorkflow(g)
					name, version = wrapper.Name, wrapper.Version
					result["budget"] = map[string]any{"modelCalls": g.MaxModelCalls, "toolCalls": g.MaxToolCalls, "outputTokens": g.MaxOutputTokens, "contextBytes": g.MaxContextBytes, "timeoutMillis": g.TimeoutMillis}
					break
				}
			}
		}
	default:
		return nil, contracts.Problem("invalid")
	}
	if err != nil {
		return nil, err
	}
	if hash != expectedHash {
		return nil, contracts.Problem("invalid")
	}
	diagram, err := e.Diagram(ctx, name, version)
	if err != nil {
		return nil, err
	}
	result["diagram"] = diagram
	result["referenceHash"] = hash
	return result, nil
}

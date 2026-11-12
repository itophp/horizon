package registry

import (
	"context"
	"testing"

	"horizon/foundation/kernel/capability/contracts"
)

func TestExecutionPlansPreserveNativeHandlersAndVersions(t *testing.T) {
	calls := 0
	h := contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) {
		calls++
		return contracts.Output{Data: map[string]any{"native": true}}, nil
	}}
	entries := []Entry{
		{Definition: definition("example.record.list", "1.0.0"), Handler: h},
		{Definition: definition("example.record.list", "2.0.0"), Handler: h},
	}
	r := New(entries)
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	old, pinned, ok := r.ResolveExecution("example.record.list", "1.0.0")
	if !ok || pinned.ScopeType != "platform" || pinned.Plan.Mode != "direct" || pinned.Plan.Version != 1 || pinned.Plan.Status != "published" {
		t.Fatalf("missing default plan: %+v", pinned)
	}
	if pinned.Plan.ImplementationRef != old.Definition.Key() {
		t.Fatal("native handler reference changed")
	}
	result, err := old.Handler.Execute(context.Background(), nil)
	if err != nil || calls != 1 || result.Data.(map[string]any)["native"] != true {
		t.Fatal("native behavior changed")
	}
	_, latest, ok := r.ResolveExecution("example.record.list", "")
	if !ok || latest.Plan.CapabilityVersion != "2.0.0" || latest.Plan.PlanID == pinned.Plan.PlanID {
		t.Fatal("contract versions must have independent plans")
	}
	// Returned metadata is a value; callers cannot mutate the catalog.
	latest.Plan.Mode = "graph"
	_, again, _ := r.ResolveExecution("example.record.list", "")
	if again.Plan.Mode != "direct" {
		t.Fatal("mutable execution plan escaped")
	}
	restarted := New(entries)
	_, restored, _ := restarted.ResolveExecution("example.record.list", "1.0.0")
	if restored != pinned {
		t.Fatal("deployment restart changed plan identity")
	}
	for _, query := range [][2]string{{"missing", ""}, {"example.record.list", "9.0.0"}} {
		if _, _, ok := r.ResolveExecution(query[0], query[1]); ok {
			t.Fatal("unknown execution target accepted")
		}
	}
	var absent *Registry
	if _, _, ok := absent.ResolveExecution("example.record.list", ""); ok {
		t.Fatal("nil registry accepted")
	}
}

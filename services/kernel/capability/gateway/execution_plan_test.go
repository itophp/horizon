package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"baseline/services/api/kernel/capability/contracts"
)

func TestExecutionPlanAuditAndContext(t *testing.T) {
	binding := contracts.DefaultExecutionBinding(contracts.Definition{Name: "example.record.list", Version: "1.0.0"})
	raw, err := json.Marshal(storedResult{ExecutionBinding: &binding, Data: "original", EventIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	var restored storedResult
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ExecutionBinding == nil || *restored.ExecutionBinding != binding || restored.Data != "original" {
		t.Fatal("execution identity lost")
	}
	var legacy storedResult
	if err = json.Unmarshal([]byte(`{"data":"legacy","eventIds":[]}`), &legacy); err != nil || legacy.ExecutionBinding != nil || legacy.Data != "legacy" {
		t.Fatal("legacy replay format broken")
	}
	ctx := withInvocation(context.Background(), InvocationContext{ExecutionBinding: binding, InTransaction: true})
	meta, ok := CurrentInvocation(ctx)
	if !ok || meta.ExecutionBinding != binding {
		t.Fatal("native execution cannot inspect its pinned plan")
	}
	if err = inheritRequest(ctx, &Request{}); err == nil {
		t.Fatal("plan context bypassed transaction nesting guard")
	}
}

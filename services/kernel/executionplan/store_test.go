package executionplan

import (
	"encoding/json"
	"testing"

	"baseline/services/api/kernel/capability/contracts"
)

func TestDecodePinnedPlanFailsClosed(t *testing.T) {
	d := contracts.Definition{Name: "example.record.list", Version: "1.0.0", Input: contracts.Object(nil)}
	original := contracts.DefaultExecutionBinding(d)
	raw, _ := json.Marshal(original)
	got, err := Decode(raw, d)
	if err != nil || got != original {
		t.Fatal(got, err)
	}
	for _, mutate := range []func(*contracts.ExecutionBinding){
		func(b *contracts.ExecutionBinding) { b.Plan.Mode = "workflow" },
		func(b *contracts.ExecutionBinding) { b.Plan.CapabilityID = "other" },
		func(b *contracts.ExecutionBinding) { b.Plan.CapabilityVersion = "2.0.0" },
		func(b *contracts.ExecutionBinding) { b.Plan.ImplementationRef = "other" },
		func(b *contracts.ExecutionBinding) { b.Plan.Status = "draft" },
		func(b *contracts.ExecutionBinding) { b.Plan.Version = 0 },
	} {
		b := original
		mutate(&b)
		raw, _ := json.Marshal(b)
		if _, err = Decode(raw, d); err == nil {
			t.Fatal("invalid pinned plan accepted", b)
		}
	}
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`null`)} {
		if _, err = Decode(raw, d); err == nil {
			t.Fatal("missing plan accepted")
		}
	}
	hash := ContractHash(d)
	d.Input = contracts.Object(map[string]*contracts.Schema{"required": {Type: "string"}}, "required")
	if hash == ContractHash(d) {
		t.Fatal("schema change did not invalidate plan")
	}
}

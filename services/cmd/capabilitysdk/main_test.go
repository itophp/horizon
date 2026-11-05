package main

import (
	"strings"
	"testing"

	"baseline/services/api/kernel/capability/contracts"
)

func TestSchemaTypesUseRequiredFieldsAndEnums(t *testing.T) {
	s := contracts.Object(map[string]*contracts.Schema{"partition": {Type: "string", Enum: []string{"agent", "activity"}}, "limit": {Type: "integer"}}, "partition")
	got := tsType(s)
	if !strings.Contains(got, `"partition": "agent" | "activity"`) || !strings.Contains(got, `"limit"?: number`) {
		t.Fatalf("incorrect schema: %s", got)
	}
	if nameOf("memory.item.create") != "MemoryItemCreate" {
		t.Fatal("incorrect capability name")
	}
}

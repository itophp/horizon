package executionplans

import (
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/metadata"
	"testing"
)

func TestExecutionPlanEntitySchemaAndEnumDrift(t *testing.T) {
	tables, err := metadata.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range metadata.Validate(tables, Entities()) {
		if issue.Severity == metadata.SeverityError {
			t.Fatal(issue)
		}
	}
	entity := execution_planEntity()
	for i := range entity.Attributes {
		if entity.Attributes[i].Name == "status" {
			entity.Attributes[i].Enum = append(entity.Attributes[i].Enum, "invalid-state")
		}
	}
	found := false
	for _, issue := range metadata.Validate(tables, []contracts.Entity{entity}) {
		if issue.Code == "E4" && issue.Severity == metadata.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("entity enum drift escaped schema validation")
	}
}

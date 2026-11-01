package example

import "baseline/services/api/internal/platform/capability/contracts"

// exampleRecordEntity mirrors example_records. The state set and the transitions
// are declared because they cannot be derived from the table alone: the archive
// capability accepts any non-archived state (WHERE status <> 'archived'), and
// nothing currently produces 'active'.
func exampleRecordEntity() contracts.Entity {
	maxTitle := 200
	return contracts.Entity{
		Name:        "example_record",
		Domain:      "example",
		Table:       "example_records",
		Description: "Reference record used to prove the capability loop end to end.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "Primary key."},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxTitle, Description: "Short human readable title."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"draft", "active", "archived"}, Description: "Lifecycle state."},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true,
				Description: "Account that created the record, injected from the principal."},
		},
		States: []contracts.State{
			{Name: "draft"},
			{Name: "active"},
			{Name: "archived", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{To: "draft", Capability: "example.record.create"},
			{From: []string{"draft", "active"}, To: "archived", Capability: "example.record.archive"},
		},
	}
}

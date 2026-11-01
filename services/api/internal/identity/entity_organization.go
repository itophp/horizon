package identity

import "baseline/services/api/internal/platform/capability/contracts"

// organizationEntity mirrors iam_organizations. Status is a business enum rather
// than a state machine: one update call can set either value.
func organizationEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "organization",
		Domain:      "identity",
		Table:       "iam_organizations",
		Description: "An enterprise or company node of the organization tree.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "kind", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"enterprise", "company"}},
			{Name: "parentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "status", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"active", "inactive"}},
			{Name: "description", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "parent", Target: "organization", Kind: "one", Via: "parent_id", Required: false},
		},
	}
}

package identity

import "baseline/services/api/internal/platform/capability/contracts"

// departmentEntity mirrors iam_departments.
func departmentEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "department",
		Domain:      "identity",
		Table:       "iam_departments",
		Description: "A node of a company's department tree; accounts are placed in one.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "organizationId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "parentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "manager", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "status", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"active", "inactive"}},
			{Name: "description", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "organization", Target: "organization", Kind: "one", Via: "organization_id", Required: true},
			{Name: "parent", Target: "department", Kind: "one", Via: "parent_id", Required: false},
		},
	}
}

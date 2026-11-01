package identity

import "baseline/services/api/internal/platform/capability/contracts"

// roleEntity mirrors iam_roles. Grants are a JSON array, not a child table.
func roleEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "role",
		Domain:      "identity",
		Table:       "iam_roles",
		Description: "A named bundle of permission grants assigned to accounts.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "description", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "status", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"active", "inactive"}},
			{Name: "grants", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "array", Required: true,
				Description: "Permission, effect and scope per entry."},
		},
	}
}

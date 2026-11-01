package identity

import "baseline/services/api/internal/platform/capability/contracts"

// accountEntity mirrors iam_accounts.
func accountEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "account",
		Domain:      "identity",
		Table:       "iam_accounts",
		Description: "A person who can sign in, placed in one primary department.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "username", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "Unique sign-in name."},
			{Name: "fullName", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "email", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "passwordHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "argon2id hash; never projected to a DTO."},
			// Lifecycle flag, not a state machine: identity.account.update may set
			// either value in one call, so there is no legal transition graph to
			// declare yet.
			{Name: "status", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"active", "inactive"}},
			{Name: "organizationId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "overrides", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "array", Required: true,
				Description: "Per-account grants that replace the role grant for the same permission."},
			{Name: "isAdmin", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true,
				Description: "Built-in administrator; bypasses grant evaluation."},
			{Name: "lastLogin", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "organization", Target: "organization", Kind: "one", Via: "organization_id", Required: true},
			{Name: "department", Target: "department", Kind: "one", Via: "department_id", Required: true},
		},
	}
}

package drive

import "baseline/services/api/kernel/capability/contracts"

func memberEntity() contracts.Entity {
	return contracts.Entity{
		Name: "drive_member", Domain: "drive", Table: "drive_members", Key: "id",
		Description: "An account or department grant for a shared drive space.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "spaceId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "accountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "role", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"admin", "editor", "viewer"}},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "space", Target: "drive_space", Kind: "one", Via: "space_id", Required: true},
			{Name: "account", Target: "account", Kind: "one", Via: "account_id"},
			{Name: "department", Target: "department", Kind: "one", Via: "department_id"},
		},
	}
}

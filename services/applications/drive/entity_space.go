package drive

import "baseline/services/api/kernel/capability/contracts"

func spaceEntity() contracts.Entity {
	return contracts.Entity{
		Name: "drive_space", Domain: "drive", Table: "drive_spaces", Key: "id",
		Description: "A personal or shared file space.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "kind", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true, Enum: []string{"personal", "shared"}},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
			{Name: "members", Target: "drive_member", Kind: "many", Via: "drive_members"},
		},
	}
}

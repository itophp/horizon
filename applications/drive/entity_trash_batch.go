package drive

import "horizon/foundation/kernel/capability/contracts"

func trashBatchEntity() contracts.Entity {
	return contracts.Entity{
		Name: "drive_trash_batch", Domain: "drive", Table: "drive_trash_batches", Key: "id",
		Description: "One deletion operation retained for recovery.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "spaceId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "deletedBy", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "deletedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "expiresAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "purgingAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "space", Target: "drive_space", Kind: "one", Via: "space_id", Required: true},
			{Name: "actor", Target: "account", Kind: "one", Via: "deleted_by", Required: true},
			{Name: "folders", Target: "drive_folder", Kind: "many", Via: "drive_folders"},
		},
	}
}

package drive

import "baseline/services/api/kernel/capability/contracts"

func folderEntity() contracts.Entity {
	return contracts.Entity{
		Name: "drive_folder", Domain: "drive", Table: "drive_folders", Key: "id",
		Description: "A folder inside a single personal or shared space.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "spaceId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "parentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "deletedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "trashBatchId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "trashRoot", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "space", Target: "drive_space", Kind: "one", Via: "space_id", Required: true},
			{Name: "parent", Target: "drive_folder", Kind: "one", Via: "parent_id"},
			{Name: "creator", Target: "account", Kind: "one", Via: "created_by", Required: true},
			{Name: "trashBatch", Target: "drive_trash_batch", Kind: "one", Via: "trash_batch_id"},
		},
	}
}

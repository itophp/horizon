package drive

import "horizon/foundation/kernel/capability/contracts"

func fileEntity() contracts.Entity {
	return contracts.Entity{Name: "drive_file", Domain: "drive", Table: "drive_files", Key: "id",
		Description: "A file in a drive space, bound to the shared file store.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "spaceId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "folderId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "fileId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "createdBy", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "deletedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "trashBatchId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "trashRoot", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "space", Target: "drive_space", Kind: "one", Via: "space_id", Required: true},
			{Name: "folder", Target: "drive_folder", Kind: "one", Via: "folder_id"},
			{Name: "file", Target: "file", Kind: "one", Via: "file_id", Required: true},
			{Name: "creator", Target: "account", Kind: "one", Via: "created_by", Required: true},
			{Name: "trashBatch", Target: "drive_trash_batch", Kind: "one", Via: "trash_batch_id"},
		},
	}
}

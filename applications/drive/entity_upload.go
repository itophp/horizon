package drive

import "horizon/foundation/kernel/capability/contracts"

func uploadEntity() contracts.Entity {
	return contracts.Entity{Name: "drive_upload", Domain: "drive", Table: "drive_uploads", Key: "id",
		Description: "An upload session with preallocated storage keys and a fixed storage profile.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "spaceId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "folderId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "storageProfileId", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
			{Name: "objectKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "size", Kind: contracts.AttributeBusiness, Type: contracts.TypeInteger, Required: true},
			{Name: "state", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"open", "completed", "cancelled", "cleaned"}},
			{Name: "fileId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "expiresAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "space", Target: "drive_space", Kind: "one", Via: "space_id", Required: true},
			{Name: "folder", Target: "drive_folder", Kind: "one", Via: "folder_id"},
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
			{Name: "file", Target: "file", Kind: "one", Via: "file_id"},
			{Name: "chunks", Target: "drive_upload_chunk", Kind: "many", Via: "drive_upload_chunks"},
		},
		States: []contracts.State{{Name: "open"}, {Name: "completed"}, {Name: "cancelled"}, {Name: "cleaned", Terminal: true}},
		Transitions: []contracts.Transition{{To: "open", Capability: "drive.upload.start"}, {From: []string{"open"}, To: "completed", Capability: "drive.upload.complete"},
			{From: []string{"open"}, To: "cancelled", Capability: "drive.upload.cancel"},
			{From: []string{"open"}, To: "cancelled", Capability: "drive.cleanup.batch"},
			{From: []string{"completed", "cancelled", "open"}, To: "cleaned", Capability: "drive.cleanup.upload"}},
	}
}

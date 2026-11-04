package drive

import "baseline/services/api/kernel/capability/contracts"

func uploadChunkEntity() contracts.Entity {
	return contracts.Entity{Name: "drive_upload_chunk", Domain: "drive", Table: "drive_upload_chunks", Key: "id",
		Description: "A preallocated chunk slot and its verified digest.",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "uploadId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "chunkIndex", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "storageKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
			{Name: "expectedSize", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "sha256", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Nullable: true},
			{Name: "uploadedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{{Name: "upload", Target: "drive_upload", Kind: "one", Via: "upload_id", Required: true}},
	}
}

package files

import "baseline/services/api/internal/platform/capability/contracts"

func fileEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "file",
		Domain:      "files",
		Table:       "gov_files",
		Description: "A stored file with an immutable storage profile and an owner.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Account that uploaded the file."},
			{Name: "organizationId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "storageProfileId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Immutable storage profile; the file must never move between profiles."},
			{Name: "storageKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true,
				Description: "Path inside the profile; internal only."},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "mime", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "Verified from the bytes, not from the client header."},
			{Name: "size", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "purpose", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"logo", "attachment"}},
			{Name: "sha256", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "deletedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "purgedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
			{Name: "organization", Target: "organization", Kind: "one", Via: "organization_id", Required: true},
			{Name: "department", Target: "department", Kind: "one", Via: "department_id", Required: true},
		},
	}
}

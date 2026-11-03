package projects

import "baseline/services/api/internal/platform/capability/contracts"

// projectMemberEntity is the visibility boundary: a project without a member row
// for the caller does not exist. It has no state machine and no role column —
// the owner is projects.owner_id.
func projectMemberEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "project_member",
		Domain:      "projects",
		Table:       "project_members",
		Description: "Membership that grants read and write access to one project.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "projectId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "accountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Member account."},
			{Name: "joinedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "project", Target: "project", Kind: "one", Via: "project_id", Required: true},
			{Name: "account", Target: "account", Kind: "one", Via: "account_id", Required: true},
		},
	}
}

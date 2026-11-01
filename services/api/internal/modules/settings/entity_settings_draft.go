package settings

import "baseline/services/api/internal/platform/capability/contracts"

// settingsDraftEntity mirrors gov_drafts. The table is shared with dictionary
// drafts, which are filtered by domain; declaring one entity keeps the model
// unambiguous (see accepted gaps in cmd/entitycheck).
func settingsDraftEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "settings_draft",
		Domain:      "settings",
		Table:       "gov_drafts",
		Description: "A reviewed but unpublished change set for one settings resource.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "domain", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"settings", "dictionary", "override"}},
			{Name: "resourceKey", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "scopeKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "entityId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "baseRevision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true,
				Description: "The staged change set."},
			{Name: "reason", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "sensitive", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true,
				Description: "Publishing a sensitive draft needs a reauthentication proof."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"draft", "published"}},
			{Name: "actorId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "actor", Target: "account", Kind: "one", Via: "actor_id", Required: true},
		},
		States: []contracts.State{
			{Name: "draft"},
			{Name: "published", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{From: []string{"draft"}, To: "published", Capability: "settings.draft.publish"},
		},
	}
}

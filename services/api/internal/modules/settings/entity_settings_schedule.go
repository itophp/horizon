package settings

import "baseline/services/api/internal/platform/capability/contracts"

// settingsScheduleEntity mirrors gov_settings_schedules. Publication runs in the
// background worker, so most transitions have no capability.
func settingsScheduleEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "settings_schedule",
		Domain:      "settings",
		Table:       "gov_settings_schedules",
		Description: "A publication scheduled for a future instant, executed by the worker.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "draftId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "actorId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "baseRevision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "draftHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Description: "Binds the schedule to the exact draft content."},
			{Name: "sensitiveConfirmed", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
			{Name: "scheduledAt", Kind: contracts.AttributeBusiness, Type: contracts.TypeTimestamp, Required: true},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"pending", "published", "cancelled", "failed"}},
			{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "versionId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
			{Name: "finishedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "draft", Target: "settings_draft", Kind: "one", Via: "draft_id", Required: true},
			{Name: "actor", Target: "account", Kind: "one", Via: "actor_id", Required: true},
		},
		States: []contracts.State{
			{Name: "pending"},
			{Name: "published", Terminal: true},
			{Name: "cancelled", Terminal: true},
			{Name: "failed", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{From: []string{"pending"}, To: "published"},
			{From: []string{"pending"}, To: "cancelled", Capability: "settings.schedule.cancel"},
			{From: []string{"pending"}, To: "failed"},
		},
	}
}

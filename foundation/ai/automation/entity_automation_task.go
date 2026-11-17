package automation

import "horizon/foundation/kernel/capability/contracts"

// automationTaskEntity mirrors automation_tasks. A task belongs to exactly one
// account; the owner and department are re-resolved from the database for every
// occurrence, so an ownership row is authority input, never a permission cache.
func automationTaskEntity() contracts.Entity {
	maxName, maxDescription := maxNameLength, maxDescriptionLength
	maxTimezone := 100
	return contracts.Entity{
		Name:        "automation_task",
		Domain:      "automation",
		Table:       "automation_tasks",
		Description: "A user owned instruction the workspace agent executes on a time or interval schedule.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Account whose current authority each occurrence runs under."},
			{Name: "departmentId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "Department captured when the task was created; re-resolved on every run."},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxName},
			{Name: "description", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxDescription, Sensitive: true,
				Description: "Markdown instruction used as the agent question; bounded by the AI run question limit."},
			{Name: "scheduleKind", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: ScheduleKinds},
			{Name: "runAt", Kind: contracts.AttributeBusiness, Type: contracts.TypeTimestamp, Required: true,
				Description: "First occurrence anchor; recurring kinds reuse its local wall clock components."},
			{Name: "timezone", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxTimezone},
			{Name: "enabled", Kind: contracts.AttributeBusiness, Type: contracts.TypeBoolean, Required: true,
				Description: "Disabled tasks are never scheduled; a manual run is still allowed."},
			{Name: "scheduleDay", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Nullable: true,
				Description: "Intended day-of-month for monthly series; anchors are clamped, this value is not."},
			{Name: "nextRunAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true,
				Description: "Computed by the registry worker, never accepted from a caller."},
			{Name: "lastRunAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "failureCount", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "Consecutive failed occurrences; reset by a completed run."},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
		},
	}
}

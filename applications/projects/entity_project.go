package projects

import "horizon/foundation/kernel/capability/contracts"

// projectEntity mirrors projects. The lifecycle is declared because the status
// column only changes through project.status.change, and archiving is terminal.
func projectEntity() contracts.Entity {
	maxCode, maxName := 32, 120
	return contracts.Entity{
		Name:        "project",
		Domain:      "projects",
		Table:       "projects",
		Description: "A collaboration project with an owner, members, plans and tasks.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "Primary key."},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "code", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxCode, Description: "Stable business code, unique per tenant."},
			{Name: "name", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxName, Description: "Project name."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"active", "paused", "completed", "archived"}, Description: "Lifecycle state."},
			{Name: "priority", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"low", "normal", "high", "urgent"}, Description: "Priority."},
			{Name: "dueDate", Kind: contracts.AttributeBusiness, Type: contracts.TypeDate,
				Nullable: true, Description: "Planned end date."},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Account that owns the project; always has a member row."},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
			{Name: "members", Target: "project_member", Kind: "many", Via: "project_members"},
			{Name: "plans", Target: "project_plan", Kind: "many", Via: "project_plans"},
			{Name: "tasks", Target: "project_task", Kind: "many", Via: "project_tasks"},
		},
		States: []contracts.State{
			{Name: "active"},
			{Name: "paused"},
			{Name: "completed"},
			{Name: "archived", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{To: "active", Capability: "project.create"},
			{To: "active", Capability: "project.import.commit"},
			{From: []string{"active"}, To: "paused", Capability: "project.status.change"},
			{From: []string{"paused"}, To: "active", Capability: "project.status.change"},
			{From: []string{"active"}, To: "completed", Capability: "project.status.change"},
			{From: []string{"paused"}, To: "completed", Capability: "project.status.change"},
			{From: []string{"active", "paused", "completed"}, To: "archived", Capability: "project.status.change"},
		},
	}
}

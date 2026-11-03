package projects

import "baseline/services/api/kernel/capability/contracts"

// projectPlanEntity mirrors project_plans. Content is Markdown and stays out of
// the audit projection, the same way article.body does.
func projectPlanEntity() contracts.Entity {
	maxTitle, maxContent := 200, 20000
	return contracts.Entity{
		Name:        "project_plan",
		Domain:      "projects",
		Table:       "project_plans",
		Description: "A milestone, phase or deliverable inside one project.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "projectId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "planType", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"milestone", "phase", "deliverable"}, Description: "Plan kind."},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxTitle, Description: "Plan title."},
			{Name: "content", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxContent, Sensitive: true, Description: "Markdown body."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"pending", "in_progress", "done", "cancelled"}, Description: "Lifecycle state."},
			{Name: "priority", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"low", "normal", "high", "urgent"}},
			{Name: "dueDate", Kind: contracts.AttributeBusiness, Type: contracts.TypeDate, Nullable: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Account responsible for the plan; always a project member."},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "project", Target: "project", Kind: "one", Via: "project_id", Required: true},
			{Name: "owner", Target: "account", Kind: "one", Via: "owner_id", Required: true},
			{Name: "tasks", Target: "project_task", Kind: "many", Via: "project_tasks"},
		},
		States: []contracts.State{
			{Name: "pending"},
			{Name: "in_progress"},
			{Name: "done"},
			{Name: "cancelled", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{To: "pending", Capability: "project.plan.create"},
			{From: []string{"pending"}, To: "in_progress", Capability: "project.plan.status.change"},
			{From: []string{"in_progress"}, To: "done", Capability: "project.plan.status.change"},
			{From: []string{"done"}, To: "in_progress", Capability: "project.plan.status.change"},
			{From: []string{"pending", "in_progress"}, To: "cancelled", Capability: "project.plan.status.change"},
		},
	}
}

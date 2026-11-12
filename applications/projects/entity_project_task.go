package projects

import "horizon/foundation/kernel/capability/contracts"

// projectTaskEntity mirrors project_tasks. plan_id and assignee_id are nullable
// on purpose: a task may exist before it is planned or assigned.
func projectTaskEntity() contracts.Entity {
	maxTitle, maxDescription := 200, 20000
	return contracts.Entity{
		Name:        "project_task",
		Domain:      "projects",
		Table:       "project_tasks",
		Description: "An executable unit of work inside a project.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "projectId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "planId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true,
				Description: "Optional plan this task belongs to."},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxTitle},
			{Name: "description", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxDescription, Sensitive: true, Description: "Markdown description."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"todo", "in_progress", "done", "cancelled"}},
			{Name: "priority", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"low", "normal", "high", "urgent"}},
			{Name: "assigneeId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true,
				Description: "Account doing the task; nullable until assigned."},
			{Name: "dueDate", Kind: contracts.AttributeBusiness, Type: contracts.TypeDate, Nullable: true},
			{Name: "completedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true,
				Description: "Set when the task enters done, cleared when it leaves."},
			{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeReference, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "project", Target: "project", Kind: "one", Via: "project_id", Required: true},
			{Name: "plan", Target: "project_plan", Kind: "one", Via: "plan_id"},
			{Name: "assignee", Target: "account", Kind: "one", Via: "assignee_id"},
		},
		States: []contracts.State{
			{Name: "todo"},
			{Name: "in_progress"},
			{Name: "done"},
			{Name: "cancelled", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{To: "todo", Capability: "project.task.create"},
			{From: []string{"todo"}, To: "in_progress", Capability: "project.task.status.change"},
			{From: []string{"in_progress"}, To: "done", Capability: "project.task.status.change"},
			{From: []string{"done"}, To: "todo", Capability: "project.task.status.change"},
			{From: []string{"todo", "in_progress"}, To: "cancelled", Capability: "project.task.status.change"},
		},
	}
}

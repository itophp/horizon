package projects

import "horizon/foundation/kernel/capability/contracts"

// projectTaskBriefEntity stores the AI handover brief. It is a separate object
// so a model failure never rewrites a task, and one task keeps only the newest
// brief (upsert on task_id).
func projectTaskBriefEntity() contracts.Entity {
	maxSummary := 4000
	return contracts.Entity{
		Name:        "project_task_brief",
		Domain:      "projects",
		Table:       "project_task_briefs",
		Description: "AI generated handover brief for one task. Advice only.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "taskId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "runId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "AI run that produced the brief, 0 when unknown."},
			{Name: "summary", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				MaxLength: &maxSummary, Description: "Generated handover text."},
			{Name: "model", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "task", Target: "project_task", Kind: "one", Via: "task_id", Required: true},
		},
	}
}

package automation

import "horizon/foundation/kernel/capability/contracts"

// automationRunEntity mirrors automation_runs. One row is one occurrence; the
// unique key makes scheduling idempotent and the worker-driven transitions are
// declared without a capability so the state graph stays honest.
func automationRunEntity() contracts.Entity {
	maxOccurrence := 120
	return contracts.Entity{
		Name:        "automation_run",
		Domain:      "automation",
		Table:       "automation_runs",
		Description: "One execution of an automation task, joined to the AI run it launched.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "taskId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "occurrenceKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				MaxLength: &maxOccurrence, Description: "Deduplication key: the scheduled instant or the manual idempotency key."},
			{Name: "scheduledFor", Kind: contracts.AttributeBusiness, Type: contracts.TypeTimestamp, Required: true},
			{Name: "trigger", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Required: true,
				Column: "trigger_kind", Enum: []string{"schedule", "manual"}},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"queued", "running", "awaiting_approval", "completed", "failed", "skipped", "cancelled"}},
			{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "aiRunId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Nullable: true,
				Description: "The AI run this occurrence launched; null before the worker starts it."},
			{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "startedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "finishedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "task", Target: "automation_task", Kind: "one", Via: "task_id", Required: true},
		},
		States: []contracts.State{
			{Name: "queued"},
			{Name: "running"},
			{Name: "awaiting_approval"},
			{Name: "completed", Terminal: true},
			{Name: "failed", Terminal: true},
			{Name: "skipped", Terminal: true},
			{Name: "cancelled", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{To: "queued", Capability: "automation.run.now"},
			{From: []string{"queued"}, To: "running"},
			{From: []string{"queued"}, To: "skipped"},
			{From: []string{"running"}, To: "awaiting_approval"},
			{From: []string{"running"}, To: "completed"},
			{From: []string{"running"}, To: "failed"},
			{From: []string{"running"}, To: "skipped"},
			{From: []string{"running"}, To: "cancelled"},
			{From: []string{"awaiting_approval"}, To: "completed"},
			{From: []string{"awaiting_approval"}, To: "failed"},
		},
	}
}

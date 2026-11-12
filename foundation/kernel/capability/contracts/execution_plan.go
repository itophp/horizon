package contracts

// ExecutionPlan identifies an immutable implementation of a capability version.
// CapabilityVersion is deliberately separate from the execution plan version.
// Direct plans cannot redirect to another capability's native handler.
type ExecutionPlan struct {
	GraphHash         string `json:"graphHash,omitempty"`
	WorkflowHash      string `json:"workflowHash,omitempty"`
	PlanID            string `json:"planId"`
	CapabilityID      string `json:"capabilityId"`
	CapabilityVersion string `json:"capabilityVersion"`
	Version           int    `json:"version"`
	Mode              string `json:"mode"`
	ImplementationRef string `json:"implementationRef"`
	Status            string `json:"status"`
}

// ExecutionBinding captures the platform fallback or a revision of a tenant's
// durable override. Gateway pins this value when an invocation is accepted.
type ExecutionBinding struct {
	ScopeID   string        `json:"scopeId,omitempty"`
	Revision  int64         `json:"revision"`
	BindingID string        `json:"bindingId"`
	ScopeType string        `json:"scopeType"`
	Plan      ExecutionPlan `json:"plan"`
}

// DefaultExecutionBinding is deterministic across restarts and includes the
// capability version so deploying a new contract never mutates an old plan.
func DefaultExecutionBinding(d Definition) ExecutionBinding {
	id := d.Key() + ".default"
	return ExecutionBinding{
		BindingID: id + ".platform", ScopeType: "platform",
		Plan: ExecutionPlan{PlanID: id, CapabilityID: d.Name,
			CapabilityVersion: d.Version, Version: 1, Mode: "direct",
			ImplementationRef: d.Key(), Status: "published"},
	}
}

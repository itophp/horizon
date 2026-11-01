// Package policy decides whether one capability invocation may run, and whether
// it needs a human confirmation first. It is deterministic code: the model's
// opinion never changes these rules.
package policy

import "baseline/services/api/internal/platform/capability/contracts"

// Decision is the outcome of an admission check.
type Decision struct {
	Allowed  bool
	Approval contracts.Approval
	Code     string
}

// Admit applies permission, risk and origin rules.
//
// A human clicking a button has already given an explicit instruction, so a high
// risk capability runs directly. An agent, workflow, event or external API call
// has no such implicit consent and must stop for confirmation. Critical
// capabilities are never reachable by automation, and even a human call needs a
// second approver. An already approved resumption is trusted because the
// approval store re-checked the requester's authority before calling back in.
func Admit(def contracts.Definition, caller contracts.Caller, origin contracts.Origin) Decision {
	if !caller.Allowed(def.Permission) {
		return Decision{Code: "forbidden"}
	}
	if def.Approval == contracts.ApprovalForbidden {
		return Decision{Code: "forbidden"}
	}
	if origin == contracts.OriginApproval {
		return Decision{Allowed: true, Approval: contracts.ApprovalAuto}
	}
	automated := origin == contracts.OriginCapability || origin == contracts.OriginAgent || origin == contracts.OriginWorkflow || origin == contracts.OriginEvent || origin == contracts.OriginAPI

	if def.Risk == contracts.RiskCritical {
		if automated {
			return Decision{Code: "forbidden"}
		}
		return Decision{Allowed: true, Approval: contracts.ApprovalFourEyes}
	}
	switch def.Approval {
	case contracts.ApprovalFourEyes:
		return Decision{Allowed: true, Approval: contracts.ApprovalFourEyes}
	case contracts.ApprovalConfirm:
		if automated {
			return Decision{Allowed: true, Approval: contracts.ApprovalConfirm}
		}
		return Decision{Allowed: true, Approval: contracts.ApprovalAuto}
	default:
		// Risk high defaults to a confirmation even when the definition forgot to
		// declare it; the policy table must not silently downgrade.
		if def.Risk == contracts.RiskHigh && automated {
			return Decision{Allowed: true, Approval: contracts.ApprovalConfirm}
		}
		return Decision{Allowed: true, Approval: contracts.ApprovalAuto}
	}
}

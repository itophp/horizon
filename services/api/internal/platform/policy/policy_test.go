package policy

import (
	"testing"

	"baseline/services/api/internal/platform/capability/contracts"
)

func definition(risk contracts.Risk, approval contracts.Approval) contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "test.action", Version: "1.0", Permission: "test:write",
		Risk: risk, Approval: approval, Idempotency: contracts.Idempotency{Required: true},
		SuccessEvents: []string{"test.happened"},
	}
}

func TestHumanClicksRunDirectly(t *testing.T) {
	caller := contracts.Caller{Grants: []string{"test:write"}}
	high := definition(contracts.RiskHigh, contracts.ApprovalConfirm)
	decision := Admit(high, caller, contracts.OriginHumanUI)
	if !decision.Allowed || decision.Approval != contracts.ApprovalAuto {
		t.Fatalf("a human click is the confirmation: %+v", decision)
	}
}

func TestAutomationMustConfirmHighRisk(t *testing.T) {
	caller := contracts.Caller{Grants: []string{"test:write"}}
	high := definition(contracts.RiskHigh, contracts.ApprovalConfirm)
	for _, origin := range []contracts.Origin{contracts.OriginCapability, contracts.OriginAgent, contracts.OriginWorkflow, contracts.OriginEvent, contracts.OriginAPI} {
		decision := Admit(high, caller, origin)
		if !decision.Allowed || decision.Approval != contracts.ApprovalConfirm {
			t.Fatalf("%s must confirm: %+v", origin, decision)
		}
	}
}

func TestCriticalIsClosedToAutomation(t *testing.T) {
	caller := contracts.Caller{Grants: []string{"test:write"}}
	critical := definition(contracts.RiskCritical, contracts.ApprovalFourEyes)
	if decision := Admit(critical, caller, contracts.OriginAgent); decision.Allowed {
		t.Fatal("agents must never auto-execute critical capabilities")
	}
	if decision := Admit(critical, caller, contracts.OriginEvent); decision.Allowed {
		t.Fatal("events must never auto-execute critical capabilities")
	}
	if decision := Admit(critical, caller, contracts.OriginHumanUI); !decision.Allowed || decision.Approval != contracts.ApprovalFourEyes {
		t.Fatalf("a human call needs a second approver: %+v", decision)
	}
	if decision := Admit(critical, caller, contracts.OriginApproval); !decision.Allowed || decision.Approval != contracts.ApprovalAuto {
		t.Fatalf("an approved call may resume: %+v", decision)
	}
}

func TestPermissionAndForbiddenPolicy(t *testing.T) {
	high := definition(contracts.RiskHigh, contracts.ApprovalConfirm)
	if decision := Admit(high, contracts.Caller{}, contracts.OriginHumanUI); decision.Allowed || decision.Code != "forbidden" {
		t.Fatalf("missing grant must be denied: %+v", decision)
	}
	forbidden := definition(contracts.RiskLow, contracts.ApprovalForbidden)
	if decision := Admit(forbidden, contracts.Caller{Admin: true}, contracts.OriginHumanUI); decision.Allowed {
		t.Fatal("forbidden stays closed to everyone")
	}
}

func TestHighRiskDefaultsToConfirmation(t *testing.T) {
	caller := contracts.Caller{Grants: []string{"test:write"}}
	// A definition that forgot to declare confirm must not be silently downgraded.
	high := definition(contracts.RiskHigh, contracts.ApprovalAuto)
	if decision := Admit(high, caller, contracts.OriginAgent); decision.Approval != contracts.ApprovalConfirm {
		t.Fatalf("high risk must default to confirmation: %+v", decision)
	}
}

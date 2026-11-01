// Package contracts holds the stable vocabulary of the capability platform:
// definitions, schemas, invocation results and business events. It deliberately
// depends on no storage, transport or AI package, so UI, Agent, Workflow and
// external adapters can share it without dragging in the runtime.
package contracts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Risk expresses how much damage a wrong execution can do. The policy package
// maps risk and origin to an approval requirement.
type Risk string

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

// Approval is the human gate a capability needs.
type Approval string

const (
	ApprovalAuto      Approval = "auto"
	ApprovalConfirm   Approval = "confirm"
	ApprovalFourEyes  Approval = "four-eyes"
	ApprovalForbidden Approval = "forbidden"
)

// Origin is the trusted adapter that produced a call. It is never taken from a
// client supplied field; the transport decides it per entry point.
type Origin string

const (
	OriginHumanUI    Origin = "human-ui"
	OriginAgent      Origin = "agent"
	OriginWorkflow   Origin = "workflow"
	OriginCapability Origin = "capability"
	OriginAPI        Origin = "api"
	OriginEvent      Origin = "event"
	OriginApproval   Origin = "approval"
)

// Status is the invocation state machine.
type Status string

const (
	StatusRequested        Status = "requested"
	StatusAwaitingApproval Status = "awaiting_approval"
	StatusExecuting        Status = "executing"
	StatusSucceeded        Status = "succeeded"
	StatusFailed           Status = "failed"
	StatusCancelled        Status = "cancelled"
)

// Idempotency declares whether a write capability requires a caller supplied key
// and how broad the replay window is.
type Idempotency struct {
	Required bool
	Scope    string
}

// Definition is the registered contract of one capability.
type Definition struct {
	Layer       Layer
	Kind        Kind
	Reads       []string
	Writes      []string
	Effect      Effect
	RetrySafe   bool
	Name        string
	Version     string
	Title       string
	Description string
	Domain      string
	Entity      string
	// Entities lists every business object this capability reads or writes, the
	// primary Entity first. Empty means the capability touches exactly one
	// entity, which keeps single-entity capabilities unchanged.
	Entities    []string
	Input       *Schema
	Output      *Schema
	Permission  string
	Risk        Risk
	Approval    Approval
	Idempotency Idempotency
	// SuccessEvents must be produced by the executor in Output.Events. They are
	// declared so the registry can validate and so UI/tooling can show the
	// closed loop before the capability ever runs.
	SuccessEvents []string
	Tags          []string
	Executor      string
	ReadOnly      bool
	// TransientResult prevents sensitive read results from being persisted or replayed.
	TransientResult bool
	// Loggable lists input fields allowed into the audit projection. Everything
	// else is replaced by the payload digest.
	Loggable []string
}

// Key identifies a capability version.
func (d Definition) Key() string { return d.Name + "@" + d.Version }

// ToolName is the model facing name. Chat completion tool names only accept word
// characters, so the dotted capability name is flattened.
func (d Definition) ToolName() string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, d.Name)
	name = strings.Trim(name, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// Input is a decoded argument object.
type Input map[string]any

// Event is a business fact produced by a successful capability.
type Event struct {
	Type          string
	AggregateType string
	AggregateID   string
	Payload       map[string]any
}

// Output is what an executor returns on success.
type Output struct {
	Data    any
	Summary string
	Events  []Event
}

// Result is the uniform invocation result returned to every caller.
type Result struct {
	InvocationID string   `json:"invocationId"`
	Status       Status   `json:"status"`
	Data         any      `json:"data,omitempty"`
	EventIDs     []string `json:"eventIds"`
	ApprovalID   int64    `json:"approvalId,omitempty"`
	ErrorCode    string   `json:"errorCode,omitempty"`
	TraceID      string   `json:"traceId"`
	Replayed     bool     `json:"replayed,omitempty"`
}

// Executor runs a capability outside a caller supplied transaction. Existing
// domain services own their transactions this way.
type Executor func(ctx context.Context, in Input) (Output, error)

// TxExecutor runs inside the gateway transaction, which lets the executor write
// business data and outbox events in one commit. New modules should prefer it.
type TxExecutor func(ctx context.Context, tx pgx.Tx, in Input) (Output, error)

// Handler binds one definition to its implementation. Exactly one field is set.
type Handler struct {
	Execute   Executor
	ExecuteTx TxExecutor
}

// Caller is the resolved subject of a call. It is built from a trusted
// principal, never from request bodies.
type Caller struct {
	TenantID     string
	AccountID    int64
	DepartmentID int64
	Admin        bool
	Grants       []string
}

// Allowed reports whether the caller holds the permission a capability needs.
func (c Caller) Allowed(permission string) bool {
	if permission == "" {
		return true
	}
	if c.Admin {
		return true
	}
	for _, granted := range c.Grants {
		if granted == permission {
			return true
		}
	}
	return false
}

// Redact projects arguments for the audit trail.
func (d Definition) Redact(in Input) map[string]any {
	out := map[string]any{}
	for _, key := range d.Loggable {
		value, ok := in[key]
		if !ok {
			continue
		}
		if text, isText := value.(string); isText {
			if len([]rune(text)) > 200 {
				text = string([]rune(text)[:200])
			}
			out[key] = text
			continue
		}
		out[key] = value
	}
	return out
}

// Digest binds idempotency and approvals to the exact payload.
func Digest(in Input) string {
	raw, err := json.Marshal(in)
	if err != nil {
		raw = []byte("{}")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Validate checks the declared preconditions of a definition.
//
// The error is a stable code because callers map it onto a transport response.
// Tooling that has to tell a human what is wrong uses Problems instead.
func (d Definition) Validate() error {
	if len(d.Problems()) > 0 {
		return Problem("invalid")
	}
	return nil
}

// Problems lists every declaration mistake with a reason that names the field,
// so a failed startup or a failed check says which capability to fix.
func (d Definition) Problems() []string {
	var out []string
	out = append(out, d.taxonomyProblems()...)
	if d.TransientResult && !d.ReadOnly {
		out = append(out, "transient results are read-only")
	}
	if d.Name == "" || d.Version == "" {
		out = append(out, "name and version are required")
	}
	if d.Input == nil || d.Input.Type != "object" {
		out = append(out, "input must be an object schema")
	}
	if d.Risk != RiskLow && d.Risk != RiskMedium && d.Risk != RiskHigh && d.Risk != RiskCritical {
		out = append(out, "risk must be low, medium, high or critical")
	}
	if d.Approval != ApprovalAuto && d.Approval != ApprovalConfirm && d.Approval != ApprovalFourEyes && d.Approval != ApprovalForbidden {
		out = append(out, "approval must be auto, confirm, four-eyes or forbidden")
	}
	if !d.ReadOnly && !d.Idempotency.Required {
		// A write that cannot be replayed safely must not be exposed as a tool.
		out = append(out, "write capabilities require an idempotency key")
	}
	if len(d.SuccessEvents) == 0 && !d.ReadOnly {
		out = append(out, "write capabilities must declare at least one success event")
	}
	if len(d.Entities) > 0 && !containsName(d.Entities, d.Entity) {
		// The primary entity must be part of the declared set, otherwise the
		// permission projection and the audit trail disagree about the object.
		out = append(out, "primary entity "+d.Entity+" is not listed in Entities")
	}
	return out
}

// EntityNames returns every entity the capability touches.
func (d Definition) EntityNames() []string {
	if len(d.Entities) == 0 {
		return []string{d.Entity}
	}
	return d.Entities
}

func containsName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// SortedDefinitions returns a stable copy ordered by name and version.
func SortedDefinitions(definitions []Definition) []Definition {
	out := append([]Definition{}, definitions...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Version < out[j].Version
		}
		return out[i].Name < out[j].Name
	})
	return out
}

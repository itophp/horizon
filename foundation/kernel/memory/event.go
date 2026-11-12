// Package memory defines the canonical, privacy-preserving collection boundary.
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const EnvelopeVersion = "1.0"

var ErrInvalidEvent = errors.New("invalid memory event")

// Envelope contains metadata only. Payload fields must be explicitly registered
// by the producer; request bodies, responses and arbitrary errors are not accepted.
type Envelope struct {
	EventID             string            `json:"eventId"`
	EventType           string            `json:"eventType"`
	EventVersion        string            `json:"eventVersion"`
	OccurredAt          time.Time         `json:"occurredAt"`
	IngestedAt          time.Time         `json:"ingestedAt"`
	TenantID            string            `json:"tenantId"`
	OrganizationID      string            `json:"organizationId,omitempty"`
	DepartmentID        int64             `json:"departmentId,omitempty"`
	Actor               Actor             `json:"actor"`
	Source              Source            `json:"source"`
	Resource            Resource          `json:"resource"`
	ActionName          string            `json:"-"`
	Result              string            `json:"result,omitempty"`
	InvocationID        string            `json:"invocationId,omitempty"`
	SessionID           string            `json:"sessionId,omitempty"`
	TaskID              string            `json:"taskId,omitempty"`
	WorkflowInstanceID  string            `json:"workflowInstanceId,omitempty"`
	TraceID             string            `json:"traceId,omitempty"`
	CorrelationID       string            `json:"correlationId,omitempty"`
	CausationID         string            `json:"causationId,omitempty"`
	Depth               int               `json:"triggerDepth"`
	Sensitivity         string            `json:"sensitivity"`
	CandidatePartitions []string          `json:"-"`
	SchemaName          string            `json:"-"`
	SchemaVersion       string            `json:"-"`
	Payload             map[string]string `json:"payload"`
}

type Actor struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
type Source struct {
	Type          string `json:"type"`
	ID            string `json:"id,omitempty"`
	Module        string `json:"module,omitempty"`
	PluginID      string `json:"pluginId,omitempty"`
	PluginVersion string `json:"pluginVersion,omitempty"`
}
type Resource struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
}

// MarshalJSON exposes the canonical nested envelope; unavailable context is
// explicitly empty instead of being inferred from unrelated identifiers.
func (e Envelope) MarshalJSON() ([]byte, error) {
	type contextFields struct {
		InvocationID       string `json:"invocationId"`
		SessionID          string `json:"sessionId"`
		TaskID             string `json:"taskId"`
		WorkflowInstanceID string `json:"workflowInstanceId"`
		TraceID            string `json:"traceId"`
		CorrelationID      string `json:"correlationId"`
		CausationID        string `json:"causationId"`
		TriggerDepth       int    `json:"triggerDepth"`
	}
	type classification struct {
		CandidatePartitions []string `json:"candidatePartitions"`
		Sensitivity         string   `json:"sensitivity"`
	}
	type action struct {
		Name   string `json:"name"`
		Result string `json:"result"`
	}
	type schema struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	return json.Marshal(struct {
		EventID        string            `json:"eventId"`
		EventType      string            `json:"eventType"`
		EventVersion   string            `json:"eventVersion"`
		OccurredAt     time.Time         `json:"occurredAt"`
		IngestedAt     time.Time         `json:"ingestedAt"`
		TenantID       string            `json:"tenantId"`
		OrganizationID string            `json:"organizationId"`
		DepartmentID   int64             `json:"departmentId"`
		Actor          Actor             `json:"actor"`
		Source         Source            `json:"source"`
		Action         action            `json:"action"`
		Resource       Resource          `json:"resource"`
		Context        contextFields     `json:"context"`
		Classification classification    `json:"classification"`
		Payload        map[string]string `json:"payload"`
		Schema         schema            `json:"schema"`
	}{e.EventID, e.EventType, e.EventVersion, e.OccurredAt, e.IngestedAt, e.TenantID, e.OrganizationID, e.DepartmentID, e.Actor, e.Source, action{Name: e.ActionName, Result: e.Result}, e.Resource,
		contextFields{e.InvocationID, e.SessionID, e.TaskID, e.WorkflowInstanceID, e.TraceID, e.CorrelationID, e.CausationID, e.Depth}, classification{e.CandidatePartitions, e.Sensitivity}, e.Payload, schema{e.SchemaName, e.SchemaVersion}})
}

// Sink receives an already normalized event. Implementations must treat delivery
// as at least once and deduplicate by tenant and event ID.
type Sink interface {
	Collect(context.Context, Envelope) error
}

// Policy maps each registered event type to the permitted payload keys and
// values. Unknown types and values fail closed; an empty set permits no values.
type Policy map[string]map[string][]string

// Collector normalizes every event before it reaches the sink. Policy and sink
// are configured server-side; neither is derived from client input.
type Collector struct {
	policy Policy
	sink   Sink
}

func NewCollector(policy Policy, sink Sink) *Collector {
	// Make the privacy policy immutable to callers after construction.
	copyPolicy := make(Policy, len(policy))
	for eventType, fields := range policy {
		copyFields := make(map[string][]string, len(fields))
		for field, values := range fields {
			copyFields[field] = append([]string(nil), values...)
		}
		copyPolicy[eventType] = copyFields
	}
	return &Collector{policy: copyPolicy, sink: sink}
}

func (c *Collector) Collect(ctx context.Context, e Envelope) error {
	if c == nil || c.sink == nil {
		return ErrInvalidEvent
	}
	normalized, err := c.policy.Normalize(e, time.Now())
	if err != nil {
		return err
	}
	return c.sink.Collect(ctx, normalized)
}

func (p Policy) Normalize(e Envelope, now time.Time) (Envelope, error) {
	allowed, ok := p[e.EventType]
	if !ok || e.EventID == "" || e.TenantID == "" || e.Actor.ID == "" || e.Actor.Type == "" || e.Source.Type == "" || e.OccurredAt.IsZero() || e.Depth < 0 || now.IsZero() {
		return Envelope{}, ErrInvalidEvent
	}
	for _, s := range []string{e.EventID, e.EventType, e.TenantID, e.OrganizationID, e.Actor.ID, e.Actor.Type, e.Source.Type, e.Source.ID, e.Source.Module, e.Source.PluginID, e.Source.PluginVersion, e.Resource.Type, e.Resource.ID, e.ActionName, e.Result, e.InvocationID, e.SessionID, e.TaskID, e.WorkflowInstanceID, e.TraceID, e.CorrelationID, e.CausationID, e.Sensitivity, e.SchemaName, e.SchemaVersion} {
		if len(s) > 256 || strings.ContainsAny(s, "\r\n\x00") {
			return Envelope{}, ErrInvalidEvent
		}
	}
	if len(e.Payload) > 32 {
		return Envelope{}, ErrInvalidEvent
	}
	clean := make(map[string]string, len(e.Payload))
	for key, value := range e.Payload {
		values, exists := allowed[key]
		if !exists {
			continue
		}
		for _, permitted := range values {
			if value == permitted {
				clean[key] = value
				break
			}
		}
	}
	e.Payload = clean
	if e.Sensitivity == "" {
		e.Sensitivity = "internal"
	}
	switch {
	case strings.HasPrefix(e.EventType, "system.exception."):
		e.CandidatePartitions = []string{"exception"}
	case strings.HasPrefix(e.EventType, "workflow.node.failed"):
		e.CandidatePartitions = []string{"execution", "exception"}
	case strings.HasPrefix(e.EventType, "agent."), strings.HasPrefix(e.EventType, "workflow."):
		e.CandidatePartitions = []string{"execution"}
	case strings.HasPrefix(e.EventType, "capability."):
		e.CandidatePartitions = []string{"activity", "execution"}
	case strings.HasPrefix(e.EventType, "activity."):
		e.CandidatePartitions = []string{"activity"}
	case e.Source.Type == "domain_event":
		e.CandidatePartitions = []string{"business", "activity"}
	default:
		e.CandidatePartitions = []string{}
	}
	e.EventVersion = EnvelopeVersion
	e.IngestedAt = now.UTC()
	e.OccurredAt = e.OccurredAt.UTC()
	return e, nil
}

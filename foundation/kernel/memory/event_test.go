package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type recordingSink struct{ events []Envelope }

func (s *recordingSink) Collect(_ context.Context, e Envelope) error {
	s.events = append(s.events, e)
	return nil
}

func TestCollectorFreezesPolicyAndFiltersBeforeDelivery(t *testing.T) {
	p := Policy{"activity.page.viewed": {"action": {"view"}}}
	sink := &recordingSink{}
	collector := NewCollector(p, sink)
	p["activity.page.viewed"]["secret"] = []string{"password"}
	e := Envelope{EventID: "evt", EventType: "activity.page.viewed", TenantID: "tenant", Actor: Actor{ID: "1", Type: "user"}, Source: Source{Type: "web"}, OccurredAt: time.Now(), Payload: map[string]string{"action": "view", "secret": "password"}}
	if err := collector.Collect(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 || len(sink.events[0].Payload) != 1 || sink.events[0].Payload["action"] != "view" {
		t.Fatalf("unexpected delivery: %#v", sink.events)
	}
}

func TestNormalizeFiltersUnregisteredContent(t *testing.T) {
	now := time.Now()
	e := Envelope{EventID: "evt", EventType: "activity.page.viewed", TenantID: "t", Actor: Actor{ID: "1", Type: "user"}, Source: Source{Type: "web"}, OccurredAt: now, Payload: map[string]string{"action": "view", "password": "secret", "result": "private text"}}
	got, err := (Policy{"activity.page.viewed": {"action": {"view"}, "result": {"success", "failure"}}}).Normalize(e, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Payload) != 1 || got.Payload["action"] != "view" {
		t.Fatalf("unexpected payload: %#v", got.Payload)
	}
	if got.EventVersion != EnvelopeVersion || got.IngestedAt.IsZero() {
		t.Fatal("missing ingestion metadata")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Classification struct {
			CandidatePartitions []string `json:"candidatePartitions"`
			Sensitivity         string   `json:"sensitivity"`
		} `json:"classification"`
		Context struct {
			CorrelationID string `json:"correlationId"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire.Classification.CandidatePartitions) != 1 || wire.Classification.CandidatePartitions[0] != "activity" || wire.Classification.Sensitivity != "internal" {
		t.Fatalf("invalid canonical envelope: %s %v", raw, err)
	}
	if len(e.Payload) != 3 {
		t.Fatal("input payload mutated")
	}
}

func TestNormalizeRejectsUnknownOrInvalidEvent(t *testing.T) {
	now := time.Now()
	e := Envelope{EventID: "evt", EventType: "unknown", TenantID: "t", Actor: Actor{ID: "1", Type: "user"}, Source: Source{Type: "web"}, OccurredAt: now}
	if _, err := (Policy{}).Normalize(e, now); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("unknown event: %v", err)
	}
	e.EventType = "known"
	e.TenantID = ""
	if _, err := (Policy{"known": {}}).Normalize(e, now); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("missing tenant: %v", err)
	}
}

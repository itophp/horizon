package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	canonical "baseline/services/api/kernel/memory"
	"baseline/services/api/system/identity"
)

type denialSink struct{ events []canonical.Envelope }

func (s *denialSink) Submit(e canonical.Envelope) { s.events = append(s.events, e) }

func TestDeniedCapabilityObservationExcludesInput(t *testing.T) {
	d := contracts.Definition{Layer: contracts.LayerAtomic, Name: "example.record.read", Version: "1.0.0", Title: "Read", Description: "Read a record.", Domain: "example", Entity: "example_record", Permission: "example:read", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, Executor: "go:read", Input: contracts.Object(nil), Output: contracts.Object(nil)}
	r := registry.New([]registry.Entry{{Definition: d, Handler: contracts.Handler{Execute: func(context.Context, contracts.Input) (contracts.Output, error) {
		t.Fatal("unauthorized handler ran")
		return contracts.Output{}, nil
	}}}})
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	p, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 1}, "tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	sink := &denialSink{}
	g := New(Options{Registry: r, Observations: sink})
	_, err = g.Invoke(identity.WithPrincipal(context.Background(), p), Request{Name: d.Name, Origin: contracts.OriginHumanUI, Input: contracts.Input{"password": "sensitive"}})
	if contracts.Code(err) != "forbidden" || len(sink.events) != 1 {
		t.Fatalf("expected denial observation: %v %#v", err, sink.events)
	}
	raw, err := json.Marshal(sink.events[0])
	if err != nil || strings.Contains(string(raw), "sensitive") || sink.events[0].Payload["reason"] != "permission" {
		t.Fatalf("unsafe observation: %s %v", raw, err)
	}
}

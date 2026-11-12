package harness

import (
	"context"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	genidentity "horizon/foundation/gen/identity"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/system/identity"
	"strings"
	"testing"
)

func TestProgressRequiresTheSameIdentityAndGrantAsRun(t *testing.T) {
	service := &Service{}
	principal, err := identity.NewPrincipal(&genidentity.AccountRecord{ID: 42, DepartmentID: 1}, "tenant-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ctx := range []context.Context{context.Background(), identity.WithPrincipal(context.Background(), principal)} {
		events := 0
		if _, err := service.RunStream(ctx, RunInput{Question: "test"}, func(Progress) { events++ }); err == nil {
			t.Fatal("unauthorized stream accepted")
		}
		if events != 0 {
			t.Fatal("unauthorized progress leaked")
		}
	}
}

func TestProgressStreamsPublicContentWithoutPrivateReasoning(t *testing.T) {
	var events []Progress
	ctx := context.WithValue(context.Background(), progressKey{}, progressHandler(func(event Progress) { events = append(events, event) }))
	output := &adk.MessageVariant{IsStreaming: true, Role: schema.Assistant, MessageStream: schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, Content: "hello ", ReasoningContent: "private-secret"},
		{Role: schema.Assistant, Content: "world"},
	})}
	message, err := publicMessage(ctx, output)
	if err != nil || message.Content != "hello world" {
		t.Fatalf("message = %v, %v", message, err)
	}
	if len(events) != 2 || events[0].Delta != "hello " || events[1].Delta != "world" {
		t.Fatalf("wrong deltas: %+v", events)
	}
	for _, event := range events {
		if strings.Contains(event.Delta, "private-secret") {
			t.Fatal("private reasoning leaked")
		}
	}
}

func TestApprovalProjectionSettlesOnlyAfterAllCallsAreTerminal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		calls []audit.Invocation
		want  string
	}{
		{"pending", []audit.Invocation{{Status: "awaiting_approval"}}, StatusAwaitingApproval},
		{"partially settled", []audit.Invocation{{Status: "succeeded"}, {Status: "awaiting_approval"}}, StatusAwaitingApproval},
		{"approved", []audit.Invocation{{Status: "succeeded"}}, StatusCompleted},
		{"business failed", []audit.Invocation{{Status: "failed"}}, StatusCompleted},
		{"not loaded", nil, StatusAwaitingApproval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := &Run{Status: StatusAwaitingApproval, Invocations: tc.calls}
			settleWaiting(run)
			if run.Status != tc.want {
				t.Fatalf("status=%s want %s", run.Status, tc.want)
			}
		})
	}
	run := &Run{Status: StatusFailed, Invocations: []audit.Invocation{{Status: "succeeded"}}}
	settleWaiting(run)
	if run.Status != StatusFailed {
		t.Fatal("AI failure was overwritten")
	}
}

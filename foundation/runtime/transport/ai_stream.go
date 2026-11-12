package transport

import (
	"context"
	"horizon/foundation/ai/harness"
	genai "horizon/foundation/gen/ai"
	"horizon/foundation/kernel/capability/contracts"
	"sync"
)

// The generated Goa SSE transport shares the authenticated run boundary.
func (a *aiAdapter) RunStream(ctx context.Context, payload *genai.RunStreamPayload, stream genai.RunStreamServerStream) error {
	input := harness.RunInput{Question: payload.Question, Origin: contracts.OriginHumanUI}
	if payload.SessionID != nil {
		input.SessionID = *payload.SessionID
	}
	if payload.Page != nil {
		if payload.Page.Route != nil {
			input.Page.Route = *payload.Page.Route
		}
		if payload.Page.Entity != nil {
			input.Page.Entity = *payload.Page.Entity
		}
		if payload.Page.EntityID != nil {
			input.Page.EntityID = *payload.Page.EntityID
		}
	}
	var mu sync.Mutex
	sequence := 0
	var sendErr error
	emit := func(event harness.Progress) {
		mu.Lock()
		defer mu.Unlock()
		if sendErr != nil {
			return
		}
		sequence++
		value := &genai.AIStreamEvent{Sequence: sequence, Kind: event.Kind, Delta: event.Delta, Capability: event.Capability, Status: event.Status, ErrorCode: event.ErrorCode}
		if event.Run != nil {
			value.Run = aiRunProjection(event.Run)
		}
		sendErr = stream.Send(value)
	}
	run, err := a.harness.RunStream(ctx, input, emit)
	if err != nil {
		// Before the first event the normal HTTP authorization/error contract applies.
		mu.Lock()
		started := sequence > 0
		mu.Unlock()
		if !started {
			return aiProblem(err)
		}
		emit(harness.Progress{Kind: "failed", ErrorCode: aiErrorCode(err)})
	} else {
		kind := "completed"
		if run.Status == harness.StatusFailed || run.Status == harness.StatusCancelled {
			kind = "failed"
		}
		emit(harness.Progress{Kind: kind, Run: run, ErrorCode: run.ErrorCode})
	}
	mu.Lock()
	defer mu.Unlock()
	return sendErr
}

package memory

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	canonical "horizon/foundation/kernel/memory"
)

func telemetryEvent(id string) canonical.Envelope {
	return canonical.Envelope{
		EventID: id, EventType: "activity.http.completed", TenantID: "tenant-a",
		OccurredAt: time.Now(), Actor: canonical.Actor{ID: "1", Type: "user"},
		Source: canonical.Source{Type: "http"}, Payload: map[string]string{"method": "GET"},
	}
}

func TestObserverExhaustedDeliveryUnblocksQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		started := time.Now()
		attempts := 0
		o := &Observer{queue: make(chan canonical.Envelope, 2)}
		o.persist = func(ctx context.Context, e canonical.Envelope) error {
			if e.EventID == "blocked" {
				attempts++
				<-ctx.Done()
				return ctx.Err()
			}
			cancel()
			return nil
		}
		o.Submit(telemetryEvent("blocked"))
		o.Submit(telemetryEvent("healthy"))
		o.Submit(telemetryEvent("overflow"))
		o.Run(ctx)
		if attempts != 5 || time.Since(started) != 35*time.Second {
			t.Fatalf("delivery retry budget: %d attempts, %v", attempts, time.Since(started))
		}
		if o.submitted.Load() != 2 || o.stored.Load() != 1 || o.failed.Load() != 1 || o.dropped.Load() != 1 {
			t.Fatalf("unexpected counters: submitted=%d stored=%d failed=%d dropped=%d",
				o.submitted.Load(), o.stored.Load(), o.failed.Load(), o.dropped.Load())
		}
	})
}

func TestObserverCancellationAndNewInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		o := &Observer{queue: make(chan canonical.Envelope, 2)}
		o.persist = func(ctx context.Context, _ canonical.Envelope) error {
			<-ctx.Done()
			return ctx.Err()
		}
		o.Submit(telemetryEvent("in-flight"))
		o.Submit(telemetryEvent("queued"))
		done := make(chan struct{})
		go func() {
			defer close(done)
			o.Run(ctx)
		}()
		synctest.Wait()
		cancel()
		<-done
		if o.dropped.Load() != 2 || o.stored.Load() != 0 || o.failed.Load() != 0 {
			t.Fatalf("unexpected shutdown counters: dropped=%d stored=%d failed=%d",
				o.dropped.Load(), o.stored.Load(), o.failed.Load())
		}

		ctx, cancel = context.WithCancel(t.Context())
		defer cancel()
		next := &Observer{queue: make(chan canonical.Envelope, 2)}
		attempts := 0
		next.persist = func(_ context.Context, e canonical.Envelope) error {
			if e.EventID != "new" {
				t.Fatalf("previous instance's event was delivered: %s", e.EventID)
			}
			attempts++
			if attempts == 1 {
				return errors.New("temporary storage failure")
			}
			cancel()
			return nil
		}
		next.Submit(telemetryEvent("new"))
		next.Run(ctx)
		if attempts != 2 || next.stored.Load() != 1 {
			t.Fatalf("new instance failed to recover: attempts=%d stored=%d", attempts, next.stored.Load())
		}
	})
}

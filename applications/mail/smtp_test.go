package mail

import (
	"context"
	"errors"
	"testing"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestMailEndpointsUseExactAllowlist(t *testing.T) {
	allowed := []string{"smtp.example.test:587", "pop.example.test:995"}
	for _, item := range []struct {
		host string
		port int
		ok   bool
	}{
		{"smtp.example.test", 587, true},
		{"pop.example.test", 995, true},
		{"smtp.example.test", 25, false},
		{"evil.example.test", 587, false},
	} {
		if got := allowedEndpoint(item.host, item.port, allowed); got != item.ok {
			t.Fatalf("allowedEndpoint(%s:%d)=%v", item.host, item.port, got)
		}
	}
}

func TestTimeoutClassifiesAmbiguousProviderOutcome(t *testing.T) {
	if !timeout(context.DeadlineExceeded) {
		t.Fatal("context deadline was not classified as timeout")
	}
	if !timeout(timeoutError{}) {
		t.Fatal("network timeout was not classified as timeout")
	}
	if timeout(errors.New("authentication failed")) {
		t.Fatal("ordinary provider failure was misclassified as timeout")
	}
}

func TestDeliveryOutcomeRetriesUntilLimitThenStops(t *testing.T) {
	status, code, event := deliveryOutcome(errors.New("authentication failed"), 1, 5)
	if status != "queued" || code != "unavailable" || event != DeliveryFailedEvent {
		t.Fatalf("retryable outcome=%s/%s/%s", status, code, event)
	}
	status, code, event = deliveryOutcome(errors.New("authentication failed"), 5, 5)
	if status != "failed" || code != "unavailable" || event != DeliveryFailedEvent {
		t.Fatalf("terminal outcome=%s/%s/%s", status, code, event)
	}
	status, code, event = deliveryOutcome(timeoutError{}, 1, 5)
	if status != "ambiguous" || code != "timeout" || event != DeliveryAmbiguous {
		t.Fatalf("timeout outcome=%s/%s/%s", status, code, event)
	}
}

package harness

import (
	genai "baseline/services/api/gen/ai"
	aihttp "baseline/services/api/gen/http/ai/server"
	"bufio"
	"context"
	goahttp "goa.design/goa/v3/http"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Exercise the actual generated HTTP stream: the first event must arrive
// before the endpoint is allowed to finish, not as a buffered JSON response.
func TestGoaSSEFlushesBeforeCompletion(t *testing.T) {
	gate := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	handler := aihttp.NewRunStreamHandler(func(ctx context.Context, value any) (any, error) {
		stream := value.(*genai.RunStreamEndpointInput).Stream
		if err := stream.Send(&genai.AIStreamEvent{Sequence: 1, Kind: "planning"}); err != nil {
			return nil, err
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, stream.Send(&genai.AIStreamEvent{Sequence: 2, Kind: "completed"})
	}, goahttp.NewMuxer(), goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	server := httptest.NewServer(handler)
	defer func() { release(); server.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(`{"question":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", "session=fixture")
	request.Header.Set("Accept", "text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("not SSE: %d", response.StatusCode)
	}
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(first, `"kind":"planning"`) {
		t.Fatalf("first event=%q err=%v", first, err)
	}
	release()
	rest, err := io.ReadAll(reader)
	if err != nil || !strings.Contains(string(rest), `"kind":"completed"`) {
		t.Fatalf("final event missing: %q %v", rest, err)
	}
}

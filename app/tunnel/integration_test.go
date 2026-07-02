package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bdnelson/sse-webhook-tunnel/core/forward"
	"github.com/bdnelson/sse-webhook-tunnel/core/sse"
)

// TestTunnel_EndToEnd wires the real SSE client, forwarder, and tunnel together
// against live local servers, verifying the complete data path from an SSE
// frame to a forwarded HTTP request and a published event.
func TestTunnel_EndToEnd(t *testing.T) {
	// Sink captures forwarded requests.
	var (
		mu       sync.Mutex
		received []string
		headers  []string
	)
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(body))
		headers = append(headers, r.Header.Get("X-Github-Event"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer sink.Close()

	// Source streams two smee-style events then blocks open.
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 1; i <= 2; i++ {
			fmt.Fprintf(w, "id: %d\nevent: message\ndata: {\"x-github-event\":\"push\",\"body\":{\"seq\":%d}}\n\n", i, i)
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer source.Close()

	client := sse.New(source.URL)
	forwarder := forward.New(false)
	pub := &mockPublisher{}
	tn := New(client, forwarder, pub, sink.URL+"/hook")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = tn.Run(ctx) }()

	// Wait for both events to be forwarded.
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: only %d of 2 events forwarded", count)
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	for i, body := range received {
		want := fmt.Sprintf(`{"seq":%d}`, i+1)
		if body != want {
			t.Errorf("forwarded body[%d] = %q, want %q", i, body, want)
		}
		if headers[i] != "push" {
			t.Errorf("forwarded X-Github-Event[%d] = %q, want push", i, headers[i])
		}
	}

	events := pub.snapshot()
	if len(events) < 2 {
		t.Fatalf("published %d events, want at least 2", len(events))
	}
	for i, e := range events[:2] {
		if !e.Forwarded || e.Status != http.StatusOK {
			t.Errorf("event %d not forwarded ok: %+v", i, e)
		}
		if e.Summary() == "" {
			t.Errorf("event %d has empty summary", i)
		}
	}

	// The payload must survive round-trip for display in the detail view.
	if events[0].PrettyJSON() == "" {
		t.Error("expected non-empty pretty JSON for display")
	}
}

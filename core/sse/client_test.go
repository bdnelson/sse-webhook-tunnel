package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

var _ Stream = (*Client)(nil)

// streamServer returns an httptest.Server that writes the provided raw SSE
// body once, then blocks until the request context is cancelled so the client
// observes a live, long-lived stream.
func streamServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter is not a Flusher")
			return
		}
		fmt.Fprint(w, body)
		flusher.Flush()
		<-r.Context().Done()
	}))
}

func collect(ctx context.Context, ch <-chan Frame, n int) []Frame {
	frames := make([]Frame, 0, n)
	for len(frames) < n {
		select {
		case <-ctx.Done():
			return frames
		case f, ok := <-ch:
			if !ok {
				return frames
			}
			frames = append(frames, f)
		}
	}
	return frames
}

func TestClient_Subscribe_ParsesFrames(t *testing.T) {
	body := "event: message\n" +
		"id: 1\n" +
		"data: {\"body\":{\"a\":1}}\n" +
		"\n" +
		"data: line one\n" +
		"data: line two\n" +
		"\n"

	srv := streamServer(t, body)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := New(srv.URL)
	frames := collect(ctx, c.Subscribe(ctx), 2)

	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if frames[0].Event != "message" || string(frames[0].Data) != `{"body":{"a":1}}` {
		t.Errorf("frame 0 = %+v", frames[0])
	}
	if frames[0].ID != "1" {
		t.Errorf("frame 0 id = %q, want 1", frames[0].ID)
	}
	if string(frames[1].Data) != "line one\nline two" {
		t.Errorf("frame 1 data = %q, want joined lines", string(frames[1].Data))
	}
}

func TestClient_Subscribe_SkipsControlEvents(t *testing.T) {
	body := "event: ready\n" +
		"data: ready\n" +
		"\n" +
		"event: ping\n" +
		"data: {}\n" +
		"\n" +
		"data: {\"body\":{}}\n" +
		"\n"

	srv := streamServer(t, body)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := New(srv.URL)
	frames := collect(ctx, c.Subscribe(ctx), 1)

	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1 (control events skipped)", len(frames))
	}
	if string(frames[0].Data) != `{"body":{}}` {
		t.Errorf("frame data = %q", string(frames[0].Data))
	}
}

func TestClient_Subscribe_IgnoresComments(t *testing.T) {
	body := ":keep-alive\n" +
		"data: {\"body\":{}}\n" +
		"\n"

	srv := streamServer(t, body)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := New(srv.URL)
	frames := collect(ctx, c.Subscribe(ctx), 1)

	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
}

func TestClient_Subscribe_ContextCancelClosesChannel(t *testing.T) {
	srv := streamServer(t, "data: {\"body\":{}}\n\n")
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := New(srv.URL)
	ch := c.Subscribe(ctx)

	// Drain the first frame, then cancel.
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first frame")
	}
	cancel()

	// The channel must close promptly after cancellation.
	closed := make(chan struct{})
	go func() {
		for range ch {
		}
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("channel not closed after context cancel")
	}
}

func TestClient_Subscribe_ResendsLastEventID(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
		gotID    string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		current := attempts
		if current == 2 {
			gotID = r.Header.Get("Last-Event-ID")
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		if current == 1 {
			// First connection: emit an event with an id, then end so the
			// client reconnects.
			fmt.Fprint(w, "id: 99\ndata: {\"body\":{}}\n\n")
			flusher.Flush()
			return
		}
		// Subsequent connections: keep the stream open.
		fmt.Fprint(w, "data: {\"body\":{}}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := New(srv.URL, WithBackoff(10*time.Millisecond, 50*time.Millisecond))
	collect(ctx, c.Subscribe(ctx), 2)

	mu.Lock()
	defer mu.Unlock()
	if gotID != "99" {
		t.Errorf("Last-Event-ID on reconnect = %q, want 99", gotID)
	}
}

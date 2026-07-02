package tunnel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
	"github.com/bdnelson/sse-webhook-tunnel/core/forward"
	"github.com/bdnelson/sse-webhook-tunnel/core/sse"
)

// mockStream is a hand-written Stream mock backed by a preloaded frame slice.
type mockStream struct {
	frames []sse.Frame
}

var _ sse.Stream = (*mockStream)(nil)

func (m *mockStream) Subscribe(ctx context.Context) <-chan sse.Frame {
	ch := make(chan sse.Frame)
	go func() {
		defer close(ch)
		for _, f := range m.frames {
			select {
			case <-ctx.Done():
				return
			case ch <- f:
			}
		}
	}()
	return ch
}

// mockSender is a hand-written Sender mock with an injectable behavior.
type mockSender struct {
	forwardFunc func(ctx context.Context, targetURL string, payload event.Parsed) (int, error)
	calls       int
}

var _ forward.Sender = (*mockSender)(nil)

func (m *mockSender) Forward(ctx context.Context, targetURL string, payload event.Parsed) (int, error) {
	m.calls++
	if m.forwardFunc != nil {
		return m.forwardFunc(ctx, targetURL, payload)
	}
	return 200, nil
}

// mockPublisher collects published events under a lock.
type mockPublisher struct {
	mu     sync.Mutex
	events []event.Event
}

var _ EventPublisher = (*mockPublisher)(nil)

func (m *mockPublisher) Publish(e event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *mockPublisher) snapshot() []event.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]event.Event, len(m.events))
	copy(out, m.events)
	return out
}

func TestTunnel_Run_ForwardsAndPublishes(t *testing.T) {
	stream := &mockStream{frames: []sse.Frame{
		{ID: "1", Event: "message", Data: []byte(`{"body":{"a":1}}`)},
		{ID: "2", Event: "message", Data: []byte(`{"body":{"b":2}}`)},
	}}
	sender := &mockSender{}
	pub := &mockPublisher{}

	fixedTime := time.Date(2026, 7, 2, 13, 35, 52, 0, time.UTC)
	tn := New(stream, sender, pub, "http://target.test/hook", withClock(func() time.Time { return fixedTime }))

	if err := tn.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events := pub.snapshot()
	if len(events) != 2 {
		t.Fatalf("published %d events, want 2", len(events))
	}
	if sender.calls != 2 {
		t.Errorf("forwarder called %d times, want 2", sender.calls)
	}
	if tn.Count() != 2 {
		t.Errorf("Count() = %d, want 2", tn.Count())
	}
	for i, e := range events {
		if !e.Forwarded {
			t.Errorf("event %d not marked forwarded", i)
		}
		if e.Status != 200 {
			t.Errorf("event %d status = %d, want 200", i, e.Status)
		}
		if !e.Time.Equal(fixedTime) {
			t.Errorf("event %d time = %v, want %v", i, e.Time, fixedTime)
		}
	}
}

func TestTunnel_Run_RecordsForwardError(t *testing.T) {
	stream := &mockStream{frames: []sse.Frame{
		{ID: "1", Data: []byte(`{"body":{}}`)},
	}}
	forwardErr := errors.New("target down")
	sender := &mockSender{forwardFunc: func(context.Context, string, event.Parsed) (int, error) {
		return 502, forwardErr
	}}
	pub := &mockPublisher{}

	tn := New(stream, sender, pub, "http://target.test/hook")
	if err := tn.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events := pub.snapshot()
	if len(events) != 1 {
		t.Fatalf("published %d events, want 1", len(events))
	}
	e := events[0]
	if e.Forwarded {
		t.Error("event should not be marked forwarded on error")
	}
	if e.Status != 502 {
		t.Errorf("status = %d, want 502", e.Status)
	}
	if !errors.Is(e.ForwardErr, forwardErr) {
		t.Errorf("ForwardErr = %v, want %v", e.ForwardErr, forwardErr)
	}
}

func TestTunnel_Run_StopsOnContextCancel(t *testing.T) {
	// A stream that never sends; Run must return when the context is cancelled.
	stream := &blockingStream{}
	tn := New(stream, &mockSender{}, &mockPublisher{}, "http://target.test/hook")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tn.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancel")
	}
}

// blockingStream returns a channel that never emits and closes on cancel.
type blockingStream struct{}

var _ sse.Stream = (*blockingStream)(nil)

func (b *blockingStream) Subscribe(ctx context.Context) <-chan sse.Frame {
	ch := make(chan sse.Frame)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}

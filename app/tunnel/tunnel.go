// Package tunnel orchestrates the flow from the SSE source to the HTTP target:
// it consumes SSE frames, interprets each as a webhook payload, forwards it to
// the target, and publishes the resulting event for display.
package tunnel

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
	"github.com/bdnelson/sse-webhook-tunnel/core/forward"
	"github.com/bdnelson/sse-webhook-tunnel/core/sse"
)

// EventPublisher receives completed events for display. The TUI implements this
// by forwarding into the Bubble Tea program's message loop.
type EventPublisher interface {
	Publish(event.Event)
}

// Logger is the minimal logging surface the tunnel depends on.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Tunnel connects an SSE stream to an HTTP forwarder.
type Tunnel struct {
	stream    sse.Stream
	sender    forward.Sender
	publisher EventPublisher
	targetURL string
	log       Logger
	now       func() time.Time

	mu    sync.RWMutex
	count int
}

// Option configures a Tunnel.
type Option func(*Tunnel)

// WithLogger sets the logger.
func WithLogger(l Logger) Option {
	return func(t *Tunnel) { t.log = l }
}

// withClock overrides the time source (used in tests).
func withClock(now func() time.Time) Option {
	return func(t *Tunnel) { t.now = now }
}

// New constructs a Tunnel.
func New(stream sse.Stream, sender forward.Sender, publisher EventPublisher, targetURL string, opts ...Option) *Tunnel {
	t := &Tunnel{
		stream:    stream,
		sender:    sender,
		publisher: publisher,
		targetURL: targetURL,
		log:       nopLogger{},
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Run consumes frames until the context is cancelled or the stream ends. It is
// intended to run in its own goroutine.
func (t *Tunnel) Run(ctx context.Context) error {
	frames := t.stream.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-frames:
			if !ok {
				return nil
			}
			t.handle(ctx, frame)
		}
	}
}

// Count returns the number of events received so far.
func (t *Tunnel) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}

// handle interprets a frame, forwards it, and publishes the resulting event.
func (t *Tunnel) handle(ctx context.Context, frame sse.Frame) {
	ev := event.Event{
		ID:   frame.ID,
		Time: t.now(),
		Raw:  json.RawMessage(frame.Data),
	}

	payload := event.Parse(frame.Data)
	status, err := t.sender.Forward(ctx, t.targetURL, payload)
	ev.Status = status
	if err != nil {
		ev.ForwardErr = err
		t.log.Error("failed to forward event", "id", frame.ID, "status", status, "error", err)
	} else {
		ev.Forwarded = true
		t.log.Info("forwarded event", "id", frame.ID, "status", status)
	}

	t.mu.Lock()
	t.count++
	t.mu.Unlock()

	t.publisher.Publish(ev)
}

// nopLogger is the default no-op logger.
type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

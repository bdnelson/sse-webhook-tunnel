// Package sse implements a minimal Server-Sent Events client using only the
// standard library. It connects to a source URL, parses the event stream, and
// reconnects with backoff, following the SSE specification closely enough to
// consume smee.io channels.
package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Frame is a single dispatched SSE event.
type Frame struct {
	// ID is the last event id seen for the stream, if any.
	ID string
	// Event is the event name ("message" when unspecified).
	Event string
	// Data is the concatenated data payload (multiple data: lines joined by
	// newlines).
	Data []byte
}

// Stream is the interface consumers depend on to receive SSE frames. It allows
// the tunnel to be tested against a mock without a real network connection.
type Stream interface {
	// Subscribe connects and streams frames until the context is cancelled or
	// an unrecoverable error occurs. The returned channel is closed when the
	// subscription ends.
	Subscribe(ctx context.Context) <-chan Frame
}

// controlEvents are named events emitted by smee.io that carry no forwardable
// payload and are therefore skipped.
var controlEvents = map[string]bool{
	"ready": true,
	"ping":  true,
}

// Logger is the minimal logging surface the client depends on.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Client is an SSE client for a single source URL.
type Client struct {
	url            string
	httpClient     *http.Client
	log            Logger
	initialBackoff time.Duration
	maxBackoff     time.Duration

	lastEventID string
}

var _ Stream = (*Client)(nil)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the HTTP client used to connect.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.httpClient = c }
}

// WithLogger sets the logger.
func WithLogger(l Logger) Option {
	return func(cl *Client) { cl.log = l }
}

// WithBackoff sets the initial and maximum reconnect backoff.
func WithBackoff(initial, max time.Duration) Option {
	return func(cl *Client) {
		cl.initialBackoff = initial
		cl.maxBackoff = max
	}
}

// New constructs a Client for the given source URL.
func New(url string, opts ...Option) *Client {
	c := &Client{
		url:            url,
		httpClient:     &http.Client{Timeout: 0}, // no timeout: the stream is long-lived
		log:            nopLogger{},
		initialBackoff: time.Second,
		maxBackoff:     30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Subscribe connects and streams frames until ctx is cancelled. On connection
// loss it reconnects with exponential backoff, honoring any server-provided
// retry interval and resending the Last-Event-ID header.
func (c *Client) Subscribe(ctx context.Context) <-chan Frame {
	out := make(chan Frame)

	go func() {
		defer close(out)
		backoff := c.initialBackoff

		for {
			if ctx.Err() != nil {
				return
			}

			retry, err := c.connectAndStream(ctx, out)
			if retry > 0 {
				backoff = retry
			}

			if ctx.Err() != nil {
				return
			}
			if err != nil {
				c.log.Error("sse connection error, reconnecting", "error", err, "backoff", backoff.String())
			} else {
				c.log.Info("sse stream ended, reconnecting", "backoff", backoff.String())
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > c.maxBackoff {
				backoff = c.maxBackoff
			}
		}
	}()

	return out
}

// connectAndStream performs one connection attempt and streams frames until the
// connection closes or the context is cancelled. The returned duration is a
// server-suggested retry interval (0 when none was provided).
func (c *Client) connectAndStream(ctx context.Context, out chan<- Frame) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("connecting: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	c.log.Info("sse connected", "url", c.url)
	return c.parseStream(ctx, resp.Body, out)
}

// parseStream reads and dispatches SSE frames from r. It returns the most
// recently seen server retry interval.
func (c *Client) parseStream(ctx context.Context, r io.Reader, out chan<- Frame) (time.Duration, error) {
	reader := bufio.NewReader(r)
	var (
		dataLines [][]byte
		eventName string
		retry     time.Duration
	)

	// dispatch emits the accumulated event and resets per-event state.
	dispatch := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := bytes.Join(dataLines, []byte("\n"))
		name := eventName
		dataLines = nil
		eventName = ""

		if controlEvents[name] {
			return nil
		}

		frame := Frame{ID: c.lastEventID, Event: name, Data: data}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- frame:
			return nil
		}
	}

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")

			switch {
			case len(trimmed) == 0:
				if derr := dispatch(); derr != nil {
					return retry, derr
				}
			case bytes.HasPrefix(trimmed, []byte(":")):
				// Comment line; ignored (used as keep-alive).
			default:
				field, value := splitField(trimmed)
				switch field {
				case "event":
					eventName = string(value)
				case "data":
					dataLines = append(dataLines, value)
				case "id":
					c.lastEventID = string(value)
				case "retry":
					if ms, perr := strconv.Atoi(string(value)); perr == nil {
						retry = time.Duration(ms) * time.Millisecond
					}
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				// Dispatch any trailing event before ending.
				_ = dispatch()
				return retry, nil
			}
			return retry, err
		}
	}
}

// splitField splits an SSE line into its field name and value, stripping a
// single leading space from the value per the specification.
func splitField(line []byte) (field string, value []byte) {
	name, rest, found := bytes.Cut(line, []byte(":"))
	if !found {
		return string(line), nil
	}
	return string(name), bytes.TrimPrefix(rest, []byte(" "))
}

// nopLogger is the default no-op logger.
type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

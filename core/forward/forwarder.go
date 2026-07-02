// Package forward replays a parsed SSE payload to the target URL as an HTTP
// request, mirroring the behavior of smee-client.
package forward

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

// Sender replays a parsed payload to a target. It is an interface so the tunnel
// can be tested against a mock.
type Sender interface {
	// Forward delivers the parsed payload to targetURL, returning the target's
	// HTTP status code.
	Forward(ctx context.Context, targetURL string, payload event.Parsed) (int, error)
}

// Forwarder is the default Sender backed by an *http.Client.
type Forwarder struct {
	client *http.Client
}

var _ Sender = (*Forwarder)(nil)

// Option configures a Forwarder.
type Option func(*Forwarder)

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(f *Forwarder) { f.client = c }
}

// New constructs a Forwarder. When insecure is true, TLS certificate
// verification is disabled to support internal test targets using self-signed
// certificates.
func New(insecure bool, opts ...Option) *Forwarder {
	transport := &http.Transport{}
	if insecure {
		// Guarded by the explicit --insecure flag; intended for internal test
		// environments with self-signed certificates.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // G402: opt-in via --insecure
	}
	f := &Forwarder{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Forward POSTs the payload body to targetURL, replaying the parsed headers and
// appending the parsed query-string parameters.
func (f *Forwarder) Forward(ctx context.Context, targetURL string, payload event.Parsed) (int, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return 0, fmt.Errorf("parsing target url: %w", err)
	}

	if len(payload.Query) > 0 {
		query := target.Query()
		for key, values := range payload.Query {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		target.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(payload.Body))
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	for key, value := range payload.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("forwarding to target: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resp.StatusCode, fmt.Errorf("target returned status %d: %s", resp.StatusCode, string(body))
	}

	return resp.StatusCode, nil
}

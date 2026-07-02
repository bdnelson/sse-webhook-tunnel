// Package event defines the domain model for a tunneled webhook event and the
// logic for interpreting an SSE data frame into a forwardable HTTP request.
package event

import (
	"bytes"
	"encoding/json"
	"time"
)

// Event is a single inbound message received over SSE and (optionally)
// forwarded to the target URL.
type Event struct {
	// ID is the SSE event id, when present.
	ID string
	// Time is when the event was received.
	Time time.Time
	// Raw is the event data exactly as received over SSE. It is retained for
	// display and is not guaranteed to be valid JSON.
	Raw json.RawMessage
	// Forwarded reports whether a forward attempt to the target completed
	// without a transport-level error.
	Forwarded bool
	// Status is the HTTP status code returned by the target, when forwarded.
	Status int
	// ForwardErr holds any error encountered while forwarding.
	ForwardErr error
}

// timeFormat is the timestamp layout used in the event list, e.g.
// "2026-07-02 13:35:52".
const timeFormat = "2006-01-02 15:04:05"

// Summary renders the single-line list entry for the event, for example
// "2026-07-02 13:35:52 Payload received".
func (e Event) Summary() string {
	return e.Time.Format(timeFormat) + " Payload received"
}

// PrettyJSON returns the event payload indented for display. When the raw data
// is not valid JSON it is returned unchanged.
func (e Event) PrettyJSON() string {
	if len(e.Raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, e.Raw, "", "  "); err != nil {
		return string(e.Raw)
	}
	return buf.String()
}

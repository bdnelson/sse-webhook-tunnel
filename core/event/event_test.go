package event

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEvent_Summary(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "formats timestamp with fixed layout",
			time:     time.Date(2026, 7, 2, 13, 35, 52, 0, time.UTC),
			expected: "2026-07-02 13:35:52 Payload received",
		},
		{
			name:     "pads single-digit fields",
			time:     time.Date(2026, 1, 5, 4, 3, 9, 0, time.UTC),
			expected: "2026-01-05 04:03:09 Payload received",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Time: tt.time}
			if got := e.Summary(); got != tt.expected {
				t.Errorf("Summary() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEvent_PrettyJSON(t *testing.T) {
	tests := []struct {
		name     string
		raw      json.RawMessage
		expected string
	}{
		{
			name:     "empty raw returns empty string",
			raw:      nil,
			expected: "",
		},
		{
			name:     "valid json is indented",
			raw:      json.RawMessage(`{"a":1,"b":"x"}`),
			expected: "{\n  \"a\": 1,\n  \"b\": \"x\"\n}",
		},
		{
			name:     "invalid json is returned unchanged",
			raw:      json.RawMessage(`not json`),
			expected: "not json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Raw: tt.raw}
			if got := e.PrettyJSON(); got != tt.expected {
				t.Errorf("PrettyJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEvent_FieldsRetained(t *testing.T) {
	forwardErr := errors.New("boom")
	e := Event{
		ID:         "42",
		Forwarded:  true,
		Status:     200,
		ForwardErr: forwardErr,
	}
	if e.ID != "42" || !e.Forwarded || e.Status != 200 || !errors.Is(e.ForwardErr, forwardErr) {
		t.Errorf("event fields not retained: %+v", e)
	}
}

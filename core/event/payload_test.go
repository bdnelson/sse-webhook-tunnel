package event

import (
	"encoding/json"
	"testing"
)

func TestParse_SmeeEnvelope(t *testing.T) {
	envelope := `{
		"host": "smee.io",
		"content-length": "123",
		"content-type": "application/json",
		"x-github-event": "push",
		"x-github-delivery": "abc-123",
		"timestamp": 1751462152000,
		"query": {"foo": "bar"},
		"body": {"action": "opened", "number": 7}
	}`

	got := Parse([]byte(envelope))

	// Body is the re-serialized "body" object.
	var body map[string]any
	if err := json.Unmarshal(got.Body, &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["action"] != "opened" {
		t.Errorf("body action = %v, want opened", body["action"])
	}

	// Scalar string headers are replayed; body/query/timestamp are dropped.
	if got.Headers["x-github-event"] != "push" {
		t.Errorf("x-github-event = %q, want push", got.Headers["x-github-event"])
	}
	if got.Headers["content-type"] != "application/json" {
		t.Errorf("content-type = %q, want application/json", got.Headers["content-type"])
	}

	// Hop-by-hop headers are stripped.
	if _, ok := got.Headers["host"]; ok {
		t.Error("host header should be stripped")
	}
	if _, ok := got.Headers["content-length"]; ok {
		t.Error("content-length header should be stripped")
	}

	// Non-header envelope keys are not surfaced as headers.
	for _, key := range []string{"body", "query", "timestamp"} {
		if _, ok := got.Headers[key]; ok {
			t.Errorf("%q should not be a header", key)
		}
	}

	if got.Query.Get("foo") != "bar" {
		t.Errorf("query foo = %q, want bar", got.Query.Get("foo"))
	}
}

func TestParse_DefaultsContentType(t *testing.T) {
	got := Parse([]byte(`{"x-github-event":"ping","body":{"ok":true}}`))
	if got.Headers["content-type"] != "application/json" {
		t.Errorf("content-type = %q, want application/json default", got.Headers["content-type"])
	}
}

func TestParse_RawFallback(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "not json", data: `this is not json`},
		{name: "json without body key", data: `{"action":"opened","number":7}`},
		{name: "json array", data: `[1,2,3]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse([]byte(tt.data))
			if string(got.Body) != tt.data {
				t.Errorf("Body = %q, want verbatim %q", string(got.Body), tt.data)
			}
			if got.Headers["content-type"] != "application/json" {
				t.Errorf("content-type = %q, want application/json", got.Headers["content-type"])
			}
			if len(got.Query) != 0 {
				t.Errorf("Query = %v, want empty", got.Query)
			}
		})
	}
}

func TestParse_QueryAsString(t *testing.T) {
	got := Parse([]byte(`{"query":"a=1&b=2","body":{}}`))
	if got.Query.Get("a") != "1" || got.Query.Get("b") != "2" {
		t.Errorf("Query = %v, want a=1 b=2", got.Query)
	}
}

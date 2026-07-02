package event

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

// Parsed is the result of interpreting an SSE data frame as a forwardable
// HTTP request.
type Parsed struct {
	// Headers are the request headers to replay to the target.
	Headers map[string]string
	// Body is the request body to POST to the target.
	Body []byte
	// Query holds query-string parameters to append to the target URL.
	Query url.Values
}

// hopByHopKeys are envelope header keys that must not be replayed because the
// HTTP client recomputes them for the outgoing request.
var hopByHopKeys = map[string]bool{
	"host":           true,
	"content-length": true,
}

// Parse interprets an SSE data frame following smee.io/smee-client semantics:
// the payload is a JSON object whose top-level scalar keys are the original
// request headers, whose "body" key is the request body, whose "query" key
// carries query-string parameters, and whose "timestamp" key is metadata.
//
// When the data is not a smee envelope (it does not unmarshal as an object, or
// it lacks a "body" key) the raw data is forwarded verbatim as a JSON body.
func Parse(data []byte) Parsed {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return rawFallback(data)
	}

	bodyRaw, hasBody := envelope["body"]
	if !hasBody {
		return rawFallback(data)
	}

	parsed := Parsed{
		Headers: make(map[string]string),
		Body:    []byte(bodyRaw),
		Query:   url.Values{},
	}

	if queryRaw, ok := envelope["query"]; ok {
		parsed.Query = parseQuery(queryRaw)
	}

	for key, valueRaw := range envelope {
		switch key {
		case "body", "query", "timestamp":
			continue
		}
		if hopByHopKeys[key] {
			continue
		}
		// Only scalar string header values are replayed; non-string values
		// (nested objects, numbers) are envelope metadata, not headers.
		var value string
		if err := json.Unmarshal(valueRaw, &value); err != nil {
			continue
		}
		parsed.Headers[key] = value
	}

	if _, ok := parsed.Headers["content-type"]; !ok {
		parsed.Headers["content-type"] = "application/json"
	}

	// Unwrap a GitHub form-encoded "payload" field into the inner JSON. This
	// must run after the header loop so it overrides any copied content-type
	// (e.g. application/x-www-form-urlencoded) with application/json.
	if body, ok := unwrapPayload(parsed.Body); ok {
		parsed.Body = body
		parsed.Headers["content-type"] = "application/json"
	}

	return parsed
}

// rawFallback forwards the data verbatim as a JSON body with no replayed
// headers or query parameters. A GitHub form-encoded "payload" wrapper is still
// unwrapped into its inner JSON.
func rawFallback(data []byte) Parsed {
	body := data
	if unwrapped, ok := unwrapPayload(data); ok {
		body = unwrapped
	}
	return Parsed{
		Headers: map[string]string{"content-type": "application/json"},
		Body:    body,
		Query:   url.Values{},
	}
}

// unwrapPayload handles the GitHub form-encoded webhook shape. When body is a
// JSON object whose "payload" field is a string containing a JSON object or
// array, it returns that inner JSON (compacted) and true. Otherwise it returns
// body unchanged and false.
func unwrapPayload(body []byte) ([]byte, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, false
	}
	raw, ok := obj["payload"]
	if !ok {
		return body, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { // payload is not a string
		return body, false
	}
	inner := []byte(strings.TrimSpace(s))
	// Only unwrap a JSON object or array, per the intent ("JSON object"); this
	// leaves scalar strings and non-JSON values alone.
	if len(inner) == 0 || (inner[0] != '{' && inner[0] != '[') || !json.Valid(inner) {
		return body, false
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, inner); err != nil {
		return body, false
	}
	return buf.Bytes(), true
}

// parseQuery converts the envelope "query" value into url.Values. It accepts
// either an object of string values or a raw query string.
func parseQuery(raw json.RawMessage) url.Values {
	var asObject map[string]string
	if err := json.Unmarshal(raw, &asObject); err == nil {
		values := url.Values{}
		for key, value := range asObject {
			values.Set(key, value)
		}
		return values
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if values, err := url.ParseQuery(asString); err == nil {
			return values
		}
	}

	return url.Values{}
}

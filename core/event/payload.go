package event

import (
	"encoding/json"
	"net/url"
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

	return parsed
}

// rawFallback forwards the data verbatim as a JSON body with no replayed
// headers or query parameters.
func rawFallback(data []byte) Parsed {
	return Parsed{
		Headers: map[string]string{"content-type": "application/json"},
		Body:    data,
		Query:   url.Values{},
	}
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

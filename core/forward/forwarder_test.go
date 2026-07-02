package forward

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

func TestForwarder_Forward_ReplaysRequest(t *testing.T) {
	var (
		gotMethod string
		gotBody   string
		gotHeader string
		gotQuery  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Github-Event")
		gotQuery = r.URL.Query().Get("foo")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := event.Parsed{
		Headers: map[string]string{"X-Github-Event": "push", "content-type": "application/json"},
		Body:    []byte(`{"action":"opened"}`),
		Query:   map[string][]string{"foo": {"bar"}},
	}

	f := New(false)
	status, err := f.Forward(context.Background(), srv.URL, payload)
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != `{"action":"opened"}` {
		t.Errorf("body = %q", gotBody)
	}
	if gotHeader != "push" {
		t.Errorf("X-Github-Event = %q, want push", gotHeader)
	}
	if gotQuery != "bar" {
		t.Errorf("query foo = %q, want bar", gotQuery)
	}
}

func TestForwarder_Forward_Non2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := New(false)
	status, err := f.Forward(context.Background(), srv.URL, event.Parsed{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for non-2xx status")
	}
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
}

func TestForwarder_Forward_InvalidTargetURL(t *testing.T) {
	f := New(false)
	if _, err := f.Forward(context.Background(), "://bad", event.Parsed{Body: []byte(`{}`)}); err == nil {
		t.Fatal("expected error for invalid target url")
	}
}

func TestForwarder_Forward_DefaultsContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := New(false)
	if _, err := f.Forward(context.Background(), srv.URL, event.Parsed{Body: []byte(`{}`)}); err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

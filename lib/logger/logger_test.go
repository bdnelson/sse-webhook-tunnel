package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_WritesStructuredRecordsToFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	log, err := New(logFile)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Info("connected", "url", "https://example.test")
	log.Error("failed", "error", "boom")
	_ = log.Sync()

	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	got := string(contents)
	for _, want := range []string{"connected", "https://example.test", "failed", "boom"} {
		if !strings.Contains(got, want) {
			t.Errorf("log file missing %q; contents:\n%s", want, got)
		}
	}
}

func TestNew_InvalidPathReturnsError(t *testing.T) {
	// A path whose parent directory does not exist cannot be opened.
	_, err := New(filepath.Join(t.TempDir(), "missing-dir", "test.log"))
	if err == nil {
		t.Fatal("expected error for unwritable log path")
	}
}

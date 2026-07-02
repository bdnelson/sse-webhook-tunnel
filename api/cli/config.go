package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// config holds the runtime configuration for the tunnel.
type config struct {
	// SourceURL is the SSE endpoint to subscribe to.
	SourceURL string
	// TargetURL is the URL each event is forwarded to.
	TargetURL string
	// LogFile is where structured logs are written.
	LogFile string
	// Insecure disables TLS verification for the target.
	Insecure bool
}

const defaultLogFile = "sse-webhook-tunnel.log"

// parseConfig builds the configuration from command-line arguments, falling
// back to environment variables. Flags take precedence over the environment.
// getenv is injected to keep the function testable.
func parseConfig(args []string, getenv func(string) string) (config, error) {
	fs := flag.NewFlagSet("sse-webhook-tunnel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	source := fs.String("source", getenv("SOURCE_URL"), "SSE source URL to subscribe to (env SOURCE_URL)")
	target := fs.String("target", getenv("TARGET_URL"), "target URL to forward events to (env TARGET_URL)")
	logFile := fs.String("log-file", envOr(getenv, "LOG_FILE", defaultLogFile), "path to the log file (env LOG_FILE)")
	insecure := fs.Bool("insecure", envBool(getenv("INSECURE")), "skip TLS verification for the target (env INSECURE)")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	cfg := config{
		SourceURL: strings.TrimSpace(*source),
		TargetURL: strings.TrimSpace(*target),
		LogFile:   strings.TrimSpace(*logFile),
		Insecure:  *insecure,
	}

	var missing []string
	if cfg.SourceURL == "" {
		missing = append(missing, "--source (or SOURCE_URL)")
	}
	if cfg.TargetURL == "" {
		missing = append(missing, "--target (or TARGET_URL)")
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	if cfg.LogFile == "" {
		cfg.LogFile = defaultLogFile
	}

	return cfg, nil
}

// envOr returns the environment value for key, or fallback when unset.
func envOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool interprets common truthy string values.
func envBool(v string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false
	}
	return b
}

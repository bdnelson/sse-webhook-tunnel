// Package logger provides a thin wrapper around zap configured to write
// structured logs to a file. A file sink is used because the TUI owns stdout
// and stderr; console logging would corrupt the display.
package logger

import (
	"fmt"

	"go.uber.org/zap"
)

// Logger is a structured logger writing JSON records to a file.
type Logger struct {
	handler *zap.SugaredLogger
}

// New constructs a Logger that writes to logFile. The file is created if it
// does not exist and appended to otherwise.
func New(logFile string) (Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{logFile}
	cfg.ErrorOutputPaths = []string{logFile}

	handler, err := cfg.Build()
	if err != nil {
		return Logger{}, fmt.Errorf("building logger: %w", err)
	}
	return Logger{handler: handler.Sugar()}, nil
}

// Info logs at info level with structured key-value pairs.
func (l Logger) Info(msg string, args ...any) {
	l.handler.Infow(msg, args...)
}

// Error logs at error level with structured key-value pairs.
func (l Logger) Error(msg string, args ...any) {
	l.handler.Errorw(msg, args...)
}

// Sync flushes buffered log entries. Call via defer at the creation site.
func (l Logger) Sync() error {
	return l.handler.Sync()
}

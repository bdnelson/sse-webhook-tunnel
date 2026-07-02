// Command sse-webhook-tunnel subscribes to a source URL over Server-Sent
// Events, replays each received event as an HTTP request to a target URL, and
// displays the inbound events in an interactive terminal UI. It behaves like
// the smee-client project.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bdnelson/sse-webhook-tunnel/app/tui"
	"github.com/bdnelson/sse-webhook-tunnel/app/tunnel"
	"github.com/bdnelson/sse-webhook-tunnel/core/forward"
	"github.com/bdnelson/sse-webhook-tunnel/core/sse"
	"github.com/bdnelson/sse-webhook-tunnel/lib/logger"
)

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run builds the components, wires graceful shutdown, and drives the TUI. It
// blocks until the UI exits or a signal is received.
func run(cfg config) error {
	log, err := logger.New(cfg.LogFile)
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Deriving the program's context from ctx means cancelling ctx also stops
	// the TUI, and unblocks Publisher.Send if it is mid-delivery.
	model := tui.New(cfg.SourceURL, cfg.TargetURL)
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen())

	client := sse.New(cfg.SourceURL, sse.WithLogger(log))
	forwarder := forward.New(cfg.Insecure)
	publisher := tui.NewPublisher(program)
	tn := tunnel.New(client, forwarder, publisher, cfg.TargetURL, tunnel.WithLogger(log))

	log.Info("starting tunnel", "source", cfg.SourceURL, "target", cfg.TargetURL)

	var wg sync.WaitGroup
	wg.Go(func() {
		if rerr := tn.Run(ctx); rerr != nil && !errors.Is(rerr, context.Canceled) {
			log.Error("tunnel stopped", "error", rerr)
		}
	})

	// Translate OS signals into a context cancellation, which stops both the
	// TUI (via WithContext) and the tunnel goroutine.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		select {
		case sig := <-sigChan:
			log.Info("received shutdown signal", "signal", sig.String())
			cancel()
		case <-ctx.Done():
		}
	}()

	_, runErr := program.Run()

	// The UI has exited (user quit, signal, or error); stop the tunnel and wait
	// for it to unwind.
	cancel()
	wg.Wait()
	signal.Stop(sigChan)

	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, tea.ErrProgramKilled) {
		return fmt.Errorf("running tui: %w", runErr)
	}
	return nil
}

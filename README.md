# SSE Webhook Tunnel

A terminal UI that tunnels webhook events from a Server-Sent Events (SSE) source
to an internal target URL, in the spirit of
[smee-client](https://github.com/probot/smee-client). It subscribes to a source
channel, replays each received event as an HTTP request to a target you control,
and displays the inbound events interactively so you can inspect their payloads.

## Table of Contents

- [How it works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Running](#running)
- [Key bindings](#key-bindings)
- [Development](#development)
- [Project structure](#project-structure)

## How it works

The tool opens an SSE connection to the source URL (for example a
`https://smee.io/<channel>` channel) and reads the event stream. Each event's
data is interpreted as a smee.io-style envelope: the top-level scalar keys are
treated as the original request headers, the `body` key is the request body, and
the `query` key contributes query-string parameters. The reconstructed request
is POSTed to the target URL. When the data is not a smee envelope (it is not a
JSON object, or it has no `body` key) the raw data is forwarded verbatim as a
JSON body.

If the body is a `payload` form wrapper — the shape GitHub sends when a webhook
is configured with the `application/x-www-form-urlencoded` content type,
i.e. `{"payload":"<escaped JSON>"}` — the inner JSON is unwrapped and forwarded
as the body with `Content-Type: application/json`. Bodies without such a wrapper
are forwarded unchanged.

Every event is shown in the TUI as a timestamped line
(`2026-07-02 13:35:52 Payload received`) with a forwarding-status indicator.
Selecting a line expands it to show the pretty-printed JSON payload. The list
scrolls and paginates as it fills; a status line reports uptime, the number of
events received, and the target URL.

## Prerequisites

- Go 1.26+
- A TTY (the UI runs in the alternate screen buffer)

## Configuration

Configuration is supplied by command-line flags, each with an environment
variable fallback. Flags take precedence over the environment.

| Flag | Environment | Default | Description |
|------|-------------|---------|-------------|
| `--source` | `SOURCE_URL` | (required) | SSE source URL to subscribe to |
| `--target` | `TARGET_URL` | (required) | URL each event is forwarded to |
| `--log-file` | `LOG_FILE` | `sse-webhook-tunnel.log` | Path to the structured log file |
| `--insecure` | `INSECURE` | `false` | Skip TLS verification for the target (self-signed internal certs) |

Logs are written to a file rather than the console because the TUI owns the
terminal. 

## Running

Build the binary and run it against a source and target:

```bash
make build
./sse-webhook-tunnel --source https://smee.io/your-channel --target http://localhost:9000/hook
```

Or with environment variables:

```bash
export SOURCE_URL=https://smee.io/your-channel
export TARGET_URL=http://localhost:9000/hook
./sse-webhook-tunnel
```

`make run` runs the tool via `go run ./api/cli`, but note it requires a TTY and
still needs `--source`/`--target` (or the environment equivalents) to be set.

A worked local example is in [docs/usage.md](docs/usage.md).

## Key bindings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move the selection |
| `←` / `→`, `pgup` / `pgdn` | Page backward / forward through the list |
| `enter` | Expand the selected event to view its JSON payload |
| `esc` / `backspace` | Return to the list from the detail view |
| `↑` / `↓`, `pgup` / `pgdn` | Scroll the payload in the detail view |
| `:q` then `enter` | Quit (type `:` to open the command line, then `q`) |
| `ctrl+c` | Force quit (emergency escape hatch) |

Quitting is deliberate — a single keystroke will not end the session. Type `:`
to open a command line at the bottom of the screen, then `q` and `enter`, in the
manner of vim.

## Development

```bash
make fmt      # Format code
make lint     # go vet + staticcheck
make test     # Run all tests with -race
make build    # Build the binary
make security # govulncheck + gosec
```

Run a single package's tests with `make test-pkg PKG=./core/sse`.

## Project structure

```
.
├── api/cli/            # Entry point: config, signal handling, wiring
├── app/
│   ├── tui/            # Bubble Tea UI (list, viewport, status line)
│   └── tunnel/         # Orchestrates SSE -> parse -> forward -> publish
├── core/
│   ├── event/          # Event model and smee payload parsing
│   ├── forward/        # HTTP forwarder to the target
│   └── sse/            # Server-Sent Events client (standard library)
├── lib/logger/         # zap wrapper writing to a file
└── docs/               # Supplementary documentation
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

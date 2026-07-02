# Go Microservice Standards

A prescriptive guide for building Go microservices. These standards are distilled from proven patterns in production services and cover project layout, configuration, testing, observability, and deployment.

## Table of Contents

1. [Project Layout](#1-project-layout)
2. [Go Module and Dependency Management](#2-go-module-and-dependency-management)
3. [Configuration](#3-configuration)
4. [Code Architecture Patterns](#4-code-architecture-patterns)
5. [Testing Standards](#5-testing-standards)
6. [Makefile](#6-makefile)
7. [Code Quality and Linting](#7-code-quality-and-linting)
8. [Docker](#8-docker)
9. [Local Development (Tilt/Kubernetes)](#9-local-development-tiltkubernetes)
10. [Observability](#10-observability)
11. [Documentation](#11-documentation)
12. [Security Practices](#12-security-practices)

---

## 1. Project Layout

Organize the project in layers that separate concerns. Each directory has a defined purpose:

```
.
├── api/                          # Entry points (main packages)
│   └── service/                  # Main deployable service
├── app/                          # Application services (routing, message handling, HTTP servers)
│   ├── consumer/                 # Message consumer
│   └── privateapi/               # Internal API (health, readiness)
├── core/                         # Business logic (domain rules, processing)
├── lib/                          # Shared libraries (thin wrappers, utilities)
│   ├── health/                   # Health check interface
│   ├── logger/                   # Structured logging wrapper
│   └── testutil/                 # Test utilities
├── vendor/                       # Vendored dependencies (committed)
├── zarf/                         # Deployment artifacts
│   ├── .env.sample               # Sample environment variables
│   ├── config.sample.yaml        # Sample configuration
│   ├── <service>.dockerfile      # Docker build
│   ├── Tiltfile                  # Tilt local dev configuration
│   └── README.md                 # Deployment documentation
├── docs/                         # Supplementary documentation
├── Makefile                      # Build and development automation
├── go.mod / go.sum               # Go module definition
├── AGENT.md                      # AI assistant instructions
├── CLAUDE.md                     # Claude Code bridge (references AGENT.md)
├── CONTRIBUTING.md               # Development guidelines
├── README.md                     # Project documentation
└── catalog-info.yaml             # Service catalog registration
```

### Layer Responsibilities

| Layer | Purpose | Dependencies |
|-------|---------|-------------|
| `api/` | Configuration, main entry point, signal handling, lifecycle | `app/`, `core/`, `lib/` |
| `app/` | Application services: HTTP servers, message consumers, routing | `core/`, `lib/` |
| `core/` | Business logic: domain rules, data transformation, processing | `lib/` |
| `lib/` | Shared utilities: logging, health interfaces, test helpers | Standard library, third-party |

### File Naming Conventions

- Test files are co-located with source: `consumer.go` and `consumer_test.go` in the same package.
- Dockerfile is named `<service-name>.dockerfile` and stored in `zarf/`.
- Sample configuration files use the `.sample` suffix: `config.sample.yaml`, `.env.sample`.

---

## 2. Go Module and Dependency Management

### Module Setup

Pin the Go version in `go.mod`:

```
module <service-name>

go 1.25.4
```

Use a short module name (the service name, not a full URL path) for internal services.

### Dependency Policy

Minimize external dependencies. Only add a new dependency when it provides significant value that would be costly to implement. Every dependency is a maintenance and security liability.

### Vendoring

Always vendor dependencies and commit the `vendor/` directory:

```bash
go mod tidy
go mod vendor
```

This ensures reproducible builds and eliminates external network dependencies during CI and Docker builds.

### Makefile Targets

```makefile
tidy: ## Tidy mod file
	go mod tidy

vendor: tidy ## Vendor dependencies
	go mod vendor

upgrade-deps: install-tools ## Upgrade dependencies
	go-mod-upgrade
	go mod tidy
	go mod vendor
	go test -race --count=1 $(TESTPATHS)
```

The `upgrade-deps` target upgrades, vendors, and runs all tests to verify nothing broke.

### Acceptable Dependencies

In addition to the totality of the [Golang Standard Library](https://pkg.go.dev/std), the following dependencies have been approved for use:

* [uber-go/zap](https://github.com/uber-go/zap) - Structured logging
* [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) - MCP Server/Client
* [nats-io/nats.go](https://github.com/nats-io/nats.go) - NATS connectivity
* [aws/aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) - AWS SDK
* [jackc/pgx](https://github.com/jackc/pgx) - PostgreSQL driver and toolkit
* [joho/godotenv](https://github.com/joho/godotenv) - dotenv port
* [spf13/viper](https://github.com/spf13/viper) - Go configuration library
* [go-sql-driver/mysql](https://github.com/go-sql-driver/MYSQL) - MySQL/MariaDB driver
* [elastic/go-elasticsearch](https://github.com/elastic/go-elasticsearch) - ElasticSearch client
* [golang-jwt/jwt](https://github.com/golang-jwt/jwt) - JWT signing/verification. Approved because hand-rolling JWS signature verification and claim validation (algorithm-confusion guards, `exp`/`nbf`/`aud`/`iss` checks) is security-sensitive and costly to get right - the dependency-policy carve-out for "significant value that would be costly to implement."

---

## 3. Configuration

### Layered Precedence

Configuration sources are loaded in this order (later overrides earlier):

1. **Hardcoded defaults** (via `viper.SetDefault`)
2. **Configuration file** (YAML, searched in standard paths)
3. **Environment variables** (bound via `viper.BindEnv`)
4. **`.env` files** (loaded via `godotenv` before Viper setup)

### Implementation

Use [Viper](https://github.com/spf13/viper) for configuration management and [godotenv](https://github.com/joho/godotenv) for `.env` file loading.

```go
func setupConfiguration() {
	loadEnvFile()
	setDefaults()
	bindEnvironmentVariables()
	setupConfigFile()
}
```

### Defaults

Define all defaults explicitly so the service runs with zero configuration:

```go
func setDefaults() {
	viper.SetDefault("server.health_port", 8081)
	viper.SetDefault("server.deployed", false)
	viper.SetDefault("nats.url", "nats://nats:4222")
}
```

If a value can not be safely defaulted, the service should error and exit if a value has not be specified in one of the configuration sources.

### Environment Variable Binding

Bind sensitive or deployment-specific values to environment variables:

```go
func bindEnvironmentVariables() {
	err := viper.BindEnv("nats.url", "NATS_URL")
	if err != nil {
		panic(fmt.Errorf("fatal error binding to environment variable: %w", err))
	}
}
```

### Config File Search Paths

Search multiple standard locations:

```go
viper.SetConfigName("config")
viper.AddConfigPath("/app")
viper.AddConfigPath("/etc/<service-name>/")
viper.AddConfigPath(".")
```

Handle missing config files gracefully (fall back to defaults), but panic on malformed files.

### `.env` File Loading

Load `.env` files from multiple locations, ignoring errors when files don't exist:

```go
_ = godotenv.Load(".env")
_ = godotenv.Load("/app/.env")
_ = godotenv.Load("/etc/<service-name>/.env")
```

### Configuration Structs

Group configuration into clear structs:

```go
type ApiConfig struct {
	Server ServerConfig
	Nats   NatsConfig
	Slack  SlackConfig
}

type ServerConfig struct {
	HealthPort int
	Deployed   bool
}
```

Gather configuration into these structs from Viper at startup:

```go
func gatherConfiguration() ApiConfig {
	return ApiConfig{
		Server: ServerConfig{
			HealthPort: viper.GetInt("server.health_port"),
			Deployed:   viper.GetBool("server.deployed"),
		},
		// ...
	}
}
```

### Sample Files

Provide `zarf/config.sample.yaml` documenting all options with their defaults and comments explaining each field. Provide `zarf/.env.sample` showing all supported environment variables.

### Secrets

Never store secrets in checked-in config files. Use environment variables or `.env` files (excluded via `.gitignore`) for credentials.

---

## 4. Code Architecture Patterns

### Interface-Based Dependency Injection

Define interfaces for all external boundaries (message brokers, HTTP clients, health checks). This enables testing without real infrastructure.

```go
// Define the interface for what you need
type NatsConnection interface {
	Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
	Close()
	SetDisconnectErrHandler(handler nats.ConnErrHandler)
	SetReconnectHandler(handler nats.ConnHandler)
}

// Wrap the real implementation
type realNatsConnection struct {
	conn *nats.Conn
}
```

```go
// HTTP client interface for external calls
type HTTPClient interface {
	Post(url string, contentType string, body io.Reader, headers map[string]string) (*http.Response, error)
}
```

### Compile-Time Interface Compliance

Verify interface implementations at compile time with blank variable declarations:

```go
var _ NatsConnection = (*realNatsConnection)(nil)
var _ health.Checker = (*Consumer)(nil)
var _ HTTPClient = (*DefaultHTTPClient)(nil)
```

Place these immediately after the struct definition or at the top of the file.

### Health Check Interface

Define a minimal health check interface in `lib/health/`:

```go
package health

type Checker interface {
	IsHealthy() bool
}
```

Components that manage connections implement this interface, allowing the health endpoint to aggregate status.

### Graceful Shutdown

Handle OS signals for graceful shutdown using context, WaitGroup, and signal channels:

```go
// Create context for coordinating shutdown
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Set up signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)

// Start services in goroutines with WaitGroup
var wg sync.WaitGroup

wg.Add(1)
go func() {
	defer wg.Done()
	if err := consumer.Start(); err != nil {
		errChan <- err
	}
}()

// Wait for shutdown signal or error
select {
case sig := <-sigChan:
	log.Info("received shutdown signal", "signal", sig.String())
case err := <-errChan:
	log.Error("received error from service", "error", err)
case <-ctx.Done():
	log.Info("context cancelled")
}

// Orderly shutdown: stop, close, wait
cancel()
wg.Wait()
```

### Thread-Safe State Management

Use `sync.RWMutex` for state that is read frequently and written infrequently:

```go
type Consumer struct {
	mu      sync.RWMutex
	healthy bool
}

func (c *Consumer) updateHealthStatus(healthy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = healthy
}

func (c *Consumer) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}
```

### Thin Library Wrappers

Wrap third-party libraries in `lib/` to provide a consistent, simplified interface:

```go
// lib/logger/logger.go - wraps zap
type Logger struct {
	production bool
	handler    *zap.SugaredLogger
}

func New(production bool) (Logger, error) {
	var l *zap.Logger
	if production {
		l, _ = zap.NewProduction()
	} else {
		l, _ = zap.NewDevelopment()
	}
	return Logger{production, l.Sugar()}, nil
}

func (log *Logger) Info(msg string, args ...any) {
	log.handler.Infow(msg, args...)
}
```

### Health and Readiness Endpoints

Run health/readiness endpoints on a dedicated port, separate from any public-facing API:

```go
healthMux := http.NewServeMux()
healthMux.HandleFunc("GET /health", s.handleHealth)
healthMux.HandleFunc("GET /ready", s.handleReady)

s.healthServer = &http.Server{
	Addr:         ":" + strconv.Itoa(config.HealthPort),
	Handler:      healthMux,
	ReadTimeout:  10 * time.Second,
	WriteTimeout: 10 * time.Second,
}
```

- `/health` returns JSON with status and reason on failure: `{"status":"healthy"}` or `{"status":"unhealthy","reason":"NATS disconnected"}`
- `/ready` returns `204 No Content` when the server is running.

---

## 5. Testing Standards

### Co-located Test Files

Place `_test.go` files alongside the source code they test, in the same package. Every package with production code has tests.

### Table-Driven Tests

Use table-driven tests as the default pattern:

```go
func TestGenerateMessage_Success(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		expected string
	}{
		{
			name:     "simple string",
			template: "Hello World",
			data:     map[string]interface{}{},
			expected: "Hello World",
		},
		{
			name:     "single variable",
			template: "Hello {{.Name}}",
			data:     map[string]interface{}{"Name": "Alice"},
			expected: "Hello Alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test body
		})
	}
}
```

### Hand-Written Mocks

Write mock implementations by hand using interface function fields -- no code generation tools. This keeps mocks explicit and readable.

```go
type mockNatsConnection struct {
	subscribeFunc           func(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
	closeFunc               func()
	setDisconnectErrHandler func(handler nats.ConnErrHandler)
	setReconnectHandler     func(handler nats.ConnHandler)
}

// Compile-time interface verification
var _ NatsConnection = (*mockNatsConnection)(nil)

func (m *mockNatsConnection) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if m.subscribeFunc != nil {
		return m.subscribeFunc(subject, handler)
	}
	return &nats.Subscription{}, nil
}

func (m *mockNatsConnection) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}
```

This pattern allows each test to inject specific behavior while providing sensible defaults.

### Compile-Time Mock Verification

Verify mocks implement their interfaces at compile time:

```go
var _ NatsConnection = (*mockNatsConnection)(nil)
var _ health.Checker = (*mockHealthChecker)(nil)
var _ HTTPClient = (*mockHTTPClient)(nil)
```

### Test Utilities (`lib/testutil/`)

Provide shared test helpers for common concerns:

**Configuration isolation** -- reset Viper state between tests:

```go
func SetupTestConfig(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})
}
```

**Environment variable management** -- set/restore env vars:

```go
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	oldValue, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, oldValue)
		} else {
			os.Unsetenv(key)
		}
	})
}
```

**Temporary config files** -- create and auto-clean temp files:

```go
func CreateTempConfigFile(t *testing.T, filename, content string) string {
	t.Helper()
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	return tmpDir
}
```

**Log capture** -- assert on log output using zap's observer:

```go
type LogCapture struct {
	Core     zapcore.Core
	Observed *observer.ObservedLogs
	Logger   *zap.Logger
}

func NewLogCapture() *LogCapture {
	core, observed := observer.New(zapcore.DebugLevel)
	return &LogCapture{Core: core, Observed: observed, Logger: zap.New(core)}
}

func (lc *LogCapture) AssertLogContains(t *testing.T, expected string) { /* ... */ }
```

### Integration Tests

Use `t.Skip()` for tests that require real infrastructure:

```go
func TestNewConsumer_Success(t *testing.T) {
	t.Skip("Skipping integration test - requires NATS server")
	// ...
}
```

### Race Condition Tests

Write explicit concurrency tests and always run tests with `-race`:

```go
func TestConsumer_ConcurrentHealthChecks(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = consumer.IsHealthy()
		}()
	}
	wg.Wait()
}
```

### HTTP Handler Testing

Use `net/http/httptest` for testing HTTP handlers:

```go
func TestHandleHealth_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	service.handleHealth(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}
```

### Test Execution

All tests run with `-race` flag. The Makefile provides targets for different test execution patterns:

```makefile
TESTPATHS := $(shell go list ./... | grep -v lib/testutil)

test:            ## Run tests
	go test -race $(TESTPATHS)

test-force:      ## Clear test cache and run tests
	go test -race --count=1 $(TESTPATHS)

test-pkg:        ## Run tests for a specific package
	go test -v $(PKG) -run $(PATTERN)

test-single:     ## Run a single test
	go test -v -run $(TEST)
```

Exclude utility-only packages (like `lib/testutil`) from test paths since they contain no tests.

### Test Data

If a testable part of the application needs data, then test data should be generated based on the schema of the data.  No historical data will be provided to the application.

---

## 6. Makefile

The Makefile is the central automation hub. It provides a consistent interface for all development tasks.

### Required Targets

| Target | Purpose |
|--------|---------|
| `help` | Show all available targets with descriptions |
| `build` | Build the service binary |
| `run` | Run the service locally |
| `clean` | Remove build artifacts |
| `test` | Run all tests with `-race` |
| `test-force` | Run tests bypassing cache |
| `test-coverage` | Run tests with coverage reporting |
| `test-pkg` | Run tests for a specific package |
| `test-single` | Run a single test by name |
| `fmt` | Format all Go code |
| `lint` | Run vet + staticcheck |
| `security` | Run govulncheck + gosec |
| `tidy` | Run `go mod tidy` |
| `vendor` | Tidy and vendor dependencies |
| `upgrade-deps` | Upgrade, vendor, and test dependencies |
| `install-tools` | Install required development tools |
| `docker-build` | Build Docker image |
| `docker-run` | Run Docker container |
| `commit` | Entry point for pre-commit hook |
| `push` | Entry point for pre-push hook (lint + test) |

### Conventions

- Set `.DEFAULT_GOAL := help` so running bare `make` shows available targets.
- Add `## Description` comments after target names for auto-generated help output.
- Define `TESTPATHS` to exclude utility-only packages.
- The `push` target depends on `lint` and `test` to enforce quality before push.

```makefile
.DEFAULT_GOAL := help

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

push: lint test
	@echo "Validating changeset before push..."
```

---

## 7. Code Quality and Linting

### Tools

| Tool | Purpose | Target |
|------|---------|--------|
| `go fmt` | Code formatting | `make fmt` |
| `go vet` | Suspicious constructs | `make lint` |
| `staticcheck` | Advanced static analysis | `make lint` |
| `govulncheck` | Known vulnerability detection | `make security` |
| `gosec` | Security-focused analysis | `make security` |

### Lint Target

```makefile
lint: ## Lint code
	go vet $(ALLGO)
	staticcheck $(ALLGO)
```

### Security Target

```makefile
security: ## Security check code
	govulncheck $(ALLGO)
	gosec $(ALLGO)
```

### Tool Installation

```makefile
install-tools: ## Install tools
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
```

---

## 8. Docker

### Multi-Stage Alpine Build

Store the Dockerfile in `zarf/<service-name>.dockerfile`.

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum vendor ./
COPY . .

# Static binary, stripped debug info
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-w -s" -o <service-name> ./api/service

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

# Non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/<service-name> .
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ="UTC"

RUN chown -R appuser:appuser /app
USER appuser

EXPOSE 8080

CMD ["/app/<service-name>"]
```

### Key Principles

- **Static binary**: `CGO_ENABLED=0` produces a self-contained binary.
- **Stripped**: `-ldflags="-w -s"` removes debug info to reduce image size.
- **Non-root**: Create and switch to a dedicated `appuser`.
- **Minimal runtime**: Alpine base with only `ca-certificates` and `tzdata`.

### `.dockerignore`

Exclude files not needed in the build context:

```
.git/
.gitignore
.vscode/
.idea/
README.md
CLAUDE.md
docs/
config.yaml
*.test
coverage.*
```

---

## 9. Local Development (Tilt/Kubernetes)

### Tiltfile

Store the Tiltfile in `zarf/Tiltfile`. It defines the local Kubernetes development stack.

### Structure

1. **Infrastructure dependencies** as K8s deployments (e.g., NATS server).
2. **ConfigMap** with service configuration mounted into the container.
3. **Service deployment** with the Docker image built from source.
4. **Liveness/readiness probes** matching the health endpoints.
5. **Manual trigger mode** for service rebuilds (avoids constant rebuilds during active development).

### Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8081
  initialDelaySeconds: 10
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /ready
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Manual Trigger Mode

Use `TRIGGER_MODE_MANUAL` so service rebuilds only happen when explicitly triggered:

```python
k8s_resource('service-name',
    resource_deps=['nats'],
    trigger_mode=TRIGGER_MODE_MANUAL
)
```

Trigger a rebuild via:

```bash
make tilt-trigger
```

### Makefile Targets

```makefile
tilt-infra-only: ## Start supporting infrastructure only
	tilt up nats --file=zarf/Tiltfile

tilt-up: ## Start full Tilt stack
	tilt up --file=zarf/Tiltfile

tilt-down: ## Stop Tilt
	tilt down --file=zarf/Tiltfile

tilt-trigger: ## Trigger service rebuild
	tilt trigger $(BIN_NAME)
```

---

## 10. Observability

### Structured Logging

Use [zap](https://github.com/uber-go/zap) wrapped in a thin library in `lib/logger/`:

```go
type Logger struct {
	production bool
	handler    *zap.SugaredLogger
}

func New(production bool) (Logger, error) {
	if production {
		l, err = zap.NewProduction()  // JSON output
	} else {
		l, err = zap.NewDevelopment() // Console output
	}
	return Logger{production, l.Sugar()}, nil
}
```

- **Production** (`deployed: true`): JSON structured output for log aggregation.
- **Development** (`deployed: false`): Human-readable console output.

Use key-value pairs for structured context:

```go
log.Info("starting http service", "port", config.Server.HealthPort)
log.Error("consumer error", "error", err)
```

Always defer `log.Sync()` at the logger's creation site.

### Health Endpoint

The `/health` endpoint returns JSON indicating service status:

```json
{"status":"healthy"}
```

Or on failure:

```json
{"status":"unhealthy","reason":"NATS disconnected"}
```

Return HTTP 200 for healthy, HTTP 503 for unhealthy.

### Readiness Endpoint

The `/ready` endpoint returns `204 No Content` when the server is running and able to accept traffic.

---

## 11. Documentation

### Required Files

| File | Purpose |
|------|---------|
| `README.md` | Project documentation: prerequisites, configuration, running, development workflow, troubleshooting, project structure |
| `CONTRIBUTING.md` | Development guidelines: language, Makefile usage, testing policy, dependency management, code style |
| `AGENT.md` | AI coding assistant instructions. References `CONTRIBUTING.md` and adds agent-specific preferences |
| `CLAUDE.md` | Claude Code bridge file. References `AGENT.md` via `@AGENT.md` |
| `zarf/README.md` | Deployment artifact index |
| `catalog-info.yaml` | Service catalog registration (Backstage format) |
| `zarf/openapi.yaml` | [OpenAPI](https://spec.openapis.org/oas/latest.html) specfication describing the APIs provided |
| `zarf/asyncapi.yaml` | [AsyncAPI](https://www.asyncapi.com/docs) specification describing the asynchronous events supported |

### README.md Structure

1. Service name and description
2. Table of contents
3. Prerequisites
4. Configuration (layered config, config file, env vars, `.env` files, precedence)
5. Running the service (local, Docker, Tilt)
6. Development (setup, building, testing, code quality, workflow)
7. Common issues / troubleshooting
8. Project structure (directory tree with descriptions)
9. Additional resources (links to CONTRIBUTING.md, sample configs)

### CONTRIBUTING.md Content

- Language: Go
- Makefile: define repetitive actions as targets
- Testing: write unit tests when adding or changing functionality
- Documentation: README.md for general docs, `docs/` for supplementary
- Dependencies: minimize external deps, always vendor
- Workflow: project must build and all tests must pass before finishing work
- Code style: `make fmt` and `make lint`

### AGENT.md Content

References `CONTRIBUTING.md` via `@CONTRIBUTING.md` and adds:

- Prefer running single tests over the whole suite
- Use Makefile targets for repetitive tasks
- Keep documentation up to date as changes are made

### catalog-info.yaml

Register the service in Backstage:

```yaml
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: <service-name>
  annotations:
    github.com/project-slug: <org>/<service-name>
spec:
  type: other
  lifecycle: unknown
  owner: <team>
```

---

## 12. Security Practices

### Secrets Management

- `.gitignore` excludes `.env` and `config.yaml` -- secrets never enter version control.
- Sensitive values (webhook URLs, auth tokens) are provided via environment variables.
- Sample files (`zarf/.env.sample`, `zarf/config.sample.yaml`) contain placeholder values only.

### Container Security

- Non-root user (`appuser`) in the runtime Docker image.
- Static binary compilation (`CGO_ENABLED=0`) -- no shared library dependencies.
- Minimal Alpine base image with only `ca-certificates` and `tzdata`.

### Static Analysis

Run security scanning as part of the development workflow:

```makefile
security: ## Security check code
	govulncheck $(ALLGO)
	gosec $(ALLGO)
```

- `govulncheck` detects known vulnerabilities in dependencies.
- `gosec` detects common security issues in Go source code.

### HTTP Client Practices

- Drain and close response bodies to prevent resource leaks.
- Check HTTP status codes and surface errors with context.

```go
defer func() {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}()

if resp.StatusCode != http.StatusOK {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
}
```

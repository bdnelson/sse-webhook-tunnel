# Default target
.DEFAULT_GOAL := help

# Output
PROJECT_NAME := sse-webhook-tunnel
BIN_NAME := sse-webhook-tunnel
TEST_BIN_NAME := sse_webhook_tunnel

# Release build parameters (override for cross-compilation)
RELEASE_GOOS ?= $(shell go env GOOS)
RELEASE_GOARCH ?= $(shell go env GOARCH)
RELEASE_OUTPUT ?= $(BIN_NAME)

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

# Paths
GOPATH := $(shell go env GOPATH)
STATICCHECK_BIN := $(GOPATH)/bin/staticcheck
YQ_BIN := $(GOPATH)/bin/yq
GO_MOD_UPGRADE_BIN := $(GOPATH)/bin/go-mod-upgrade
GO_VUL_CHECK_BIN := $(GOPATH)/bin/govulncheck
GO_SEC_BIN := $(GOPATH)/bin/gosec
TESTPATHS := $(shell go list ./... | grep -v lib/testutil)
ALLGO := $(shell go list ./...)

help: ## Show this help message
	@echo "$(BLUE)$(PROJECT_NAME) - Makefile Commands$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

install-tools: ## Install tools
	git config core.hooksPath './.githooks'
	go install golang.org/x/tools/gopls@latest
	go install github.com/go-delve/delve/cmd/dlv@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/oligot/go-mod-upgrade@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/mikefarah/yq/v4@latest
	go install github.com/antonmedv/fx@latest
	go install github.com/charmbracelet/glow/v2@latest
	cargo install csvlens
	cargo install presenterm

run: ## Run the project
	go run ./api/cli

build: ## Build the project
	go build -o $(BIN_NAME) ./api/cli

build-release: ## Build a static, stripped binary (set RELEASE_GOOS/RELEASE_GOARCH/RELEASE_OUTPUT)
	CGO_ENABLED=0 GOOS=$(RELEASE_GOOS) GOARCH=$(RELEASE_GOARCH) \
		go build -buildvcs=false -ldflags="-w -s" -o $(RELEASE_OUTPUT) ./api/cli

clean: ## Clean outputs
	rm -f $(BIN_NAME)
	rm -f go_build_$(TEST_BIN_NAME)_api_cli

tidy: ## Tidy mod file
	go mod tidy

vendor: tidy ## Vendor dependencies
	go mod vendor

upgrade-deps: install-tools ## Upgrade dependencies
	go-mod-upgrade
	go mod tidy
	go mod vendor
	go test -race --count=1 $(TESTPATHS)

test: ## Run tests
	go test -race $(TESTPATHS)

test-force: ## Clear test cache and run tests
	go test -race --count=1 $(TESTPATHS)

test-coverage: ## Run tests with coverage
	go test --count=1 $(TESTPATHS) -cover

test-single: ## Run a single test
	@echo "Running single test: $(TEST)"
	@go test -v -run $(TEST)

test-pkg: ## Run tests for a specific package (usage: make test-pkg PKG=./core/datetime PATTERN='TestHandler')
	@if [ -z "$(PKG)" ]; then \
		echo "Error: PKG is required. Usage: make test-pkg PKG=./core/datetime [PATTERN=TestPattern]"; \
		exit 1; \
	fi
	@if [ -n "$(PATTERN)" ]; then \
		echo "Running tests matching '$(PATTERN)' in package $(PKG)..."; \
		go test -v $(PKG) -run $(PATTERN); \
	else \
		echo "Running all tests in package $(PKG)..."; \
		go test -v $(PKG); \
	fi

test-integration: ## Run integration tests (requires DSN env var pointing at a running database)
	@echo "Running integration tests..."
	DSN=$(DSN) go test -v -count=1 -run Integration $(TESTPATHS)

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt $(ALLGO)

lint: ## Lint code
	go vet $(ALLGO)
	@echo
	staticcheck $(ALLGO)

security: ## Security check code
	govulncheck $(ALLGO)
	@echo
	gosec $(ALLGO)

todo: ## Display all todo markers
	@PAGER=cat git grep \T\O\D\O -- :^vendor

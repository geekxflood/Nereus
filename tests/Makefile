# Nereus Testing Makefile
# Provides convenient targets for running various test suites

.PHONY: help test test-all test-unit test-integration test-e2e test-performance test-security test-chaos
.PHONY: test-coverage test-benchmarks test-setup test-clean test-ci test-watch
.PHONY: lint fmt vet security-scan deps-check

# Default target
.DEFAULT_GOAL := help

# Configuration
PROJECT_ROOT := $(shell cd .. && pwd)
TEST_DIR := $(CURDIR)
SCRIPTS_DIR := $(TEST_DIR)/scripts
REPORTS_DIR := $(PROJECT_ROOT)/test-reports
GO_FILES := $(shell find $(PROJECT_ROOT) -name "*.go" -not -path "*/vendor/*" -not -path "*/test-reports/*")

# Test configuration
TEST_TIMEOUT ?= 300s
TEST_PARALLEL ?= 4
TEST_VERBOSE ?= false
TEST_SHORT ?= false
ENABLE_COVERAGE ?= true

# Colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m

# Helper function to print colored output
define print_status
	@echo -e "$(1)[$(shell date '+%Y-%m-%d %H:%M:%S')] $(2)$(NC)"
endef

define print_info
	$(call print_status,$(BLUE),INFO: $(1))
endef

define print_success
	$(call print_status,$(GREEN),SUCCESS: $(1))
endef

define print_warning
	$(call print_status,$(YELLOW),WARNING: $(1))
endef

define print_error
	$(call print_status,$(RED),ERROR: $(1))
endef

## help: Show this help message
help:
	@echo "Nereus Testing Makefile"
	@echo ""
	@echo "Available targets:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo ""
	@echo "Configuration:"
	@echo "  TEST_TIMEOUT=$(TEST_TIMEOUT)    - Test timeout"
	@echo "  TEST_PARALLEL=$(TEST_PARALLEL)   - Parallel test processes"
	@echo "  TEST_VERBOSE=$(TEST_VERBOSE)    - Verbose output"
	@echo "  TEST_SHORT=$(TEST_SHORT)      - Short mode (skip long tests)"
	@echo "  ENABLE_COVERAGE=$(ENABLE_COVERAGE) - Coverage collection"
	@echo ""
	@echo "Examples:"
	@echo "  make test                    # Run all tests"
	@echo "  make test-unit               # Run unit tests only"
	@echo "  make test-e2e TEST_VERBOSE=true  # Run e2e tests with verbose output"
	@echo "  make test-performance TEST_SHORT=true  # Run performance tests in short mode"

## test: Run all test suites
test: test-all

## test-all: Run comprehensive test suite
test-all:
	$(call print_info,Running comprehensive test suite)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh all

## test-unit: Run unit tests
test-unit:
	$(call print_info,Running unit tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh unit

## test-integration: Run integration tests
test-integration:
	$(call print_info,Running integration tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh integration

## test-e2e: Run end-to-end tests
test-e2e:
	$(call print_info,Running end-to-end tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh e2e

## test-performance: Run performance tests
test-performance:
	$(call print_info,Running performance tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh performance

## test-security: Run security tests
test-security:
	$(call print_info,Running security tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh security

## test-chaos: Run chaos engineering tests
test-chaos:
	$(call print_info,Running chaos engineering tests)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh chaos

## test-benchmarks: Run performance benchmarks
test-benchmarks:
	$(call print_info,Running performance benchmarks)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh benchmarks

## test-coverage: Generate coverage report
test-coverage:
	$(call print_info,Generating coverage report)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh coverage

## test-setup: Setup test environment
test-setup:
	$(call print_info,Setting up test environment)
	@mkdir -p $(REPORTS_DIR)/coverage $(REPORTS_DIR)/logs
	@go mod download
	@go mod tidy
	$(call print_success,Test environment ready)

## test-clean: Clean test artifacts
test-clean:
	$(call print_info,Cleaning test artifacts)
	@chmod +x $(SCRIPTS_DIR)/run_tests.sh
	@$(SCRIPTS_DIR)/run_tests.sh clean
	@rm -rf $(REPORTS_DIR)
	@rm -rf $(PROJECT_ROOT)/test-data/nereus-test*.db
	@rm -rf $(PROJECT_ROOT)/test-temp-*
	$(call print_success,Test artifacts cleaned)

## test-ci: Run tests in CI mode
test-ci: test-setup lint vet security-scan
	$(call print_info,Running tests in CI mode)
	@TEST_VERBOSE=false TEST_SHORT=false ENABLE_COVERAGE=true $(MAKE) test-all
	@$(MAKE) test-coverage

## test-watch: Watch for changes and run tests
test-watch:
	$(call print_info,Watching for changes...)
	@which fswatch >/dev/null 2>&1 || (echo "fswatch not found. Install with: brew install fswatch" && exit 1)
	@fswatch -o $(PROJECT_ROOT) --exclude="test-reports" --exclude="*.db" | while read num; do \
		echo ""; \
		$(call print_info,Files changed, running tests...); \
		$(MAKE) test-unit || true; \
	done

## lint: Run linter
lint:
	$(call print_info,Running linter)
	@which golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/" && exit 1)
	@cd $(PROJECT_ROOT) && golangci-lint run ./...
	$(call print_success,Linting completed)

## fmt: Format Go code
fmt:
	$(call print_info,Formatting Go code)
	@cd $(PROJECT_ROOT) && go fmt ./...
	$(call print_success,Code formatted)

## vet: Run go vet
vet:
	$(call print_info,Running go vet)
	@cd $(PROJECT_ROOT) && go vet ./...
	$(call print_success,Vet completed)

## security-scan: Run security scanner
security-scan:
	$(call print_info,Running security scan)
	@which gosec >/dev/null 2>&1 || (echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest" && exit 1)
	@cd $(PROJECT_ROOT) && gosec -quiet ./...
	$(call print_success,Security scan completed)

## deps-check: Check for dependency vulnerabilities
deps-check:
	$(call print_info,Checking dependencies for vulnerabilities)
	@which govulncheck >/dev/null 2>&1 || (echo "govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest" && exit 1)
	@cd $(PROJECT_ROOT) && govulncheck ./...
	$(call print_success,Dependency check completed)

## test-quick: Run quick tests (unit + integration)
test-quick: test-unit test-integration

## test-full: Run full test suite including chaos and performance
test-full: test-all test-benchmarks

## test-smoke: Run smoke tests (basic functionality)
test-smoke:
	$(call print_info,Running smoke tests)
	@cd $(PROJECT_ROOT) && go test -short -timeout=60s ./tests/e2e/... -run TestCompleteWorkflow
	$(call print_success,Smoke tests completed)

## test-regression: Run regression test suite
test-regression: test-unit test-integration test-e2e

## test-nightly: Run comprehensive nightly test suite
test-nightly: test-setup lint vet security-scan deps-check test-full

## build-test: Build test binaries
build-test:
	$(call print_info,Building test binaries)
	@cd $(PROJECT_ROOT) && go test -c ./tests/e2e/... -o $(REPORTS_DIR)/e2e.test
	@cd $(PROJECT_ROOT) && go test -c ./tests/integration/... -o $(REPORTS_DIR)/integration.test
	@cd $(PROJECT_ROOT) && go test -c ./tests/performance/... -o $(REPORTS_DIR)/performance.test
	$(call print_success,Test binaries built)

## test-race: Run tests with race detection
test-race:
	$(call print_info,Running tests with race detection)
	@cd $(PROJECT_ROOT) && go test -race -timeout=$(TEST_TIMEOUT) ./...
	$(call print_success,Race detection tests completed)

## test-memory: Run tests with memory profiling
test-memory:
	$(call print_info,Running tests with memory profiling)
	@mkdir -p $(REPORTS_DIR)/profiles
	@cd $(PROJECT_ROOT) && go test -memprofile=$(REPORTS_DIR)/profiles/mem.prof -timeout=$(TEST_TIMEOUT) ./tests/performance/...
	$(call print_success,Memory profiling completed)

## test-cpu: Run tests with CPU profiling
test-cpu:
	$(call print_info,Running tests with CPU profiling)
	@mkdir -p $(REPORTS_DIR)/profiles
	@cd $(PROJECT_ROOT) && go test -cpuprofile=$(REPORTS_DIR)/profiles/cpu.prof -timeout=$(TEST_TIMEOUT) ./tests/performance/...
	$(call print_success,CPU profiling completed)

## test-profile: Run tests with full profiling
test-profile: test-memory test-cpu
	$(call print_info,Profiling reports available in $(REPORTS_DIR)/profiles/)

## test-docker: Run tests in Docker container
test-docker:
	$(call print_info,Running tests in Docker container)
	@docker build -t nereus-test -f $(TEST_DIR)/Dockerfile.test $(PROJECT_ROOT)
	@docker run --rm -v $(REPORTS_DIR):/reports nereus-test
	$(call print_success,Docker tests completed)

## install-tools: Install required testing tools
install-tools:
	$(call print_info,Installing testing tools)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/onsi/ginkgo/v2/ginkgo@latest
	$(call print_success,Testing tools installed)

## check-tools: Check if required tools are installed
check-tools:
	$(call print_info,Checking required tools)
	@which go >/dev/null 2>&1 || (echo "Go not found" && exit 1)
	@which golangci-lint >/dev/null 2>&1 || echo "golangci-lint not found (run: make install-tools)"
	@which gosec >/dev/null 2>&1 || echo "gosec not found (run: make install-tools)"
	@which govulncheck >/dev/null 2>&1 || echo "govulncheck not found (run: make install-tools)"
	$(call print_success,Tool check completed)

# Conditional targets based on environment
ifeq ($(CI),true)
test: test-ci
endif

# File watching for development
$(REPORTS_DIR):
	@mkdir -p $(REPORTS_DIR)/coverage $(REPORTS_DIR)/logs $(REPORTS_DIR)/profiles

# Dependencies
test-all test-unit test-integration test-e2e test-performance test-security test-chaos: $(REPORTS_DIR)
test-benchmarks test-coverage: $(REPORTS_DIR)

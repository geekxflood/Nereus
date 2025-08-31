# Nereus SNMP Trap Alerting System - Build System
# This Makefile provides comprehensive build, test, and deployment automation

# Project information
PROJECT_NAME := nereus
BINARY_NAME := nereus
PACKAGE := github.com/geekxflood/nereus

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d%H%M%S)
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags as specified in requirements
LDFLAGS := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)
BUILD_FLAGS := -ldflags "$(LDFLAGS)"

# Directories
BUILD_DIR := build
DIST_DIR := dist
DOCS_DIR := docs
TEST_DIR := test

# Go configuration
GO := go
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
CGO_ENABLED := 0

# Docker configuration
DOCKER_IMAGE := geekxflood/nereus
DOCKER_TAG := $(VERSION)
DOCKERFILE := Dockerfile

# Tools
GOLANGCI_LINT := golangci-lint
GORELEASER := goreleaser

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: all build clean test lint fmt vet deps check coverage
.PHONY: build-all build-linux build-darwin build-windows
.PHONY: docker docker-build docker-push docker-run
.PHONY: release release-snapshot release-publish
.PHONY: install uninstall
.PHONY: docs docs-serve
.PHONY: integration-test load-test
.PHONY: help

# Default target
all: clean deps check test build

## Build targets

# Build for current platform
build:
	@echo "$(BLUE)Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./main.go
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Build for all supported platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "$(BLUE)Building for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)/linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./main.go
	@mkdir -p $(BUILD_DIR)/linux-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) ./main.go

build-darwin:
	@echo "$(BLUE)Building for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)/darwin-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/darwin-amd64/$(BINARY_NAME) ./main.go
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) ./main.go

build-windows:
	@echo "$(BLUE)Building for Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)/windows-amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe ./main.go

## Test targets

# Run unit tests
test:
	@echo "$(BLUE)Running unit tests...$(NC)"
	$(GO) test -v -race ./internal/...
	@echo "$(GREEN)Unit tests completed$(NC)"

# Run tests with coverage
coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) test -v -race -coverprofile=$(BUILD_DIR)/coverage.out ./internal/...
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	$(GO) tool cover -func=$(BUILD_DIR)/coverage.out
	@echo "$(GREEN)Coverage report generated: $(BUILD_DIR)/coverage.html$(NC)"

# Run integration tests
integration-test:
	@echo "$(BLUE)Running integration tests...$(NC)"
	$(GO) test -v -tags=integration ./test/integration/...
	@echo "$(GREEN)Integration tests completed$(NC)"

# Run load tests
load-test:
	@echo "$(BLUE)Running load tests...$(NC)"
	$(GO) test -v -tags=load -timeout=10m ./test/load/...
	@echo "$(GREEN)Load tests completed$(NC)"

# Run all tests
test-all: test integration-test load-test

## Code quality targets

# Run linter
lint:
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run; \
	else \
		echo "$(YELLOW)golangci-lint not found, skipping...$(NC)"; \
	fi

# Format code
fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)Code formatted$(NC)"

# Run go vet
vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)Vet completed$(NC)"

# Run all checks
check: fmt vet lint

## Dependency management

# Download dependencies
deps:
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

# Update dependencies
deps-update:
	@echo "$(BLUE)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

## Docker targets

# Build Docker image
docker-build:
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

# Push Docker image
docker-push:
	@echo "$(BLUE)Pushing Docker image...$(NC)"
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image pushed$(NC)"

# Run Docker container
docker-run:
	@echo "$(BLUE)Running Docker container...$(NC)"
	docker run --rm -it \
		-p 162:162/udp \
		-p 9090:9090 \
		-v $(PWD)/examples:/etc/nereus:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Docker compose up
docker-compose-up:
	@echo "$(BLUE)Starting with Docker Compose...$(NC)"
	docker-compose up -d

# Docker compose down
docker-compose-down:
	@echo "$(BLUE)Stopping Docker Compose...$(NC)"
	docker-compose down

## Release targets

# Create snapshot release
release-snapshot:
	@echo "$(BLUE)Creating snapshot release...$(NC)"
	@if command -v $(GORELEASER) >/dev/null 2>&1; then \
		$(GORELEASER) release --snapshot --rm-dist; \
	else \
		echo "$(YELLOW)goreleaser not found, creating manual release...$(NC)"; \
		$(MAKE) build-all; \
		$(MAKE) package; \
	fi

# Create and publish release
release-publish:
	@echo "$(BLUE)Publishing release...$(NC)"
	@if command -v $(GORELEASER) >/dev/null 2>&1; then \
		$(GORELEASER) release --rm-dist; \
	else \
		echo "$(RED)goreleaser required for publishing releases$(NC)"; \
		exit 1; \
	fi

# Package binaries
package:
	@echo "$(BLUE)Packaging binaries...$(NC)"
	@mkdir -p $(DIST_DIR)
	@for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64; do \
		echo "Packaging $$platform..."; \
		if [ "$$platform" = "windows-amd64" ]; then \
			tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$$platform.tar.gz -C $(BUILD_DIR)/$$platform $(BINARY_NAME).exe; \
		else \
			tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$$platform.tar.gz -C $(BUILD_DIR)/$$platform $(BINARY_NAME); \
		fi; \
	done
	@echo "$(GREEN)Packaging completed$(NC)"

## Installation targets

# Install binary to system
install: build
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)$(BINARY_NAME) installed to /usr/local/bin/$(NC)"

# Uninstall binary from system
uninstall:
	@echo "$(BLUE)Uninstalling $(BINARY_NAME)...$(NC)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)$(BINARY_NAME) uninstalled$(NC)"

## Documentation targets

# Generate documentation
docs:
	@echo "$(BLUE)Generating documentation...$(NC)"
	$(GO) doc -all ./... > $(DOCS_DIR)/API_REFERENCE.md
	@echo "$(GREEN)Documentation generated$(NC)"

# Serve documentation locally
docs-serve:
	@echo "$(BLUE)Serving documentation...$(NC)"
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Documentation available at http://localhost:6060/pkg/$(PACKAGE)/"; \
		godoc -http=:6060; \
	else \
		echo "$(YELLOW)godoc not found, install with: go install golang.org/x/tools/cmd/godoc@latest$(NC)"; \
	fi

## Utility targets

# Clean build artifacts
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	docker system prune -f 2>/dev/null || true
	@echo "$(GREEN)Clean completed$(NC)"

# Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit Hash: $(COMMIT_HASH)"

# Validate configuration
validate-config:
	@echo "$(BLUE)Validating configuration...$(NC)"
	@if [ -f $(BUILD_DIR)/$(BINARY_NAME) ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) validate --config examples/config.yaml; \
	else \
		echo "$(RED)Binary not found. Run 'make build' first.$(NC)"; \
		exit 1; \
	fi

# Generate sample configuration
generate-config:
	@echo "$(BLUE)Generating sample configuration...$(NC)"
	@if [ -f $(BUILD_DIR)/$(BINARY_NAME) ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) generate --output examples/config-generated.yaml; \
	else \
		echo "$(RED)Binary not found. Run 'make build' first.$(NC)"; \
		exit 1; \
	fi

# Show help
help:
	@echo "$(BLUE)Nereus Build System$(NC)"
	@echo ""
	@echo "$(YELLOW)Build Targets:$(NC)"
	@echo "  build          Build for current platform"
	@echo "  build-all      Build for all supported platforms"
	@echo "  build-linux    Build for Linux (amd64, arm64)"
	@echo "  build-darwin   Build for macOS (amd64, arm64)"
	@echo "  build-windows  Build for Windows (amd64)"
	@echo ""
	@echo "$(YELLOW)Test Targets:$(NC)"
	@echo "  test           Run unit tests"
	@echo "  coverage       Run tests with coverage report"
	@echo "  integration-test Run integration tests"
	@echo "  load-test      Run load tests"
	@echo "  test-all       Run all tests"
	@echo ""
	@echo "$(YELLOW)Code Quality:$(NC)"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo "  vet            Run go vet"
	@echo "  check          Run all checks"
	@echo ""
	@echo "$(YELLOW)Docker:$(NC)"
	@echo "  docker-build   Build Docker image"
	@echo "  docker-push    Push Docker image"
	@echo "  docker-run     Run Docker container"
	@echo ""
	@echo "$(YELLOW)Release:$(NC)"
	@echo "  release-snapshot Create snapshot release"
	@echo "  release-publish  Publish release"
	@echo "  package        Package binaries"
	@echo ""
	@echo "$(YELLOW)Utilities:$(NC)"
	@echo "  clean          Clean build artifacts"
	@echo "  deps           Download dependencies"
	@echo "  install        Install binary to system"
	@echo "  uninstall      Uninstall binary from system"
	@echo "  version        Show version information"
	@echo "  help           Show this help message"

# Development Task List

This document outlines all remaining development tasks needed to complete the nereus SNMPv2c trap alerting system. Tasks are organized by component and marked with completion status.

## Project Status Overview

- **Configuration System**: ✅ Complete
- **SNMP Trap Listener**: ✅ Complete (Core + Protocol Support)
- **MIB Parser**: 🔄 Not Started
- **Alert Management**: 🔄 Not Started
- **Webhook Notifications**: 🔄 Not Started
- **Testing Infrastructure**: 🔄 In Progress (Parser tests added)
- **Documentation**: 🔄 Partial

## Requirements

- When adding a new features, create a new internal package for it, avoid increasing the number of files in a specific internal package.
- When building a binary, the following flags should be set:

  ```shell
  go build -ldflags "-X main.version=dev -X main.buildTime=$(date +%Y%m%d%H%M%S) -X main.commitHash=$(git rev-parse --short HEAD)" -o build/nereus ./main.go
  ```

---

## 1. Foundation & Configuration ✅

### [x] Configuration System (COMPLETED)

- [x] CUE schema definition (`cmd/schemas/config.cue`)
- [x] Integration with `geekxflood/common/config` package
- [x] Generate command for sample configuration
- [x] Validate command with CUE schema validation
- [x] Configuration loading in root command
- [x] Support for default configuration paths

---

## 2. SNMP Trap Listener (Priority: HIGH)

### [x] Core UDP Listener Implementation (COMPLETED)

- [x] Create `internal/listener/listener.go`
- [x] UDP socket binding and configuration
- [x] SNMP packet parsing and validation (basic implementation)
- [x] Community string authentication
- [x] Concurrent trap handler goroutines
- [x] Graceful shutdown handling
- [x] Connection pooling and resource management
- [x] Integration with main server command
- [x] Basic unit tests

**Dependencies**: Configuration system ✅

### [x] SNMP Protocol Support (COMPLETED - SNMPv2c Only)

- [x] SNMP v2c trap parsing (SNMPv1 removed - project is SNMPv2c only)
- [x] Varbind extraction and processing
- [x] Error handling for malformed packets
- [x] Packet validation and security checks
- [x] ASN.1 BER/DER parsing implementation
- [x] OID encoding/decoding
- [x] Comprehensive packet validator
- [x] Statistics collection and monitoring

**Dependencies**: Core UDP Listener ✅

### [/] Testing & Validation (IN PROGRESS)

- [x] Unit tests for SNMP packet parsing
- [ ] Integration tests with mock SNMP agents
- [ ] Performance testing under load
- [x] Error handling validation
- [ ] Memory leak testing

**Dependencies**: SNMP Protocol Support ✅

---

## 3. MIB Parser & OID Resolution (Priority: HIGH)

### [ ] MIB File Loading

- [ ] Create `internal/loader/loader.go`
- [ ] MIB file discovery and enumeration
- [ ] Recursive directory scanning
- [ ] File format validation (.mib, .txt)
- [ ] Caching mechanism for parsed MIBs
- [ ] Hot reload support for MIB changes

**Dependencies**: Configuration system ✅

### [ ] MIB Parsing Engine

- [ ] Create `internal/parser/parser.go`
- [ ] ASN.1 MIB syntax parsing
- [ ] OID tree construction
- [ ] Symbol table generation
- [ ] Cross-reference resolution
- [ ] Error handling for invalid MIBs

**Dependencies**: MIB File Loading

### [ ] OID Resolution Service

- [ ] Create `internal/resolver/resolver.go`
- [ ] Numeric OID to symbolic name translation
- [ ] Description and type information lookup
- [ ] Reverse lookup (name to OID)
- [ ] Efficient search algorithms
- [ ] Caching for frequently accessed OIDs

**Dependencies**: MIB Parsing Engine

### [ ] Testing & Validation

- [ ] Unit tests for MIB parsing
- [ ] Test with standard MIBs (RFC1213, etc.)
- [ ] Performance testing with large MIB sets
- [ ] Memory usage optimization
- [ ] Cache effectiveness validation

**Dependencies**: OID Resolution Service

---

## 4. Alert Management & Storage (Priority: MEDIUM)

### [ ] Event Data Structures

- [ ] Create `internal/events/events.go`
- [ ] Trap event structure definition
- [ ] Alert state management
- [ ] Correlation ID generation
- [ ] Timestamp and metadata handling
- [ ] Serialization support

**Dependencies**: SNMP Trap Listener, MIB Parser

### [ ] Event Storage

- [ ] Create `internal/storage/storage.go`
- [ ] In-memory event store
- [ ] Event persistence (optional)
- [ ] Event querying and filtering
- [ ] Cleanup and retention policies
- [ ] Thread-safe operations

**Dependencies**: Event Data Structures

### [ ] Alert Correlation Engine

- [ ] Create `internal/correlator/correlator.go`
- [ ] Event correlation algorithms
- [ ] Duplicate detection
- [ ] Related event grouping
- [ ] Auto-resolution logic
- [ ] Correlation rule engine

**Dependencies**: Event Storage

### [ ] Testing & Validation

- [ ] Unit tests for event management
- [ ] Correlation algorithm testing
- [ ] Performance testing with high event volumes
- [ ] Memory management validation
- [ ] Concurrency testing

**Dependencies**: Alert Correlation Engine

---

## 5. Webhook Notification System (Priority: MEDIUM)

### [ ] Webhook Client Implementation

- [ ] Create `internal/client/client.go`
- [ ] HTTP client with timeout configuration
- [ ] Custom header support
- [ ] SSL/TLS configuration
- [ ] Connection pooling
- [ ] Request/response logging

**Dependencies**: Configuration system ✅

### [ ] Notification Engine

- [ ] Create `internal/notifier/notifier.go`
- [ ] Webhook payload templating
- [ ] Multiple endpoint support
- [ ] Filtering and routing logic
- [ ] Batch notification support
- [ ] Rate limiting

**Dependencies**: Webhook Client, Alert Management

### [ ] Retry Logic & Reliability

- [ ] Create `internal/retry/retry.go`
- [ ] Exponential backoff implementation
- [ ] Dead letter queue for failed notifications
- [ ] Circuit breaker pattern
- [ ] Health check monitoring
- [ ] Metrics collection

**Dependencies**: Notification Engine

### [ ] Testing & Validation

- [ ] Unit tests for webhook functionality
- [ ] Integration tests with mock endpoints
- [ ] Retry logic validation
- [ ] Performance testing
- [ ] Failure scenario testing

**Dependencies**: Retry Logic & Reliability

---

## 6. Integration & System Components (Priority: MEDIUM)

### [ ] Logging Integration

- [ ] Integrate `geekxflood/common/logging` package
- [ ] Structured logging throughout codebase
- [ ] Log level configuration
- [ ] Component-specific loggers
- [ ] Performance logging
- [ ] Error tracking and alerting

**Dependencies**: All major components

### [ ] Metrics & Monitoring

- [ ] Create `internal/metrics/metrics.go`
- [ ] Prometheus metrics integration
- [ ] Performance counters
- [ ] Health check endpoints
- [ ] System resource monitoring
- [ ] Alert processing metrics

**Dependencies**: All major components

### [ ] Hot Reload Support

- [ ] Configuration hot reload implementation
- [ ] MIB file hot reload
- [ ] Webhook configuration updates
- [ ] Graceful component restart
- [ ] State preservation during reload

**Dependencies**: Configuration system, MIB Parser

---

## 7. Testing Infrastructure (Priority: HIGH)

### [ ] Unit Testing Framework

- [ ] Comprehensive unit test coverage (>80%)
- [ ] Mock implementations for external dependencies
- [ ] Test utilities and helpers
- [ ] Automated test execution
- [ ] Code coverage reporting

**Dependencies**: All components

### [ ] Integration Testing

- [ ] End-to-end testing scenarios
- [ ] Mock SNMP agent for testing
- [ ] Webhook endpoint simulators
- [ ] Configuration validation tests
- [ ] Performance benchmarks

**Dependencies**: Unit Testing Framework

### [ ] Load Testing

- [ ] High-volume trap processing tests
- [ ] Concurrent connection testing
- [ ] Memory usage under load
- [ ] Webhook delivery performance
- [ ] System stability validation

**Dependencies**: Integration Testing

---

## 8. Documentation & Examples (Priority: LOW)

### [ ] API Documentation

- [ ] GoDoc comments for all public APIs
- [ ] Configuration reference documentation
- [ ] MIB integration guide
- [ ] Webhook configuration examples
- [ ] Troubleshooting guide

**Dependencies**: All components

### [ ] Deployment Documentation

- [ ] Installation instructions
- [ ] Docker deployment guide
- [ ] Kubernetes manifests
- [ ] Systemd service configuration
- [ ] Security best practices

**Dependencies**: Packaging & Deployment

### [ ] Example Configurations

- [ ] Production-ready configuration examples
- [ ] Common MIB setups
- [ ] Webhook integration examples
- [ ] Monitoring and alerting setups
- [ ] Performance tuning guides

**Dependencies**: All components

---

## 9. Packaging & Deployment (Priority: LOW)

### [ ] Build System

- [ ] Makefile with build targets
- [ ] Version injection via ldflags
- [ ] Cross-platform builds
- [ ] Release automation
- [ ] Binary packaging

**Dependencies**: All components

### [ ] Container Support

- [ ] Dockerfile optimization
- [ ] Multi-stage builds
- [ ] Security scanning
- [ ] Image size optimization
- [ ] Container registry publishing

**Dependencies**: Build System

### [ ] Distribution Packages

- [ ] RPM package creation
- [ ] DEB package creation
- [ ] Package repository setup
- [ ] Installation scripts
- [ ] Upgrade procedures

**Dependencies**: Container Support

---

## Notes

- Tasks marked with ✅ are completed
- Tasks marked with 🔄 are not started
- Dependencies must be completed before dependent tasks can begin
- Time estimates are for a single developer working full-time
- Testing should be done incrementally alongside development
- Regular code reviews and integration testing recommended throughout development

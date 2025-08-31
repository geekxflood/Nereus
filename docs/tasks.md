# Tasks

This document outlines the step-by-step tasks for simplifying the Nereus SNMP trap alerting system by reducing internal packages from 17 to 9 while maintaining all core functionality.

## Overview

**Goal**: Reduce complexity by consolidating related packages while preserving all SNMP trap processing, webhook delivery, and rule-based alert capabilities.

**Current State**: 17 internal packages (~5,970 LOC)
**Target State**: 9 internal packages (~5,500 LOC)
**Approach**: Phased consolidation with comprehensive testing

## Phase 1: MIB Processing Consolidation

### Task 1.1: Create Consolidated MIB Package Structure

- [x] Create new `internal/mib` directory structure
- [x] Design unified MIB interface combining loader, parser, and resolver functionality
- [x] Create consolidated configuration structure for MIB operations
- [x] Define new public API for the consolidated package

### Task 1.2: Migrate MIB Loader Functionality

- [x] Move `internal/loader/loader.go` core functionality to `internal/mib/loader.go`
- [x] Integrate MIB file loading with hot-reload capabilities
- [x] Preserve all MIB directory scanning and file validation logic
- [x] Update configuration handling for MIB directories and file extensions

### Task 1.3: Migrate MIB Parser Functionality

- [x] Move `internal/mib/parser.go` to new consolidated structure
- [x] Integrate parser with loader for seamless MIB processing
- [x] Preserve OID tree building and cross-reference functionality
- [x] Maintain all MIB parsing statistics and error handling

### Task 1.4: Migrate OID Resolver Functionality

- [x] Move `internal/resolver/resolver.go` functionality into consolidated MIB package
- [x] Integrate OID resolution caching with MIB parsing
- [x] Preserve both forward (OID→name) and reverse (name→OID) resolution
- [x] Maintain resolver statistics and performance metrics

### Task 1.5: Update MIB Package Integration

- [x] Update `internal/app/app.go` to use consolidated MIB package
- [x] Modify initialization order to use single MIB component
- [x] Update all imports throughout codebase from old packages to new `mib` package
- [ ] Remove old `internal/loader` and `internal/resolver` directories

### Task 1.6: Test MIB Consolidation

- [x] Migrate and consolidate all MIB-related tests
- [x] Test MIB loading, parsing, and resolution functionality
- [x] Verify hot-reload functionality still works correctly
- [x] Run integration tests to ensure no regression in OID resolution

## Phase 2: Infrastructure Consolidation

### Task 2.1: Create Infrastructure Package Structure

- [x] Create new `internal/infra` directory
- [x] Design unified interface for HTTP client, retry, and reload functionality
- [x] Create consolidated configuration structure for infrastructure components
- [x] Define clear separation between different infrastructure concerns

### Task 2.2: Migrate HTTP Client Functionality

- [x] Move `internal/client/client.go` to `internal/infra/client.go`
- [x] Preserve all webhook request functionality and statistics
- [x] Maintain timeout handling and error categorization
- [x] Keep all HTTP client configuration options

### Task 2.3: Migrate Retry Functionality

- [x] Move `internal/retry/retry.go` to `internal/infra/retry.go`
- [x] Integrate retry logic with HTTP client for webhook delivery
- [x] Preserve exponential backoff and circuit breaker functionality
- [x] Maintain all retry statistics and configuration options

### Task 2.4: Migrate Reload Functionality

- [x] Move `internal/reload/reload.go` to `internal/infra/reload.go`
- [x] Integrate reload manager with consolidated MIB package
- [x] Preserve file system watching and debouncing logic
- [x] Maintain component reload registration system

### Task 2.5: Update Infrastructure Integration

- [x] Update `internal/app/app.go` to use consolidated infrastructure package
- [x] Modify `internal/notifier` to use consolidated HTTP client and retry
- [x] Update all imports from old packages to new `infra` package
- [ ] Remove old `internal/client`, `internal/retry`, and `internal/reload` directories

### Task 2.6: Test Infrastructure Consolidation

- [x] Migrate and consolidate all infrastructure-related tests
- [x] Test HTTP client functionality with retry mechanisms
- [x] Verify reload functionality works with consolidated MIB package
- [x] Run integration tests for webhook delivery and error handling

## Phase 3: Validation and Alerts Integration

### Task 3.1: Integrate Validation into Listener

- [x] Move `internal/validator/validator.go` functionality into `internal/listener`
- [x] Integrate packet validation directly into trap processing pipeline
- [x] Preserve all validation rules and security checks
- [x] Maintain validation statistics and error reporting

### Task 3.2: Integrate Alerts into Notifier

- [x] Move `internal/alerts/alerts.go` functionality into `internal/notifier`
- [x] Integrate Prometheus alert conversion with notification pipeline
- [x] Preserve all alert formatting and label mapping
- [x] Maintain alert conversion statistics

### Task 3.3: Update Package Integration

- [x] Update `internal/listener` to handle validation internally
- [x] Update `internal/notifier` to handle alert conversion internally
- [x] Update all imports throughout codebase
- [ ] Remove old `internal/validator` and `internal/alerts` directories

### Task 3.4: Test Integration Changes

- [x] Test packet validation within listener package
- [x] Test alert conversion within notifier package
- [x] Verify no regression in validation or alert functionality
- [x] Run end-to-end tests for complete SNMP processing pipeline

## Phase 4: Correlator Simplification

### Task 4.1: Simplify Rule Engine

- [ ] Remove complex flapping detection logic from correlator
- [ ] Remove auto-acknowledgment functionality
- [ ] Simplify rule evaluation to basic field matching only
- [ ] Preserve deduplication and severity setting capabilities

### Task 4.2: Streamline Correlation Logic

- [ ] Reduce correlation rule complexity to essential operations
- [ ] Maintain event grouping and correlation window functionality
- [ ] Preserve correlation statistics for monitoring
- [ ] Update configuration schema to reflect simplified options

### Task 4.3: Test Simplified Correlator

- [ ] Update correlator tests to reflect simplified functionality
- [ ] Test basic deduplication and severity setting
- [ ] Verify correlation statistics are still accurate
- [ ] Test integration with event processing pipeline

## Phase 5: Final Integration and Testing

### Task 5.1: Update Application Orchestration

- [ ] Update `internal/app/app.go` component initialization for all changes
- [ ] Verify correct dependency order with consolidated packages
- [ ] Update health check integration for consolidated components
- [ ] Test graceful shutdown with new package structure

### Task 5.2: Update Configuration Schema

- [ ] Update CUE configuration schemas in `cmd/schemas/config.cue`
- [ ] Ensure backward compatibility with existing configurations
- [ ] Update configuration examples in `examples/` directory
- [ ] Test configuration validation with new package structure

### Task 5.3: Update Documentation

- [ ] Update README.md to reflect new package structure
- [ ] Update development rules in `.augment/rules/development.md`
- [ ] Update package documentation and GoDoc comments
- [ ] Update architecture diagrams and component descriptions

### Task 5.4: Comprehensive Testing

- [ ] Run full test suite with all consolidated packages
- [ ] Perform integration testing with real SNMP traps
- [ ] Test webhook delivery with various notification templates
- [ ] Verify metrics and monitoring functionality

### Task 5.5: Performance Validation

- [ ] Benchmark SNMP trap processing performance
- [ ] Compare memory usage before and after consolidation
- [ ] Test high-throughput scenarios with consolidated packages
- [ ] Verify no performance regression in critical paths

## Phase 6: Cleanup and Finalization

### Task 6.1: Code Cleanup

- [ ] Remove any unused imports or dead code
- [ ] Ensure consistent error handling across consolidated packages
- [ ] Verify all logging includes appropriate component context
- [ ] Update code comments and documentation strings

### Task 6.2: Build and Deployment Testing

- [ ] Test build process with new package structure
- [ ] Verify Docker container builds successfully
- [ ] Test deployment with existing configuration files
- [ ] Validate backward compatibility with production configurations

### Task 6.3: Final Validation

- [ ] Run complete end-to-end testing scenario
- [ ] Verify all original functionality is preserved
- [ ] Test error scenarios and edge cases
- [ ] Confirm metrics and monitoring data accuracy

## Success Criteria

- [ ] Package count reduced from 17 to 9 (47% reduction)
- [ ] All core SNMP trap processing functionality preserved
- [ ] Webhook delivery and notification system fully functional
- [ ] Configuration backward compatibility maintained
- [ ] No performance regression in critical paths
- [ ] All tests passing with >90% coverage
- [ ] Documentation updated and accurate

## Rollback Plan

If any phase encounters critical issues:

- [ ] Revert changes for that phase only
- [ ] Restore original package structure for affected components
- [ ] Run regression tests to ensure system stability
- [ ] Document issues and alternative approaches
- [ ] Consider alternative consolidation strategies

## Notes

- Each task should be completed and tested before proceeding to the next
- Maintain git commits for each major task for easy rollback
- Test thoroughly at each phase boundary
- Keep original packages until consolidation is fully tested and validated
- Monitor system performance throughout the consolidation process

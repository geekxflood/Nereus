# Nereus Comprehensive Testing Guide

This document provides detailed information about the Nereus SNMP trap alerting system's comprehensive testing infrastructure.

## Overview

The Nereus testing suite is designed to validate all aspects of the SNMP trap processing pipeline, from packet reception to final notification delivery. It includes multiple test categories, each targeting specific aspects of the system.

## Test Architecture

### Test Categories

1. **Unit Tests** (`tests/unit/`) - Individual component testing
2. **Integration Tests** (`tests/integration/`) - Component interaction testing
3. **End-to-End Tests** (`tests/e2e/`) - Complete workflow validation
4. **Performance Tests** (`tests/performance/`) - Load and benchmark testing
5. **Security Tests** (`tests/security/`) - Security vulnerability testing
6. **Chaos Engineering Tests** (`tests/chaos/`) - Resilience and failure testing

### Test Infrastructure

- **Common Helpers** (`tests/common/helpers/`) - Shared test utilities
- **Test Fixtures** (`tests/common/fixtures/`) - Sample data and configurations
- **Test Scripts** (`tests/scripts/`) - Automation and CI/CD scripts
- **Docker Support** (`tests/Dockerfile.test`) - Containerized testing

## Quick Start

### Prerequisites

```bash
# Install Go (1.20 or later)
go version

# Install SNMP tools
sudo apt-get install snmp snmp-mibs-downloader  # Ubuntu/Debian
brew install net-snmp                           # macOS

# Install testing tools
cd tests
make install-tools
```

### Running Tests

```bash
# Run all tests
make test

# Run specific test suites
make test-unit
make test-integration
make test-e2e
make test-performance
make test-security
make test-chaos

# Run with coverage
make test-coverage

# Run in CI mode
make test-ci
```

## Test Categories Detail

### Unit Tests

**Purpose**: Test individual functions and methods in isolation.

**Location**: `tests/unit/` (tests are co-located with source code in `internal/`)

**Coverage**: 
- SNMP packet parsing
- MIB OID resolution
- Event processing logic
- Database operations
- Configuration validation

**Example**:
```bash
# Run unit tests for specific package
go test ./internal/listener/...

# Run with coverage
go test -cover ./internal/...
```

### Integration Tests

**Purpose**: Test component interactions and interfaces.

**Location**: `tests/integration/`

**Key Tests**:
- Listener + Event Processor integration
- Storage operations and schema validation
- MIB loading and OID resolution
- Configuration management
- Health checks and metrics

**Example**:
```bash
# Run integration tests
make test-integration

# Run specific integration test
go test -v ./tests/integration/components_test.go
```

### End-to-End Tests

**Purpose**: Validate complete workflows from SNMP trap to final output.

**Location**: `tests/e2e/`

**Test Scenarios**:
- Complete trap processing pipeline
- High-volume trap handling
- MIB enrichment workflows
- Error handling and recovery
- Multi-component integration

**Example**:
```bash
# Run e2e tests
make test-e2e

# Run specific workflow test
go test -v ./tests/e2e/workflow_test.go -run TestCompleteWorkflow
```

### Performance Tests

**Purpose**: Measure system performance and identify bottlenecks.

**Location**: `tests/performance/`

**Metrics Measured**:
- Trap processing throughput
- Memory usage patterns
- CPU utilization
- Database performance
- Network latency

**Example**:
```bash
# Run performance tests
make test-performance

# Run benchmarks
make test-benchmarks

# Run with profiling
make test-profile
```

### Security Tests

**Purpose**: Validate security controls and identify vulnerabilities.

**Location**: `tests/security/`

**Test Areas**:
- SNMP community string validation
- Malformed packet handling
- Input sanitization
- Authentication bypass attempts
- Denial of service resistance

**Example**:
```bash
# Run security tests
make test-security

# Run specific security test
go test -v ./tests/security/security_test.go -run TestCommunityStringValidation
```

### Chaos Engineering Tests

**Purpose**: Test system resilience under adverse conditions.

**Location**: `tests/chaos/`

**Chaos Scenarios**:
- High-volume stress testing
- Memory exhaustion
- Database corruption
- Network partitions
- Component failures

**Example**:
```bash
# Run chaos tests (may take longer)
make test-chaos

# Run specific chaos test
go test -v ./tests/chaos/resilience_test.go -run TestHighVolumeStress
```

## Test Data and Fixtures

### SNMP Test Packets

The test suite includes comprehensive SNMP packet fixtures:

- **Standard Traps**: coldStart, warmStart, linkDown, linkUp, authenticationFailure
- **Vendor-Specific Traps**: Cisco, Juniper, HP, CyberPower
- **Malformed Packets**: Invalid versions, empty communities, oversized packets
- **Load Test Scenarios**: High-volume bursts, sustained loads

### MIB Files

Test MIB files are provided for OID resolution testing:

- `TEST-MIB.mib`: Custom test MIB with various object types
- `SNMPv2-SMI.mib`: Standard SMI definitions
- Vendor MIBs for realistic testing scenarios

### Configuration Templates

Multiple configuration templates support different test scenarios:

- Minimal configurations for unit tests
- Full-featured configurations for integration tests
- Performance-optimized configurations
- Security-hardened configurations

## Test Environment Setup

### Automated Setup

The test infrastructure automatically:

1. Creates temporary directories and databases
2. Copies test MIB files
3. Generates test configurations
4. Sets up isolated network ports
5. Initializes logging and metrics

### Manual Setup

For custom test environments:

```bash
# Create test environment
cd tests
make test-setup

# Clean test artifacts
make test-clean

# Reset environment
make test-reset
```

### Docker Testing

Run tests in isolated containers:

```bash
# Build test container
docker build -t nereus-test -f tests/Dockerfile.test .

# Run all tests
docker run --rm nereus-test

# Run specific test suite
docker run --rm nereus-test make test-unit

# Run with volume for reports
docker run --rm -v $(pwd)/test-reports:/reports nereus-test
```

## Continuous Integration

### GitHub Actions

The CI pipeline includes:

1. **Code Quality**: Linting, formatting, security scanning
2. **Multi-Platform**: Testing on Ubuntu, macOS, Windows
3. **Multi-Version**: Testing with Go 1.20 and 1.21
4. **Comprehensive Testing**: All test suites with coverage
5. **Nightly Tests**: Extended test runs with chaos engineering

### Local CI Simulation

```bash
# Run CI-equivalent tests locally
make test-ci

# Check code quality
make lint vet security-scan

# Multi-platform simulation (requires Docker)
make test-docker
```

## Test Configuration

### Environment Variables

Key environment variables for test configuration:

```bash
export TEST_TIMEOUT=600s      # Test timeout
export TEST_PARALLEL=4        # Parallel test processes
export TEST_VERBOSE=true      # Verbose output
export TEST_SHORT=true        # Skip long-running tests
export ENABLE_COVERAGE=true   # Enable coverage collection
```

### Test-Specific Configuration

Tests use isolated configurations to avoid conflicts:

- **Database**: Temporary SQLite databases
- **Ports**: Non-privileged ports (1162, 9091)
- **Directories**: Temporary directories with cleanup
- **Logging**: Debug level with structured output

## Performance Benchmarking

### Benchmark Tests

Performance benchmarks measure:

- Trap processing rate (traps/second)
- Memory allocation patterns
- CPU usage under load
- Database query performance
- Network throughput

### Running Benchmarks

```bash
# Run all benchmarks
make test-benchmarks

# Run specific benchmark
go test -bench=BenchmarkTrapProcessing ./tests/performance/

# Run with memory profiling
go test -bench=. -memprofile=mem.prof ./tests/performance/

# Analyze profile
go tool pprof mem.prof
```

### Performance Targets

Current performance targets:

- **Throughput**: >1000 traps/second (without enrichment)
- **Latency**: <100ms average processing time
- **Memory**: <512MB under normal load
- **CPU**: <50% utilization at target throughput

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure test ports (1162, 9091) are available
2. **Database Locks**: Clean up test databases between runs
3. **Permission Issues**: Use non-privileged ports for testing
4. **Resource Limits**: Increase limits for chaos/performance tests

### Debug Mode

Enable debug mode for detailed troubleshooting:

```bash
# Run with debug output
NEREUS_TEST_DEBUG=1 make test-e2e

# Run specific test with verbose output
go test -v -run TestSpecificTest ./tests/e2e/

# Enable race detection
go test -race ./...
```

### Log Analysis

Test logs are structured for easy analysis:

```bash
# View test logs
cat test-reports/logs/e2e.log | jq '.'

# Filter by component
cat test-reports/logs/integration.log | jq 'select(.component == "storage")'

# Search for errors
grep -i error test-reports/logs/*.log
```

## Contributing to Tests

### Adding New Tests

1. **Choose appropriate test category** based on what you're testing
2. **Follow naming conventions**: `Test<Component><Scenario>`
3. **Use table-driven tests** for multiple scenarios
4. **Include setup/teardown** for resource management
5. **Add documentation** for complex test scenarios

### Test Guidelines

- **Isolation**: Tests should not depend on each other
- **Repeatability**: Tests should produce consistent results
- **Speed**: Optimize for fast feedback in development
- **Clarity**: Use descriptive names and comments
- **Coverage**: Aim for comprehensive scenario coverage

### Example Test Structure

```go
func TestComponentScenario(t *testing.T) {
    // Setup
    env := helpers.SetupTestEnvironment(t, nil)
    defer env.Cleanup()
    
    // Test cases
    testCases := []struct {
        name     string
        input    interface{}
        expected interface{}
    }{
        // Test cases here
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Test Metrics and Reporting

### Coverage Reports

Coverage reports are generated automatically:

- **HTML Report**: `test-reports/coverage/coverage.html`
- **Summary**: `test-reports/coverage/summary.txt`
- **Combined**: `test-reports/coverage/combined.out`

### Test Results

Test results include:

- **Pass/Fail Status**: For each test suite
- **Performance Metrics**: Throughput, latency, resource usage
- **Coverage Percentage**: Line and function coverage
- **Benchmark Results**: Performance comparisons

### Continuous Monitoring

Set up monitoring for:

- **Test Success Rate**: Track test reliability
- **Performance Trends**: Monitor performance regressions
- **Coverage Changes**: Ensure coverage doesn't decrease
- **Security Scan Results**: Track security improvements

## Support and Resources

### Documentation

- **Main README**: Project overview and setup
- **API Documentation**: Generated from code comments
- **Configuration Guide**: Detailed configuration options
- **Deployment Guide**: Production deployment instructions

### Getting Help

- **GitHub Issues**: Report bugs and request features
- **Discussions**: Ask questions and share ideas
- **Wiki**: Additional documentation and examples
- **Code Review**: Contribute improvements and fixes

### Best Practices

- **Run tests frequently** during development
- **Use appropriate test categories** for different scenarios
- **Monitor test performance** and optimize slow tests
- **Keep tests up to date** with code changes
- **Document complex test scenarios** for future maintainers

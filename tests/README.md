# Nereus SNMP Trap Testing Suite

This directory contains comprehensive testing suites for the Nereus SNMP trap alerting system, designed to validate functionality, performance, security, and reliability across all components.

## Directory Structure

```
tests/
├── README.md                    # This file
├── common/                      # Shared test utilities and fixtures
│   ├── fixtures/               # Test data and sample files
│   ├── helpers/                # Common test helper functions
│   └── config/                 # Test configuration templates
├── e2e/                        # End-to-End Tests
├── integration/                # Integration Tests
├── chaos/                      # Chaos Engineering Tests
├── performance/                # Performance and Load Tests
├── security/                   # Security and Vulnerability Tests
└── scripts/                    # Test automation and CI/CD scripts
```

## Test Categories

### 1. End-to-End Tests (`e2e/`)
Complete workflow validation from SNMP trap reception to final delivery:
- **Workflow Tests**: Full pipeline testing with real SNMP packets
- **Database Integration**: Storage and retrieval validation
- **Webhook Delivery**: Complete notification pipeline testing
- **MIB Enrichment**: OID resolution and data enrichment validation
- **Multi-Component**: Complex scenarios involving all system components

### 2. Integration Tests (`integration/`)
Component interaction and interface validation:
- **Component Integration**: Listener + Processor + Storage interactions
- **MIB Operations**: MIB loading, parsing, and OID resolution
- **Database Operations**: Schema validation and CRUD operations
- **Configuration Management**: Config loading and validation
- **Health & Metrics**: Monitoring endpoint validation

### 3. Chaos Engineering Tests (`chaos/`)
System resilience and failure recovery testing:
- **Network Failures**: Partition simulation and recovery
- **Resource Exhaustion**: Memory, disk, and file descriptor limits
- **Component Failures**: Individual component failure scenarios
- **High-Volume Stress**: Burst traffic and overload conditions
- **Data Corruption**: Database and file corruption recovery

### 4. Performance Tests (`performance/`)
System performance and scalability validation:
- **Load Testing**: Sustained high-volume SNMP trap processing
- **Latency Benchmarks**: Response time measurements
- **Throughput Testing**: Maximum processing capacity
- **Memory Profiling**: Memory usage and leak detection
- **Scalability**: Multi-worker and concurrent processing

### 5. Security Tests (`security/`)
Security vulnerability and attack vector testing:
- **Authentication**: SNMP community string validation
- **Input Validation**: Malformed packet handling
- **Injection Attacks**: SQL injection and template injection
- **Access Control**: Authorization and privilege testing
- **Data Sanitization**: Input sanitization validation

## Quick Start

### Prerequisites
```bash
# Install test dependencies
go mod download
pip3 install -r tests/requirements.txt

# Set up test environment
make test-setup
```

### Running Tests

```bash
# Run all tests
make test-all

# Run specific test suites
make test-e2e
make test-integration
make test-chaos
make test-performance
make test-security

# Run with coverage
make test-coverage

# Run in CI mode
make test-ci
```

### Test Configuration

Tests use isolated configurations and temporary databases to avoid affecting production systems:

- **Test Database**: `test-data/nereus-test.db` (automatically created/cleaned)
- **Test Config**: `tests/common/config/test-config.yaml`
- **Test MIBs**: `tests/common/fixtures/mibs/`
- **Test Ports**: Non-privileged ports (1162 for SNMP, 9091 for metrics)

## Test Data and Fixtures

### SNMP Test Packets
- Standard SNMPv2c trap packets
- Malformed and edge-case packets
- High-volume burst scenarios
- Vendor-specific trap examples

### MIB Test Files
- Standard MIBs (SNMPv2-SMI, SNMPv2-TC, SNMPv2-MIB)
- Vendor MIBs (Cisco, Juniper, HP, etc.)
- Malformed MIB files for error testing
- Large MIB files for performance testing

### Configuration Templates
- Minimal configurations for unit tests
- Full-featured configurations for integration tests
- Invalid configurations for error testing
- Performance-optimized configurations

## Continuous Integration

### GitHub Actions Integration
```yaml
# .github/workflows/test.yml
name: Comprehensive Testing
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - name: Run Test Suite
        run: make test-ci
```

### Test Reporting
- **Coverage Reports**: HTML and XML coverage reports
- **Performance Metrics**: Benchmark results and trends
- **Security Scan Results**: Vulnerability assessment reports
- **Test Artifacts**: Logs, dumps, and debug information

## Development Guidelines

### Writing New Tests
1. **Follow naming conventions**: `Test<Component><Scenario>`
2. **Use table-driven tests** for multiple scenarios
3. **Include setup/teardown** for resource management
4. **Add documentation** for complex test scenarios
5. **Use test fixtures** for consistent test data

### Test Categories
- **Unit Tests**: Individual function/method testing
- **Integration Tests**: Component interaction testing
- **System Tests**: Full system behavior testing
- **Acceptance Tests**: User story validation

### Best Practices
- **Isolation**: Tests should not depend on each other
- **Repeatability**: Tests should produce consistent results
- **Speed**: Fast feedback for development workflow
- **Clarity**: Clear test names and documentation
- **Coverage**: Comprehensive scenario coverage

## Troubleshooting

### Common Issues
1. **Port Conflicts**: Ensure test ports are available
2. **Database Locks**: Clean up test databases between runs
3. **Resource Limits**: Increase limits for chaos testing
4. **Network Issues**: Check firewall and routing for network tests

### Debug Mode
```bash
# Run tests with debug output
NEREUS_TEST_DEBUG=1 make test-e2e

# Run specific test with verbose output
go test -v ./tests/e2e/workflow_test.go
```

### Test Environment Cleanup
```bash
# Clean test artifacts
make test-clean

# Reset test environment
make test-reset
```

## Contributing

When adding new tests:
1. **Update documentation** in this README
2. **Add test fixtures** if needed
3. **Update CI configuration** for new test categories
4. **Follow existing patterns** for consistency
5. **Include performance benchmarks** for new features

## Support

For test-related issues:
- Check the troubleshooting section above
- Review test logs in `tests/logs/`
- Run tests with debug mode enabled
- Consult the main Nereus documentation

## License

This test suite is part of the Nereus project and follows the same licensing terms.

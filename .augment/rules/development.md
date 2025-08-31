---
type: "always_apply"
---

# Nereus SNMP Trap Listener Development Rules

This document defines development guidelines and rules for the Nereus SNMPv2c trap alerting system, a production-ready application for receiving, processing, and forwarding SNMP traps via webhooks.

## Architecture & Design

### Package Structure
- **Use consolidated packages** for related functionality: `mib` (MIB loading + SNMP parsing), `infra` (HTTP client + OID resolution + hot reload), `listener` (SNMP + validation), `notifier` (webhooks + alerts)
- **Maintain clear separation of concerns** between packages: `listener` (SNMP), `storage` (persistence), `correlator` (simplified event logic), `notifier` (webhooks), `metrics` (monitoring), `events` (processing), `mib` (parsing), `infra` (infrastructure)
- **Use interfaces for component integration** - define clear interfaces like `ListenerInterface`, `StorageInterface` for testability and modularity
- **Follow dependency injection patterns** - components should receive their dependencies via constructors, not create them internally

### Component Integration
- **Initialize components in dependency order** in the main application: logging → metrics → mib → infra → storage → correlator → notifier → events → listener
- **Use the application context** (`app.ctx`) for graceful shutdown coordination across all components
- **Implement proper lifecycle management** - all components must support Start(), Stop(), and health checking

### Configuration Management
- **Always use CUE schemas** for configuration validation in `cmd/schemas/config.cue`
- **Support backward compatibility** - maintain legacy configuration keys while introducing new ones
- **Use the geekxflood/common/config package** for all configuration operations
- **Provide sensible defaults** for all configuration options

## Code Quality

### Go Code Standards
- **Follow standard Go conventions** - use `gofmt`, proper naming (PascalCase for exported, camelCase for unexported)
- **Use structured error handling** - wrap errors with context using `fmt.Errorf("operation failed: %w", err)`
- **Implement proper resource cleanup** - always use `defer` for cleanup operations and implement `io.Closer` where appropriate
- **Use sync.WaitGroup** for goroutine coordination and ensure `wg.Done()` is deferred in helper functions

### Logging Practices
- **Use geekxflood/common/logging exclusively** - do not create custom logging wrappers
- **Include component context** in all loggers using `logger.With("component", "component-name")`
- **Use structured logging** with key-value pairs: `logger.Info("message", "key", value, "operation", "operation-name")`
- **Log operations with context** - include operation names, source IPs, processing times, and error details

### Error Handling
- **Return errors up the stack** - don't log and return, choose one based on context
- **Log errors at call sites** with sufficient context for debugging
- **Use appropriate log levels** - Debug for verbose info, Info for normal operations, Warn for recoverable issues, Error for failures
- **Include relevant metadata** in error logs: source IP, packet details, processing time, component name

## SNMP Processing

### SNMPv2c Compliance
- **Support only SNMPv2c** - the project explicitly excludes SNMPv1 and SNMPv3
- **Validate community strings** before processing packets
- **Parse varbinds correctly** using proper ASN.1 BER/DER decoding
- **Handle malformed packets gracefully** - log errors but continue processing other packets

### Packet Processing
- **Use concurrent processing** with worker goroutines for trap handling
- **Implement proper buffering** with configurable queue sizes to handle traffic bursts
- **Track processing metrics** - update Prometheus counters for received, processed, and failed traps
- **Extract trap information** from varbinds including trap OID, timestamp, and variable bindings

### MIB Integration
- **Support hot-reload** of MIB files when `mib.enable_hot_reload` is true
- **Cache OID resolutions** for performance optimization
- **Handle missing MIBs gracefully** - continue processing with numeric OIDs when symbolic names unavailable
- **Validate MIB file formats** before loading and log parsing errors

## Configuration Management

### CUE Schema Validation
- **Update schemas first** when adding new configuration options to `cmd/schemas/config.cue`
- **Use proper CUE types** with constraints (e.g., `int & >=1 & <=65535` for ports)
- **Provide default values** using CUE's default syntax: `string | *"default-value"`
- **Document all configuration fields** with clear comments in the schema

### Backward Compatibility
- **Maintain legacy configuration keys** while introducing new structured ones
- **Support both `app.log_level` and `logging.level`** for logging configuration
- **Deprecate gracefully** - log warnings for deprecated configuration keys
- **Test configuration migration** to ensure existing configs continue working

## Testing

### Unit Testing Requirements
- **Achieve >90% test coverage** for all internal packages
- **Use table-driven tests** for testing multiple scenarios efficiently
- **Create mock implementations** for external dependencies using interfaces
- **Test error conditions** explicitly - don't just test happy paths

### Test Structure
- **Use the common logging package** in tests - create test loggers with `logging.NewLogger(logging.Config{Level: "debug", Format: "json"})`
- **Implement proper test cleanup** - use `defer` for resource cleanup in tests
- **Use testdata directories** for test fixtures like sample MIB files or SNMP packets
- **Test concurrent operations** where applicable using goroutines and proper synchronization

### Integration Testing
- **Test component integration** with real dependencies where possible
- **Use temporary databases** for storage tests with proper cleanup
- **Mock external HTTP endpoints** for webhook testing
- **Test configuration loading** with sample configuration files

## Documentation

### GoDoc Standards
- **Document all exported functions and types** with clear, concise descriptions
- **Include usage examples** in package documentation where helpful
- **Document error conditions** that functions may return
- **Use proper Go comment formatting** - start with the function/type name

### API Documentation
- **Update README.md** when adding new features or changing behavior
- **Maintain configuration examples** in the `examples/` directory
- **Document metrics** in `docs/METRICS.md` when adding new Prometheus metrics
- **Keep task documentation current** in `docs/tasks.md`

## Performance

### Concurrent Processing
- **Use worker pools** for CPU-intensive operations like packet parsing
- **Implement proper backpressure** with buffered channels and queue limits
- **Avoid blocking operations** in hot paths - use timeouts and context cancellation
- **Profile memory usage** regularly and optimize for high-throughput scenarios

### Memory Management
- **Reuse buffers** where possible to reduce garbage collection pressure
- **Implement proper cleanup** for long-lived objects and goroutines
- **Monitor memory metrics** using the built-in system metrics collection
- **Use sync.Pool** for frequently allocated/deallocated objects

### Metrics Collection
- **Update Prometheus metrics** for all significant operations
- **Use appropriate metric types** - Counters for totals, Gauges for current values, Histograms for timing
- **Include relevant labels** but avoid high-cardinality labels that could cause memory issues
- **Batch metric updates** where possible to reduce lock contention

## Security

### Input Validation
- **Validate all SNMP packet fields** before processing
- **Sanitize community strings** and log authentication failures
- **Implement rate limiting** to prevent abuse from specific source IPs
- **Validate configuration inputs** using CUE schemas and additional runtime checks

### Secure Defaults
- **Use secure default configurations** - disable unnecessary features by default
- **Implement proper TLS configuration** for webhook deliveries
- **Log security events** appropriately without exposing sensitive information
- **Follow principle of least privilege** in file permissions and network access

### Error Information Disclosure
- **Avoid exposing internal details** in error messages sent to external systems
- **Log detailed errors internally** while returning generic errors to clients
- **Sanitize webhook payloads** to prevent injection attacks
- **Validate template inputs** when processing webhook templates

## Deployment & Operations

### Health Monitoring
- **Implement comprehensive health checks** for all critical components
- **Use proper HTTP status codes** - 200 for healthy, 503 for unhealthy
- **Update component health status** when components fail or recover
- **Provide detailed readiness checks** separate from liveness checks

### Graceful Shutdown
- **Handle SIGTERM and SIGINT** for graceful shutdown
- **Stop accepting new requests** before shutting down components
- **Wait for in-flight operations** to complete with reasonable timeouts
- **Clean up resources** in reverse dependency order

### Configuration Hot-Reload
- **Support configuration reloading** without service restart where possible
- **Validate new configuration** before applying changes
- **Rollback on validation failures** to maintain service stability
- **Log configuration changes** for audit purposes

## Storage & Data Management

### SQLite Integration

- **Use proper connection pooling** with `max_connections` configuration
- **Implement database migrations** for schema changes with version tracking
- **Use transactions** for multi-statement operations to ensure data consistency
- **Handle database locks gracefully** with appropriate timeouts and retries

### Data Retention

- **Implement automated cleanup** based on `retention_days` configuration
- **Use efficient cleanup queries** with proper indexing to avoid performance impact
- **Log cleanup operations** with statistics on records removed
- **Monitor database size** using the metrics system

### Event Correlation

- **Use time-based windows** for correlation with configurable durations
- **Implement deduplication** using event fingerprints (source IP, trap OID, key varbinds)
- **Use simplified correlation rules** focusing on essential field matching operations
- **Clean up correlation groups** when they become inactive

## Webhook & Notification System

### Webhook Delivery

- **Implement exponential backoff** with jitter for retry attempts
- **Use circuit breaker pattern** to handle failing endpoints gracefully
- **Support custom headers** and authentication methods
- **Template webhook payloads** using Go's text/template package

### Reliability & Resilience

- **Queue notifications** with configurable queue sizes and worker pools
- **Persist failed deliveries** for retry processing across restarts
- **Monitor delivery success rates** using Prometheus metrics
- **Implement timeout handling** for slow webhook endpoints

### Template Processing

- **Validate templates** at configuration load time
- **Sanitize template data** to prevent injection attacks
- **Support conditional logic** in templates for flexible payload generation
- **Handle template errors gracefully** without stopping notification processing

## Build & Deployment

### Build Process

- **Use ldflags for version injection** as specified in requirements:

  ```bash
  go build -ldflags "-X main.version=dev -X main.buildTime=$(date +%Y%m%d%H%M%S) -X main.commitHash=$(git rev-parse --short HEAD)" -o build/nereus ./main.go
  ```

- **Support cross-platform builds** for different operating systems and architectures
- **Include build metadata** in application startup logs
- **Use proper build tags** for conditional compilation when needed

### Container Support

- **Use multi-stage Docker builds** to minimize image size
- **Run as non-root user** in containers for security
- **Support configuration via environment variables** in addition to files
- **Include health check commands** in Docker images

### Monitoring Integration

- **Expose Prometheus metrics** on a separate port from the main SNMP listener
- **Support Kubernetes probes** with proper health and readiness endpoints
- **Log structured data** compatible with log aggregation systems
- **Include trace IDs** for request correlation across components

## Development Workflow

### Code Review Guidelines

- **Review for security implications** especially in SNMP parsing and webhook delivery
- **Verify error handling** is comprehensive and appropriate
- **Check resource cleanup** and goroutine lifecycle management
- **Validate configuration schema updates** match implementation changes

### Testing Strategy

- **Run tests in CI/CD** with proper coverage reporting
- **Test with real SNMP packets** using captured traffic or generated test data
- **Validate webhook delivery** with mock HTTP servers
- **Test configuration validation** with both valid and invalid configurations

### Documentation Updates

- **Update task documentation** when completing features
- **Maintain configuration examples** that reflect current schema
- **Document breaking changes** in release notes
- **Keep metrics documentation current** when adding new metrics

## Common Patterns & Anti-Patterns

### Recommended Patterns

- **Use context.Context** for cancellation and timeouts throughout the application
- **Implement proper interfaces** for testability and modularity
- **Use structured configuration** with clear hierarchies and validation
- **Apply consistent error handling** with appropriate logging and metrics

### Anti-Patterns to Avoid

- **Don't create circular dependencies** between internal packages
- **Avoid global state** - use dependency injection instead
- **Don't ignore errors** - always handle or explicitly ignore with comments
- **Avoid blocking operations** in critical paths without timeouts
- **Don't hardcode configuration values** - make everything configurable with defaults

### Performance Considerations

- **Profile before optimizing** - use Go's built-in profiling tools
- **Optimize for the common case** - fast path for normal SNMP trap processing
- **Use appropriate data structures** - maps for lookups, slices for ordered data
- **Consider memory allocation patterns** - reuse objects where beneficial

This rule file should be treated as a living document and updated as the project evolves and new patterns emerge.

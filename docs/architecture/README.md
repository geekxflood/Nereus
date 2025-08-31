# Nereus Architecture

This document provides a comprehensive overview of the Nereus SNMP trap alerting system architecture.

## System Overview

Nereus is designed as a high-performance, production-ready SNMP trap processing system with the following key characteristics:

- **Event-Driven Architecture**: Asynchronous processing with worker pools
- **Modular Design**: Clear separation of concerns across 10 core packages
- **Interface-Based**: Loose coupling through well-defined interfaces
- **Observable**: Comprehensive metrics and structured logging
- **Scalable**: Stateless design supporting horizontal scaling

## Core Components

### 1. Application Layer (`internal/app`)

**Responsibility**: Application orchestration and lifecycle management

- Coordinates initialization of all components
- Manages graceful shutdown sequences
- Provides dependency injection container
- Handles signal processing (SIGTERM, SIGINT)

**Key Features**:
- Component health monitoring
- Graceful shutdown with timeout handling
- Context-based cancellation propagation
- Structured logging with component context

### 2. SNMP Listener (`internal/listener`)

**Responsibility**: SNMP trap reception and validation

- UDP socket management on port 162
- SNMPv2c packet validation
- Community string authentication
- Concurrent packet processing

**Key Features**:
- Configurable buffer sizes for high-throughput scenarios
- Packet validation and error handling
- Source IP tracking and logging
- Worker pool for concurrent processing

### 3. MIB Management (`internal/mib`)

**Responsibility**: MIB loading, parsing, and SNMP packet processing

**Consolidated Functionality**:
- MIB file loading and parsing
- OID to name resolution with caching
- SNMP packet parsing (ASN.1 BER/DER)
- Hot reload of MIB files

**Key Features**:
- Support for standard and custom MIB files
- Efficient OID lookup with caching
- File system watching for hot reload
- Comprehensive error handling for malformed MIBs

### 4. Event Processing (`internal/events`)

**Responsibility**: Event processing pipeline and enrichment

- SNMP trap to event conversion
- Event enrichment with MIB data
- Worker pool management
- Integration with correlator and storage

**Key Features**:
- Concurrent event processing
- Event enrichment with OID resolution
- Configurable worker pools
- Error handling and retry logic

### 5. Event Correlation (`internal/correlator`)

**Responsibility**: Event correlation and deduplication

**Simplified Design**:
- Time-based deduplication windows
- Event fingerprinting for duplicate detection
- Correlation group management
- Automatic cleanup of inactive groups

**Key Features**:
- Configurable correlation and deduplication windows
- Memory-efficient correlation tracking
- Event state management (new, duplicate, resolved)
- Metrics for correlation effectiveness

### 6. Storage Layer (`internal/storage`)

**Responsibility**: Event persistence and data management

- SQLite database operations
- Event CRUD operations
- Database schema management
- Data retention policies

**Key Features**:
- Connection pooling for performance
- Automatic schema migrations
- Configurable retention periods
- Transaction management for consistency

### 7. Notification System (`internal/notifier`)

**Responsibility**: Webhook delivery and alert management

**Consolidated Functionality**:
- Webhook delivery with retry logic
- Multiple notification formats (Alertmanager, custom)
- Template processing for custom payloads
- Circuit breaker for failing endpoints

**Key Features**:
- Exponential backoff with jitter
- Configurable retry policies
- Multiple webhook endpoints
- Template-based payload generation

### 8. Infrastructure Services (`internal/infra`)

**Responsibility**: Cross-cutting infrastructure concerns

**Consolidated Functionality**:
- HTTP client with retry mechanisms
- Configuration hot reload
- OID resolution services
- Utility functions

**Key Features**:
- Configurable HTTP timeouts and retries
- File system watching for configuration changes
- Centralized HTTP client configuration
- Error handling and logging

### 9. Metrics and Monitoring (`internal/metrics`)

**Responsibility**: Observability and health monitoring

- Prometheus metrics collection
- Health and readiness endpoints
- System metrics (CPU, memory, goroutines)
- Component-specific metrics

**Key Features**:
- Comprehensive metric coverage
- HTTP server for metrics endpoint
- Health check aggregation
- Performance monitoring

### 10. Common Types (`internal/types`)

**Responsibility**: Shared data structures and constants

- Event and trap data structures
- Configuration types
- Interface definitions
- Constants and enumerations

**Key Features**:
- Zero dependencies (pure types)
- Comprehensive data models
- Type safety and validation
- Clear interface definitions

## Data Flow Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   SNMP Traps    │───▶│   Listener      │───▶│   Event Parser  │
│   (UDP:162)     │    │   (Validation)  │    │   (MIB Lookup)  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Webhooks      │◀───│   Notifier      │◀───│   Correlator    │
│   (HTTP/HTTPS)  │    │   (Templates)   │    │   (Dedup/Group) │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Prometheus    │◀───│   Metrics       │    │   Storage       │
│   (Port 9090)   │    │   (Health)      │    │   (SQLite)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Processing Pipeline

1. **Trap Reception**: SNMP listener receives UDP packets on port 162
2. **Validation**: Community string and packet format validation
3. **Parsing**: ASN.1 BER/DER parsing of SNMP packet structure
4. **MIB Resolution**: OID lookup and name resolution using loaded MIBs
5. **Event Creation**: Conversion to internal event structure with enrichment
6. **Correlation**: Deduplication and correlation with existing events
7. **Storage**: Persistence to SQLite database with retention policies
8. **Notification**: Webhook delivery to configured endpoints
9. **Metrics**: Update Prometheus counters and timing metrics

## Package Dependencies

```
app
├── correlator ──── types
├── events ────────┼── correlator
│                  ├── mib ──── types
│                  ├── storage ──── types
│                  └── types
├── infra (no internal deps)
├── listener (no internal deps)
├── metrics (no internal deps)
├── mib ──── types
├── notifier (no internal deps)
└── storage ──── types

types (no dependencies)
```

### Dependency Principles

- **Minimal Coupling**: Most packages have zero or minimal internal dependencies
- **Interface-Based**: Components interact through well-defined interfaces
- **Dependency Injection**: Dependencies provided via constructors
- **No Circular Dependencies**: Clean dependency graph with clear hierarchy

## Scalability Considerations

### Horizontal Scaling

- **Stateless Design**: No shared state between instances
- **Database Sharing**: Multiple instances can share SQLite database
- **Load Balancing**: UDP load balancing for SNMP trap distribution
- **Webhook Partitioning**: Different instances can handle different webhook endpoints

### Performance Optimizations

- **Worker Pools**: Configurable concurrency for all processing stages
- **Connection Pooling**: Database connection reuse
- **Caching**: OID resolution caching for performance
- **Buffering**: Configurable buffer sizes for high-throughput scenarios

### Resource Management

- **Memory Efficiency**: Careful memory management in hot paths
- **Goroutine Lifecycle**: Proper cleanup of background workers
- **File Descriptor Management**: Efficient socket and file handling
- **Database Optimization**: Proper indexing and query optimization

## Security Architecture

### Network Security

- **UDP Validation**: Comprehensive packet validation
- **Community String Authentication**: SNMPv2c community validation
- **Source IP Logging**: Complete audit trail of trap sources
- **Rate Limiting**: Protection against DoS attacks

### Application Security

- **Input Validation**: All inputs validated against schemas
- **Error Handling**: Secure error messages without information disclosure
- **Configuration Validation**: CUE schema validation for all settings
- **Dependency Management**: Regular security updates and scanning

### Operational Security

- **Non-Root Execution**: Container runs as non-root user
- **Minimal Attack Surface**: Only necessary ports exposed
- **Structured Logging**: Comprehensive audit logging
- **Health Monitoring**: Continuous health and security monitoring

## Configuration Architecture

### CUE Schema Validation

- **Type Safety**: All configuration validated against CUE schemas
- **Default Values**: Sensible defaults for all optional settings
- **Constraint Validation**: Range and format validation
- **Documentation**: Self-documenting configuration schemas

### Hot Reload Support

- **File Watching**: Automatic detection of configuration changes
- **Graceful Updates**: Non-disruptive configuration updates
- **Validation**: New configuration validated before application
- **Rollback**: Automatic rollback on validation failures

### Environment Integration

- **Environment Variables**: Support for environment-based configuration
- **Container-Friendly**: Kubernetes ConfigMap and Secret integration
- **Development**: Easy local development configuration
- **Production**: Production-ready defaults and validation

This architecture provides a solid foundation for enterprise SNMP monitoring with excellent performance, reliability, and maintainability characteristics.

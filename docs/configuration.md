# Configuration Reference

This document provides a comprehensive reference for configuring Nereus.

## Configuration Overview

Nereus uses YAML configuration files with CUE schema validation for type safety and comprehensive validation. The configuration supports both new structured format and legacy compatibility.

## Quick Start

Generate a sample configuration:

```bash
./nereus generate --output config.yaml
```

Validate your configuration:

```bash
./nereus validate --config config.yaml
```

## Complete Configuration Reference

### Application Settings

```yaml
app:
  name: "nereus-snmp-listener"     # Application name for logging
  version: "1.0.0"                 # Application version
  environment: "production"        # Environment (development, staging, production)
```

### Server Configuration

```yaml
server:
  host: "0.0.0.0"                 # Listen address (default: 0.0.0.0)
  port: 162                       # SNMP trap port (default: 162)
  buffer_size: 65536              # UDP buffer size in bytes (default: 65536)
  read_timeout: "30s"             # Socket read timeout (default: 30s)
  community: "public"             # SNMP community string (default: public)
```

**Notes**:
- Port 162 requires root privileges or capability binding
- Buffer size affects memory usage and packet handling capacity
- Community string must match SNMP trap senders

### MIB Configuration

```yaml
mib:
  directories:                    # MIB file directories
    - "/usr/share/snmp/mibs"
    - "/opt/custom-mibs"
  enable_hot_reload: true         # Enable hot reload of MIB files (default: true)
  required_mibs:                  # Required MIB files for startup
    - "SNMPv2-SMI"
    - "SNMPv2-TC"
    - "SNMPv2-MIB"
  cache_size: 10000              # OID resolution cache size (default: 10000)
  reload_interval: "5m"          # Hot reload check interval (default: 5m)
```

**Notes**:
- At least one MIB directory must contain required MIBs
- Hot reload monitors directories for changes
- Cache size affects memory usage vs. lookup performance

### Event Correlation

```yaml
correlator:
  enable_correlation: true        # Enable event correlation (default: true)
  deduplication_window: "5m"     # Deduplication time window (default: 5m)
  correlation_window: "10m"      # Correlation time window (default: 10m)
  max_correlation_groups: 1000   # Maximum active correlation groups (default: 1000)
  cleanup_interval: "1m"         # Cleanup interval for inactive groups (default: 1m)
```

**Correlation Logic**:
- Events with same source IP and trap OID are deduplicated within the window
- Related events are grouped for correlation analysis
- Automatic cleanup prevents memory leaks

### Storage Configuration

```yaml
storage:
  connection_string: "./nereus_events.db"  # SQLite database file path
  max_connections: 10                      # Maximum database connections (default: 10)
  retention_days: 30                       # Event retention period (default: 30)
  cleanup_interval: "1h"                   # Cleanup job interval (default: 1h)
  enable_wal: true                         # Enable WAL mode for performance (default: true)
```

**Notes**:
- SQLite database file will be created if it doesn't exist
- WAL mode improves concurrent access performance
- Retention cleanup runs automatically

### Notification System

```yaml
notifier:
  enable_notifications: true      # Enable webhook notifications (default: true)
  worker_count: 2                # Number of notification workers (default: 2)
  queue_size: 100               # Notification queue size (default: 100)
  default_timeout: "30s"        # Default webhook timeout (default: 30s)
  max_retries: 3                # Maximum retry attempts (default: 3)
  retry_delay: "5s"             # Initial retry delay (default: 5s)
  
  default_webhooks:
    - name: "alertmanager"
      url: "http://alertmanager:9093/api/v1/alerts"
      method: "POST"              # HTTP method (default: POST)
      format: "alertmanager"      # Format: alertmanager, prometheus, custom
      enabled: true               # Enable this webhook (default: true)
      timeout: "30s"              # Request timeout (default: 30s)
      insecure: false             # Skip TLS verification (default: false)
      content_type: "application/json"  # Content-Type header
      
      # Custom headers
      headers:
        Authorization: "Bearer token"
        X-Custom-Header: "value"
      
      # Retry configuration
      retry_count: 3              # Override global retry count
      retry_delay: "5s"           # Override global retry delay
      
      # Filtering (optional)
      filters:
        severity: ["critical", "warning"]  # Only send these severities
        source_ips: ["192.168.1.0/24"]    # Only from these IP ranges
```

### Infrastructure Services

```yaml
infra:
  http_client:
    timeout: "30s"               # HTTP client timeout (default: 30s)
    max_idle_conns: 100         # Maximum idle connections (default: 100)
    max_conns_per_host: 10      # Maximum connections per host (default: 10)
    idle_conn_timeout: "90s"    # Idle connection timeout (default: 90s)
    
  hot_reload:
    enable: true                # Enable configuration hot reload (default: true)
    check_interval: "30s"       # Configuration check interval (default: 30s)
    
  oid_resolution:
    cache_ttl: "1h"            # OID cache TTL (default: 1h)
    max_cache_size: 10000      # Maximum cache entries (default: 10000)
```

### Metrics and Monitoring

```yaml
metrics:
  enabled: true                 # Enable Prometheus metrics (default: true)
  listen_address: ":9090"      # Metrics server address (default: :9090)
  path: "/metrics"             # Metrics endpoint path (default: /metrics)
  
  # Health check endpoints
  health_path: "/health"       # Health check path (default: /health)
  ready_path: "/ready"         # Readiness check path (default: /ready)
  
  # System metrics
  enable_system_metrics: true  # Enable system metrics collection (default: true)
  collection_interval: "15s"   # Metrics collection interval (default: 15s)
```

### Logging Configuration

```yaml
logging:
  level: "info"                # Log level: debug, info, warn, error (default: info)
  format: "json"               # Log format: json, text (default: json)
  output: "stdout"             # Log output: stdout, stderr, file path (default: stdout)
  
  # Component-specific logging
  components:
    listener: "debug"          # Override log level for specific components
    correlator: "info"
    notifier: "warn"
```

## Legacy Configuration Support

For backward compatibility, the following legacy configuration keys are still supported:

```yaml
# Legacy format (still supported)
resolver:
  cache_size: 10000
  
client:
  timeout: "30s"
  
reload:
  enable: true
  interval: "30s"
```

These are automatically mapped to the new `infra` section.

## Environment Variables

Configuration values can be overridden using environment variables:

```bash
# Server configuration
export NEREUS_SERVER_HOST="0.0.0.0"
export NEREUS_SERVER_PORT="162"

# Logging configuration
export NEREUS_LOG_LEVEL="debug"
export NEREUS_LOG_FORMAT="json"

# Database configuration
export NEREUS_STORAGE_CONNECTION_STRING="/data/nereus.db"
```

Environment variable format: `NEREUS_<SECTION>_<KEY>` (uppercase, underscores)

## Configuration Validation

The configuration is validated against a CUE schema that ensures:

- **Type Safety**: All values match expected types
- **Range Validation**: Numeric values within valid ranges
- **Required Fields**: Essential configuration is present
- **Format Validation**: URLs, durations, and other formats are valid

Common validation errors:

```bash
# Invalid port range
server.port: invalid value 70000 (out of bound <=65535)

# Invalid duration format
correlator.deduplication_window: invalid duration "5minutes"

# Missing required MIB
mib.required_mibs: missing required MIB "SNMPv2-SMI"
```

## Production Configuration Examples

See the [examples](../../examples/) directory for complete configuration examples:

- `config.yaml` - Basic production configuration
- `config-ha.yaml` - High availability configuration
- `config-dev.yaml` - Development configuration
- `docker-compose.yaml` - Docker deployment configuration

## Configuration Best Practices

1. **Use Configuration Validation**: Always validate before deployment
2. **Environment-Specific Configs**: Separate configs for dev/staging/prod
3. **Secret Management**: Use environment variables for sensitive data
4. **Resource Sizing**: Adjust worker counts and buffer sizes for your load
5. **Monitoring**: Enable metrics and health checks in production
6. **Backup**: Include database files in backup procedures
7. **Security**: Use TLS for webhook endpoints in production
8. **Testing**: Test configuration changes in non-production first

## Troubleshooting

Common configuration issues and solutions:

### MIB Loading Issues
```bash
# Check MIB file permissions
ls -la /usr/share/snmp/mibs/

# Validate MIB syntax
snmptranslate -M /usr/share/snmp/mibs -m ALL 1.3.6.1.2.1.1.1.0
```

### Database Issues
```bash
# Check database file permissions
ls -la ./nereus_events.db

# Check disk space
df -h .
```

### Network Issues
```bash
# Check port binding (requires root for port 162)
sudo netstat -ulnp | grep 162

# Test SNMP trap reception
snmptrap -v2c -c public localhost:162 1.3.6.1.4.1.1 1.3.6.1.4.1.1.1 s "test"
```

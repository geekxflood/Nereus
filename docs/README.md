# Nereus Documentation

Welcome to the comprehensive documentation for Nereus, the production-ready SNMP trap alerting system.

## 📚 Documentation Structure

### 🏗️ [Architecture](architecture/)
**System design and technical architecture**
- [System Overview](architecture/README.md#system-overview)
- [Core Components](architecture/README.md#core-components)
- [Data Flow Architecture](architecture/README.md#data-flow-architecture)
- [Package Dependencies](architecture/README.md#package-dependencies)
- [Scalability Considerations](architecture/README.md#scalability-considerations)
- [Security Architecture](architecture/README.md#security-architecture)

### ⚙️ [Configuration](configuration/)
**Complete configuration reference and examples**
- [Configuration Overview](configuration/README.md#configuration-overview)
- [Complete Reference](configuration/README.md#complete-configuration-reference)
- [Environment Variables](configuration/README.md#environment-variables)
- [Legacy Support](configuration/README.md#legacy-configuration-support)
- [Validation](configuration/README.md#configuration-validation)
- [Best Practices](configuration/README.md#configuration-best-practices)

### 🚀 [Deployment](deployment/)
**Production deployment guides for various environments**
- [Docker Deployment](deployment/README.md#docker-deployment)
- [Kubernetes Deployment](deployment/README.md#kubernetes-deployment)
- [Systemd Service](deployment/README.md#systemd-service)
- [Binary Installation](deployment/README.md#binary-installation)
- [Production Considerations](deployment/README.md#production-considerations)
- [Troubleshooting](deployment/README.md#troubleshooting)

### 🛠️ [Development](development/)
**Contributing and development information**
- [Development Guide](development/README.md)
- [Contributing Guidelines](development/CONTRIBUTING.md)
- [Project Structure](development/README.md#project-structure)
- [Testing Guidelines](development/README.md#testing-guidelines)
- [Code Quality Standards](development/README.md#code-quality-standards)
- [Performance Guidelines](development/README.md#performance-guidelines)

### 📡 [API Reference](api/)
**REST endpoints and metrics documentation**
- [HTTP Endpoints](api/README.md#http-endpoints)
- [Health and Status](api/README.md#health-and-status-endpoints)
- [Prometheus Metrics](api/README.md#metrics-reference)
- [Webhook Formats](api/README.md#webhook-payload-formats)
- [Monitoring Integration](api/README.md#monitoring-integration)

## 🚀 Quick Start Guide

### 1. Installation

Choose your preferred installation method:

#### Docker (Recommended)
```bash
docker run -d \
  --name nereus \
  -p 162:162/udp \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/nereus/config.yaml \
  geekxflood/nereus:latest
```

#### Binary Installation
```bash
# Download and install
wget https://github.com/geekxflood/nereus/releases/latest/download/nereus-linux-amd64.tar.gz
tar -xzf nereus-linux-amd64.tar.gz
sudo cp nereus /usr/local/bin/

# Generate configuration
nereus generate --output config.yaml

# Start the service
nereus --config config.yaml
```

### 2. Basic Configuration

Generate a sample configuration file:

```bash
nereus generate --output config.yaml
```

Minimal configuration example:

```yaml
app:
  name: "nereus-snmp-listener"

server:
  host: "0.0.0.0"
  port: 162

mib:
  directories: ["/usr/share/snmp/mibs"]
  required_mibs: ["SNMPv2-SMI", "SNMPv2-TC", "SNMPv2-MIB"]

notifier:
  default_webhooks:
    - name: "alertmanager"
      url: "http://alertmanager:9093/api/v1/alerts"
      format: "alertmanager"

storage:
  connection_string: "./nereus_events.db"

metrics:
  enabled: true
  listen_address: ":9090"

logging:
  level: "info"
  format: "json"
```

### 3. Validation and Testing

Validate your configuration:

```bash
nereus validate --config config.yaml
```

Test SNMP trap reception:

```bash
# Send a test trap (requires snmp-utils)
snmptrap -v2c -c public localhost:162 1.3.6.1.4.1.1 1.3.6.1.4.1.1.1 s "test trap"
```

Check metrics and health:

```bash
curl http://localhost:9090/health
curl http://localhost:9090/metrics
```

## 📖 Key Concepts

### SNMP Trap Processing

Nereus processes SNMP traps through a multi-stage pipeline:

1. **Reception**: UDP listener receives SNMPv2c traps on port 162
2. **Validation**: Community string and packet format validation
3. **Parsing**: ASN.1 BER/DER parsing of SNMP packet structure
4. **MIB Resolution**: OID lookup using loaded MIB definitions
5. **Correlation**: Event deduplication and correlation
6. **Storage**: Persistence to SQLite database
7. **Notification**: Webhook delivery to configured endpoints

### Event Correlation

Nereus includes intelligent event correlation:

- **Deduplication**: Automatic detection of duplicate events
- **Time Windows**: Configurable correlation and deduplication windows
- **Event Grouping**: Related events grouped for analysis
- **State Management**: Track event lifecycle (new, duplicate, resolved)

### Webhook Notifications

Flexible notification system supporting:

- **Multiple Formats**: Alertmanager, Prometheus, custom templates
- **Retry Logic**: Exponential backoff with circuit breaker
- **Multiple Endpoints**: Support for multiple webhook destinations
- **Template Engine**: Custom payload generation using templates

### Monitoring and Observability

Comprehensive monitoring capabilities:

- **Prometheus Metrics**: 20+ metrics covering all components
- **Health Checks**: Kubernetes-ready liveness and readiness probes
- **Structured Logging**: JSON-formatted logs with component context
- **Performance Tracking**: Detailed timing and throughput metrics

## 🏗️ Architecture Overview

Nereus follows a modular architecture with 10 core packages:

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

### Core Packages

- **`internal/app`**: Application orchestration and lifecycle management
- **`internal/listener`**: SNMP trap reception and validation
- **`internal/mib`**: MIB loading, parsing, and SNMP packet processing
- **`internal/events`**: Event processing pipeline and enrichment
- **`internal/correlator`**: Event correlation and deduplication
- **`internal/storage`**: Database operations and persistence
- **`internal/notifier`**: Webhook delivery and alert management
- **`internal/infra`**: Infrastructure services (HTTP client, hot reload)
- **`internal/metrics`**: Prometheus metrics and health monitoring
- **`internal/types`**: Common data structures and interfaces

## 🔧 Configuration Examples

### Production Configuration

```yaml
app:
  name: "nereus-production"
  environment: "production"

server:
  host: "0.0.0.0"
  port: 162
  buffer_size: 131072

mib:
  directories: 
    - "/usr/share/snmp/mibs"
    - "/opt/custom-mibs"
  enable_hot_reload: true
  required_mibs: ["SNMPv2-SMI", "SNMPv2-TC", "SNMPv2-MIB"]

correlator:
  enable_correlation: true
  deduplication_window: "5m"
  correlation_window: "15m"

notifier:
  enable_notifications: true
  worker_count: 4
  default_webhooks:
    - name: "alertmanager"
      url: "http://alertmanager:9093/api/v1/alerts"
      format: "alertmanager"
      timeout: "30s"
      retry_count: 3

storage:
  connection_string: "/var/lib/nereus/events.db"
  retention_days: 90
  max_connections: 20

metrics:
  enabled: true
  listen_address: ":9090"
  enable_system_metrics: true

logging:
  level: "info"
  format: "json"
  output: "/var/log/nereus/nereus.log"
```

### High Availability Configuration

```yaml
# Multiple webhook endpoints for redundancy
notifier:
  default_webhooks:
    - name: "primary-alertmanager"
      url: "http://alertmanager-1:9093/api/v1/alerts"
      format: "alertmanager"
    - name: "secondary-alertmanager"
      url: "http://alertmanager-2:9093/api/v1/alerts"
      format: "alertmanager"
    - name: "pagerduty-backup"
      url: "https://events.pagerduty.com/v2/enqueue"
      format: "custom"
      template: "pagerduty"

# Enhanced storage configuration
storage:
  connection_string: "/shared/storage/nereus_events.db"
  retention_days: 180
  max_connections: 50
  enable_wal: true

# Increased worker pools for high throughput
correlator:
  max_correlation_groups: 5000

notifier:
  worker_count: 8
  queue_size: 1000
```

## 📊 Monitoring and Alerting

### Key Metrics to Monitor

1. **SNMP Trap Processing**:
   - `nereus_traps_received_total`: Total traps received
   - `nereus_traps_processed_total`: Successfully processed traps
   - `nereus_trap_processing_duration_seconds`: Processing latency

2. **Event Correlation**:
   - `nereus_events_correlated_total`: Events after correlation
   - `nereus_correlation_groups_active`: Active correlation groups

3. **Webhook Delivery**:
   - `nereus_webhooks_sent_total`: Webhook delivery attempts
   - `nereus_webhook_duration_seconds`: Delivery latency

4. **System Health**:
   - `nereus_goroutines`: Number of goroutines
   - `nereus_memory_usage_bytes`: Memory usage

### Sample Alerting Rules

```yaml
groups:
  - name: nereus
    rules:
      - alert: NereusDown
        expr: up{job="nereus"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Nereus SNMP listener is down"
          
      - alert: HighTrapProcessingLatency
        expr: histogram_quantile(0.95, nereus_trap_processing_duration_seconds_bucket) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High SNMP trap processing latency"
          
      - alert: WebhookDeliveryFailures
        expr: rate(nereus_webhooks_sent_total{status!="200"}[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High webhook delivery failure rate"
```

## 🆘 Troubleshooting

### Common Issues

1. **Port 162 Permission Denied**:
   ```bash
   # Solution: Use capabilities or run as root
   sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/nereus
   ```

2. **MIB Loading Failures**:
   ```bash
   # Check MIB file permissions and paths
   ls -la /usr/share/snmp/mibs/
   nereus validate --config config.yaml --check-mibs
   ```

3. **Database Lock Issues**:
   ```bash
   # Check for multiple instances
   lsof /var/lib/nereus/events.db
   ```

4. **Webhook Delivery Failures**:
   ```bash
   # Check network connectivity and endpoint health
   curl -v http://alertmanager:9093/api/v1/alerts
   ```

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
logging:
  level: "debug"
  components:
    listener: "debug"
    correlator: "debug"
    notifier: "debug"
```

## 📞 Support and Community

### Getting Help

- **Documentation**: Start with this documentation
- **GitHub Issues**: Report bugs and request features
- **GitHub Discussions**: Ask questions and discuss ideas
- **Examples**: Check the `examples/` directory for configuration samples

### Contributing

We welcome contributions! See our [Contributing Guide](development/CONTRIBUTING.md) for:

- Development environment setup
- Code style guidelines
- Testing requirements
- Pull request process

### License

Nereus is licensed under the MIT License. See [LICENSE](../LICENSE) for details.

---

**Nereus** - Protecting your network infrastructure like the ancient sea god protected sailors.

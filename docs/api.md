# API Reference

This document provides comprehensive reference for Nereus APIs and endpoints.

## HTTP Endpoints

Nereus exposes several HTTP endpoints for monitoring, health checks, and metrics collection.

### Base URL

Default base URL: `http://localhost:9090`

Configurable via `metrics.listen_address` in configuration.

## Health and Status Endpoints

### Health Check

**Endpoint**: `GET /health`

**Description**: Liveness probe endpoint that indicates if the service is running.

**Response**:

```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "uptime": "2h30m15s"
}
```

**Status Codes**:

- `200 OK`: Service is running
- `503 Service Unavailable`: Service is not healthy

**Example**:

```bash
curl http://localhost:9090/health
```

### Readiness Check

**Endpoint**: `GET /ready`

**Description**: Readiness probe that indicates if the service is ready to accept traffic.

**Response**:

```json
{
  "status": "ready",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "listener": "ready",
    "storage": "ready",
    "mib_manager": "ready",
    "correlator": "ready",
    "notifier": "ready"
  }
}
```

**Status Codes**:

- `200 OK`: Service is ready
- `503 Service Unavailable`: Service is not ready

**Component States**:

- `ready`: Component is operational
- `starting`: Component is initializing
- `error`: Component has failed

**Example**:

```bash
curl http://localhost:9090/ready
```

## Metrics Endpoint

### Prometheus Metrics

**Endpoint**: `GET /metrics`

**Description**: Prometheus-compatible metrics endpoint.

**Content-Type**: `text/plain; version=0.0.4; charset=utf-8`

**Example**:

```bash
curl http://localhost:9090/metrics
```

## Metrics Reference

### SNMP Trap Metrics

#### nereus_traps_received_total

**Type**: Counter
**Description**: Total number of SNMP traps received
**Labels**:

- `source_ip`: Source IP address of the trap
- `community`: SNMP community string

```promql
nereus_traps_received_total{source_ip="192.168.1.100",community="public"} 1234
```

#### nereus_traps_processed_total
**Type**: Counter  
**Description**: Total number of SNMP traps successfully processed  
**Labels**:
- `source_ip`: Source IP address
- `trap_oid`: Trap OID

```
nereus_traps_processed_total{source_ip="192.168.1.100",trap_oid="1.3.6.1.4.1.1"} 1200
```

#### nereus_traps_failed_total
**Type**: Counter  
**Description**: Total number of SNMP traps that failed processing  
**Labels**:
- `source_ip`: Source IP address
- `error_type`: Type of error (validation, parsing, processing)

```
nereus_traps_failed_total{source_ip="192.168.1.100",error_type="validation"} 34
```

#### nereus_trap_processing_duration_seconds
**Type**: Histogram  
**Description**: Time spent processing SNMP traps  
**Labels**:
- `source_ip`: Source IP address

```
nereus_trap_processing_duration_seconds_bucket{source_ip="192.168.1.100",le="0.001"} 800
nereus_trap_processing_duration_seconds_bucket{source_ip="192.168.1.100",le="0.01"} 1150
nereus_trap_processing_duration_seconds_sum{source_ip="192.168.1.100"} 12.5
nereus_trap_processing_duration_seconds_count{source_ip="192.168.1.100"} 1200
```

### Event Correlation Metrics

#### nereus_events_correlated_total
**Type**: Counter  
**Description**: Total number of events after correlation processing  
**Labels**:
- `correlation_type`: Type of correlation (new, duplicate, resolved)

```
nereus_events_correlated_total{correlation_type="new"} 800
nereus_events_correlated_total{correlation_type="duplicate"} 400
```

#### nereus_correlation_groups_active
**Type**: Gauge  
**Description**: Number of active correlation groups  

```
nereus_correlation_groups_active 45
```

#### nereus_correlation_duration_seconds
**Type**: Histogram  
**Description**: Time spent on event correlation  

```
nereus_correlation_duration_seconds_bucket{le="0.001"} 950
nereus_correlation_duration_seconds_sum 8.2
nereus_correlation_duration_seconds_count 1000
```

### Storage Metrics

#### nereus_storage_operations_total
**Type**: Counter  
**Description**: Total number of storage operations  
**Labels**:
- `operation`: Type of operation (insert, update, delete, select)
- `status`: Operation status (success, error)

```
nereus_storage_operations_total{operation="insert",status="success"} 1200
nereus_storage_operations_total{operation="select",status="success"} 3400
```

#### nereus_storage_duration_seconds
**Type**: Histogram  
**Description**: Time spent on storage operations  
**Labels**:
- `operation`: Type of operation

```
nereus_storage_duration_seconds_bucket{operation="insert",le="0.01"} 1180
nereus_storage_duration_seconds_sum{operation="insert"} 15.2
nereus_storage_duration_seconds_count{operation="insert"} 1200
```

#### nereus_storage_events_total
**Type**: Gauge  
**Description**: Total number of events in storage  

```
nereus_storage_events_total 15420
```

### Webhook Metrics

#### nereus_webhooks_sent_total
**Type**: Counter  
**Description**: Total number of webhook deliveries attempted  
**Labels**:
- `webhook`: Webhook name
- `status`: HTTP status code or error type

```
nereus_webhooks_sent_total{webhook="alertmanager",status="200"} 980
nereus_webhooks_sent_total{webhook="alertmanager",status="timeout"} 20
```

#### nereus_webhook_duration_seconds
**Type**: Histogram  
**Description**: Time spent delivering webhooks  
**Labels**:
- `webhook`: Webhook name

```
nereus_webhook_duration_seconds_bucket{webhook="alertmanager",le="1.0"} 950
nereus_webhook_duration_seconds_sum{webhook="alertmanager"} 450.2
nereus_webhook_duration_seconds_count{webhook="alertmanager"} 1000
```

#### nereus_webhook_retries_total
**Type**: Counter  
**Description**: Total number of webhook retry attempts  
**Labels**:
- `webhook`: Webhook name
- `attempt`: Retry attempt number

```
nereus_webhook_retries_total{webhook="alertmanager",attempt="1"} 50
nereus_webhook_retries_total{webhook="alertmanager",attempt="2"} 15
```

### MIB Management Metrics

#### nereus_mib_files_loaded_total
**Type**: Gauge  
**Description**: Number of MIB files currently loaded  

```
nereus_mib_files_loaded_total 25
```

#### nereus_mib_oid_resolutions_total
**Type**: Counter  
**Description**: Total number of OID resolution attempts  
**Labels**:
- `status`: Resolution status (success, not_found, error)

```
nereus_mib_oid_resolutions_total{status="success"} 8500
nereus_mib_oid_resolutions_total{status="not_found"} 150
```

#### nereus_mib_cache_hits_total
**Type**: Counter  
**Description**: Number of OID resolution cache hits  

```
nereus_mib_cache_hits_total 7800
```

#### nereus_mib_cache_size
**Type**: Gauge  
**Description**: Current size of the OID resolution cache  

```
nereus_mib_cache_size 2500
```

### System Metrics

#### nereus_build_info
**Type**: Gauge  
**Description**: Build information  
**Labels**:
- `version`: Application version
- `commit`: Git commit hash
- `build_time`: Build timestamp

```
nereus_build_info{version="1.0.0",commit="abc123",build_time="20240115103000"} 1
```

#### nereus_start_time_seconds
**Type**: Gauge  
**Description**: Unix timestamp of when the application started  

```
nereus_start_time_seconds 1705315800
```

#### nereus_goroutines
**Type**: Gauge  
**Description**: Number of goroutines currently running  

```
nereus_goroutines 45
```

#### nereus_memory_usage_bytes
**Type**: Gauge  
**Description**: Memory usage in bytes  
**Labels**:
- `type`: Memory type (heap, stack, sys)

```
nereus_memory_usage_bytes{type="heap"} 25165824
nereus_memory_usage_bytes{type="sys"} 71303168
```

## Configuration API

### Configuration Validation

While Nereus doesn't expose a REST API for configuration, it provides CLI commands for configuration management:

```bash
# Validate configuration
./nereus validate --config config.yaml

# Generate sample configuration
./nereus generate --output config.yaml
```

## Webhook Payload Formats

### Alertmanager Format

When configured with `format: "alertmanager"`, Nereus sends webhooks in Alertmanager format:

```json
[
  {
    "labels": {
      "alertname": "SNMPTrap",
      "source_ip": "192.168.1.100",
      "trap_oid": "1.3.6.1.4.1.1",
      "trap_name": "linkDown",
      "severity": "critical",
      "correlation_id": "abc123"
    },
    "annotations": {
      "summary": "SNMP trap received from 192.168.1.100",
      "description": "Link down trap received from network device",
      "varbinds": "ifIndex=1, ifDescr=eth0",
      "timestamp": "2024-01-15T10:30:00Z"
    },
    "startsAt": "2024-01-15T10:30:00Z",
    "endsAt": "0001-01-01T00:00:00Z",
    "generatorURL": "http://nereus:9090/events/abc123"
  }
]
```

### Custom Format

When configured with `format: "custom"`, you can define custom webhook payloads using templates.

## Error Responses

### Standard Error Format

```json
{
  "error": "error description",
  "timestamp": "2024-01-15T10:30:00Z",
  "path": "/health",
  "status": 503
}
```

### Common Error Codes

- `400 Bad Request`: Invalid request format
- `404 Not Found`: Endpoint not found
- `500 Internal Server Error`: Server error
- `503 Service Unavailable`: Service not ready

## Rate Limiting

Currently, Nereus doesn't implement rate limiting on HTTP endpoints. However, SNMP trap processing includes built-in rate limiting and buffering to handle high-throughput scenarios.

## Authentication

HTTP endpoints are currently unauthenticated. For production deployments, consider:

1. **Network Security**: Restrict access using firewall rules
2. **Reverse Proxy**: Use nginx or similar for authentication
3. **VPN/Private Network**: Deploy in private network segments

## Monitoring Integration

### Prometheus Configuration

Add Nereus to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'nereus'
    static_configs:
      - targets: ['nereus:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Grafana Dashboard

Key metrics to monitor in Grafana:
- SNMP trap reception rate
- Event processing latency
- Webhook delivery success rate
- Storage operation performance
- System resource usage

### Alerting Rules

Example Prometheus alerting rules:

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
          summary: "Nereus is down"
          
      - alert: HighTrapProcessingLatency
        expr: histogram_quantile(0.95, nereus_trap_processing_duration_seconds_bucket) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High SNMP trap processing latency"
```

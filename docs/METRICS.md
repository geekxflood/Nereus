# Metrics and Monitoring

Nereus provides comprehensive Prometheus metrics for monitoring SNMP trap processing, system performance, and application health.

## Overview

The metrics system exposes detailed statistics about:
- SNMP trap processing performance
- Storage operations and database health
- Webhook delivery success rates and timing
- System resource usage (memory, CPU, goroutines)
- Event correlation statistics
- Application health and readiness

## Configuration

Enable metrics in your configuration file:

```yaml
metrics:
  enabled: true                    # Enable/disable metrics collection
  listen_address: ":9090"         # Address to bind metrics server
  metrics_path: "/metrics"        # Prometheus metrics endpoint
  health_path: "/health"          # Health check endpoint
  ready_path: "/ready"            # Readiness check endpoint
  update_interval: "30s"          # System metrics update frequency
  namespace: "nereus"             # Prometheus metrics namespace
```

## Endpoints

### Metrics Endpoint (`/metrics`)
Exposes Prometheus-compatible metrics in text format.

**Example:**
```
curl http://localhost:9090/metrics
```

### Health Check Endpoint (`/health`)
Returns application health status:
- `200 OK` - All components are healthy
- `503 Service Unavailable` - One or more components are unhealthy

**Example:**
```bash
curl http://localhost:9090/health
# Response: OK or UNHEALTHY
```

### Readiness Check (`/ready`)
Returns application readiness status:
- `200 OK` - Application is ready to serve traffic
- `503 Service Unavailable` - Application is not ready

**Example:**
```bash
curl http://localhost:9090/ready
# Response: READY or NOT READY
```

## Available Metrics

### SNMP Trap Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|---------|
| `nereus_traps_received_total` | Counter | Total SNMP traps received | - |
| `nereus_traps_processed_total` | Counter | Total SNMP traps successfully processed | - |
| `nereus_traps_failed_total` | Counter | Total SNMP traps that failed processing | - |
| `nereus_trap_processing_duration_seconds` | Histogram | Time spent processing SNMP traps | - |
| `nereus_trap_packet_size_bytes` | Histogram | Size of SNMP trap packets | - |
| `nereus_traps_by_source_total` | Counter | Total traps by source IP | `source_ip` |
| `nereus_traps_by_type_total` | Counter | Total traps by trap type | `trap_oid`, `trap_name` |

### Storage Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|---------|
| `nereus_events_stored_total` | Counter | Total events stored in database | - |
| `nereus_storage_errors_total` | Counter | Total storage operation errors | - |
| `nereus_storage_query_duration_seconds` | Histogram | Database query execution time | - |
| `nereus_database_size_bytes` | Gauge | Current database file size | - |
| `nereus_storage_active_queries` | Gauge | Number of active database queries | - |
| `nereus_events_retained_total` | Gauge | Total events currently in storage | - |

### Webhook Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|---------|
| `nereus_webhooks_delivered_total` | Counter | Total webhooks successfully delivered | - |
| `nereus_webhooks_failed_total` | Counter | Total webhook delivery failures | - |
| `nereus_webhook_delivery_duration_seconds` | Histogram | Webhook delivery time | - |
| `nereus_webhook_retries_total` | Counter | Total webhook retry attempts | - |
| `nereus_webhook_queue_length` | Gauge | Current webhook delivery queue length | - |
| `nereus_webhook_active_workers` | Gauge | Number of active webhook workers | - |
| `nereus_webhooks_by_status_total` | Counter | Webhooks by HTTP status code | `status_code`, `webhook_name` |

### System Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|---------|
| `nereus_cpu_usage_percent` | Gauge | Current CPU usage percentage | - |
| `nereus_memory_usage_bytes` | Gauge | Current memory usage in bytes | - |
| `nereus_goroutines_total` | Gauge | Current number of goroutines | - |
| `nereus_gc_duration_seconds` | Histogram | Garbage collection duration | - |
| `nereus_uptime_seconds` | Gauge | Application uptime in seconds | - |
| `nereus_file_descriptors_total` | Gauge | Number of open file descriptors | - |

### Correlation Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|---------|
| `nereus_events_correlated_total` | Counter | Total events processed by correlator | - |
| `nereus_correlation_rules_total` | Gauge | Number of active correlation rules | - |
| `nereus_correlation_groups_active` | Gauge | Number of active correlation groups | - |
| `nereus_flapping_events_total` | Counter | Total flapping events detected | - |
| `nereus_correlation_duration_seconds` | Histogram | Event correlation processing time | - |
| `nereus_correlation_rule_evaluations_total` | Counter | Total correlation rule evaluations | - |

## Prometheus Configuration

Add Nereus to your Prometheus configuration:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'nereus'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 30s
    metrics_path: /metrics
```

## Grafana Dashboard

### Key Panels to Include

1. **SNMP Trap Processing Rate**
   ```promql
   rate(nereus_traps_received_total[5m])
   ```

2. **Trap Processing Success Rate**
   ```promql
   rate(nereus_traps_processed_total[5m]) / rate(nereus_traps_received_total[5m]) * 100
   ```

3. **Average Processing Time**
   ```promql
   rate(nereus_trap_processing_duration_seconds_sum[5m]) / rate(nereus_trap_processing_duration_seconds_count[5m])
   ```

4. **Top Source IPs**
   ```promql
   topk(10, rate(nereus_traps_by_source_total[5m]))
   ```

5. **Webhook Delivery Success Rate**
   ```promql
   rate(nereus_webhooks_delivered_total[5m]) / (rate(nereus_webhooks_delivered_total[5m]) + rate(nereus_webhooks_failed_total[5m])) * 100
   ```

6. **Memory Usage**
   ```promql
   nereus_memory_usage_bytes
   ```

7. **Database Size Growth**
   ```promql
   nereus_database_size_bytes
   ```

## Alerting Rules

### Example Prometheus Alerting Rules

```yaml
# nereus-alerts.yml
groups:
  - name: nereus
    rules:
      - alert: NereusHighTrapFailureRate
        expr: rate(nereus_traps_failed_total[5m]) / rate(nereus_traps_received_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High SNMP trap failure rate"
          description: "Nereus is failing to process {{ $value | humanizePercentage }} of SNMP traps"

      - alert: NereusWebhookDeliveryFailing
        expr: rate(nereus_webhooks_failed_total[5m]) > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Webhook delivery failures detected"
          description: "Nereus webhook delivery is failing at {{ $value }} failures per second"

      - alert: NereusHighMemoryUsage
        expr: nereus_memory_usage_bytes > 1073741824  # 1GB
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Nereus memory usage is {{ $value | humanizeBytes }}"

      - alert: NereusApplicationDown
        expr: up{job="nereus"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Nereus application is down"
          description: "Nereus SNMP trap listener is not responding"
```

## Health Monitoring

The application provides health and readiness endpoints for Kubernetes and other orchestration systems:

### Kubernetes Probes

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: nereus
    image: nereus:latest
    ports:
    - containerPort: 9090
      name: metrics
    livenessProbe:
      httpGet:
        path: /health
        port: 9090
      initialDelaySeconds: 30
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /ready
        port: 9090
      initialDelaySeconds: 5
      periodSeconds: 5
```

## Troubleshooting

### Common Issues

1. **Metrics endpoint not accessible**
   - Check if `metrics.enabled` is set to `true`
   - Verify `metrics.listen_address` is correct
   - Ensure firewall allows access to the metrics port

2. **Missing metrics**
   - Check application logs for initialization errors
   - Verify Prometheus is scraping the correct endpoint
   - Ensure metrics namespace matches your queries

3. **High memory usage**
   - Monitor `nereus_memory_usage_bytes` metric
   - Check for memory leaks in long-running processes
   - Consider adjusting retention policies

### Debug Commands

```bash
# Check if metrics endpoint is responding
curl -I http://localhost:9090/metrics

# View raw metrics output
curl http://localhost:9090/metrics | grep nereus

# Check application health
curl http://localhost:9090/health

# Check readiness status
curl http://localhost:9090/ready
```

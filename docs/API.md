# Nereus API Documentation

This document describes the HTTP APIs exposed by Nereus for monitoring, health checks, and management operations.

## Table of Contents

- [Health and Status Endpoints](#health-and-status-endpoints)
- [Metrics Endpoint](#metrics-endpoint)
- [Configuration Management](#configuration-management)
- [Event Management](#event-management)
- [Component Statistics](#component-statistics)
- [Hot Reload](#hot-reload)
- [Authentication](#authentication)
- [Error Responses](#error-responses)

## Health and Status Endpoints

### Health Check

**Endpoint:** `GET /health`

**Description:** Returns the overall health status of the application.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2023-12-01T10:30:00Z",
  "uptime": "2h30m15s",
  "version": "1.0.0",
  "components": {
    "listener": "healthy",
    "storage": "healthy",
    "correlator": "healthy",
    "notifier": "healthy",
    "metrics": "healthy"
  }
}
```

**Status Codes:**
- `200 OK` - Application is healthy
- `503 Service Unavailable` - Application is unhealthy

### Readiness Check

**Endpoint:** `GET /ready`

**Description:** Returns whether the application is ready to accept traffic.

**Response:**
```json
{
  "ready": true,
  "timestamp": "2023-12-01T10:30:00Z",
  "components": {
    "listener": true,
    "storage": true,
    "correlator": true,
    "notifier": true
  }
}
```

**Status Codes:**
- `200 OK` - Application is ready
- `503 Service Unavailable` - Application is not ready

### Live Check

**Endpoint:** `GET /live`

**Description:** Returns whether the application is alive (for Kubernetes liveness probes).

**Response:**
```json
{
  "alive": true,
  "timestamp": "2023-12-01T10:30:00Z"
}
```

**Status Codes:**
- `200 OK` - Application is alive
- `503 Service Unavailable` - Application is not responding

## Metrics Endpoint

### Prometheus Metrics

**Endpoint:** `GET /metrics`

**Description:** Returns Prometheus-formatted metrics for monitoring.

**Response:** Prometheus text format

**Key Metrics:**
- `nereus_traps_received_total` - Total number of SNMP traps received
- `nereus_traps_processed_total` - Total number of traps successfully processed
- `nereus_traps_dropped_total` - Total number of traps dropped due to errors
- `nereus_webhooks_sent_total` - Total number of webhooks sent
- `nereus_webhook_errors_total` - Total number of webhook errors
- `nereus_correlation_groups_active` - Number of active correlation groups
- `nereus_storage_events_total` - Total number of events in storage
- `nereus_memory_usage_bytes` - Memory usage in bytes
- `nereus_goroutines_total` - Number of active goroutines

## Configuration Management

### Get Configuration

**Endpoint:** `GET /api/v1/config`

**Description:** Returns the current application configuration (sensitive values masked).

**Response:**
```json
{
  "app": {
    "name": "nereus-snmp-listener",
    "version": "1.0.0",
    "log_level": "info"
  },
  "listener": {
    "bind_address": "0.0.0.0:162",
    "community": "***",
    "workers": 4,
    "buffer_size": 1000
  },
  "storage": {
    "type": "sqlite",
    "connection_string": "/var/lib/nereus/nereus.db",
    "retention_days": 30
  }
}
```

### Validate Configuration

**Endpoint:** `POST /api/v1/config/validate`

**Description:** Validates a configuration without applying it.

**Request Body:**
```json
{
  "config": {
    "app": {
      "log_level": "debug"
    },
    "listener": {
      "workers": 8
    }
  }
}
```

**Response:**
```json
{
  "valid": true,
  "errors": [],
  "warnings": [
    "High worker count may impact performance"
  ]
}
```

## Event Management

### List Events

**Endpoint:** `GET /api/v1/events`

**Description:** Returns a paginated list of recent events.

**Query Parameters:**
- `limit` (int) - Maximum number of events to return (default: 100, max: 1000)
- `offset` (int) - Number of events to skip (default: 0)
- `since` (string) - ISO 8601 timestamp to filter events after
- `until` (string) - ISO 8601 timestamp to filter events before
- `source` (string) - Filter by source IP address
- `trap_oid` (string) - Filter by trap OID
- `severity` (string) - Filter by severity level

**Example:** `GET /api/v1/events?limit=50&since=2023-12-01T00:00:00Z&severity=critical`

**Response:**
```json
{
  "events": [
    {
      "id": "evt_123456789",
      "timestamp": "2023-12-01T10:30:00Z",
      "source_ip": "192.168.1.100",
      "trap_oid": "1.3.6.1.4.1.12345.1.1",
      "trap_name": "linkDown",
      "severity": "critical",
      "message": "Interface eth0 is down",
      "varbinds": {
        "1.3.6.1.2.1.1.1.0": "Router-01",
        "1.3.6.1.2.1.1.5.0": "eth0"
      },
      "correlation_id": "corr_987654321",
      "acknowledged": false,
      "resolved": false
    }
  ],
  "pagination": {
    "total": 1250,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

### Get Event Details

**Endpoint:** `GET /api/v1/events/{event_id}`

**Description:** Returns detailed information about a specific event.

**Response:**
```json
{
  "id": "evt_123456789",
  "timestamp": "2023-12-01T10:30:00Z",
  "source_ip": "192.168.1.100",
  "trap_oid": "1.3.6.1.4.1.12345.1.1",
  "trap_name": "linkDown",
  "severity": "critical",
  "message": "Interface eth0 is down",
  "varbinds": {
    "1.3.6.1.2.1.1.1.0": "Router-01",
    "1.3.6.1.2.1.1.5.0": "eth0"
  },
  "correlation_id": "corr_987654321",
  "correlation_group": {
    "id": "corr_987654321",
    "created_at": "2023-12-01T10:30:00Z",
    "event_count": 3,
    "last_seen": "2023-12-01T10:35:00Z"
  },
  "acknowledged": false,
  "acknowledged_by": null,
  "acknowledged_at": null,
  "resolved": false,
  "resolved_at": null,
  "webhooks_sent": [
    {
      "endpoint": "alertmanager",
      "sent_at": "2023-12-01T10:30:01Z",
      "status": "success",
      "response_code": 200
    }
  ]
}
```

### Acknowledge Event

**Endpoint:** `POST /api/v1/events/{event_id}/acknowledge`

**Description:** Acknowledges an event to indicate it has been seen.

**Request Body:**
```json
{
  "acknowledged_by": "admin@example.com",
  "note": "Investigating network connectivity issue"
}
```

**Response:**
```json
{
  "success": true,
  "acknowledged_at": "2023-12-01T10:45:00Z"
}
```

## Component Statistics

### Listener Statistics

**Endpoint:** `GET /api/v1/stats/listener`

**Description:** Returns SNMP listener statistics.

**Response:**
```json
{
  "packets_received": 15420,
  "packets_processed": 15380,
  "packets_dropped": 40,
  "bytes_received": 2048576,
  "active_workers": 4,
  "queue_size": 12,
  "average_processing_time_ms": 2.5,
  "uptime": "2h30m15s"
}
```

### Storage Statistics

**Endpoint:** `GET /api/v1/stats/storage`

**Description:** Returns storage system statistics.

**Response:**
```json
{
  "total_events": 125000,
  "events_last_24h": 3420,
  "database_size_mb": 45.2,
  "oldest_event": "2023-11-01T00:00:00Z",
  "newest_event": "2023-12-01T10:30:00Z",
  "cleanup_last_run": "2023-12-01T02:00:00Z",
  "cleanup_events_removed": 1250
}
```

### Correlator Statistics

**Endpoint:** `GET /api/v1/stats/correlator`

**Description:** Returns event correlation statistics.

**Response:**
```json
{
  "active_groups": 45,
  "total_groups_created": 8920,
  "events_correlated": 12450,
  "correlation_rate": 0.85,
  "average_group_size": 2.3,
  "rules_active": 12,
  "rules_matched": 8450
}
```

### Notifier Statistics

**Endpoint:** `GET /api/v1/stats/notifier`

**Description:** Returns webhook notification statistics.

**Response:**
```json
{
  "webhooks_sent": 8920,
  "webhooks_successful": 8850,
  "webhooks_failed": 70,
  "success_rate": 0.992,
  "average_response_time_ms": 125,
  "queue_size": 5,
  "active_workers": 2,
  "endpoints": {
    "alertmanager": {
      "sent": 4500,
      "successful": 4485,
      "failed": 15,
      "last_success": "2023-12-01T10:29:00Z",
      "last_failure": "2023-12-01T09:15:00Z"
    }
  }
}
```

## Hot Reload

### Reload Configuration

**Endpoint:** `POST /api/v1/reload`

**Description:** Triggers a hot reload of the configuration.

**Request Body:**
```json
{
  "component": "all"
}
```

**Response:**
```json
{
  "success": true,
  "reloaded_at": "2023-12-01T10:45:00Z",
  "components_reloaded": [
    "listener",
    "correlator",
    "notifier"
  ],
  "warnings": []
}
```

### Reload MIB Files

**Endpoint:** `POST /api/v1/reload/mibs`

**Description:** Triggers a reload of MIB files.

**Response:**
```json
{
  "success": true,
  "reloaded_at": "2023-12-01T10:45:00Z",
  "mibs_loaded": 25,
  "oids_resolved": 1250,
  "errors": []
}
```

## Authentication

Currently, Nereus does not implement authentication for its management APIs. In production environments, it is recommended to:

1. Use a reverse proxy (nginx, Apache) with authentication
2. Restrict access using firewall rules
3. Use VPN or private networks for management access
4. Consider implementing API keys in future versions

## Error Responses

All API endpoints return consistent error responses:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "The request is invalid",
    "details": "Missing required parameter 'event_id'",
    "timestamp": "2023-12-01T10:30:00Z",
    "request_id": "req_123456789"
  }
}
```

**Common Error Codes:**
- `INVALID_REQUEST` - Malformed or invalid request
- `NOT_FOUND` - Requested resource not found
- `INTERNAL_ERROR` - Internal server error
- `SERVICE_UNAVAILABLE` - Service temporarily unavailable
- `RATE_LIMITED` - Too many requests

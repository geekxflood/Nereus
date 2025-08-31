# Nereus

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](Dockerfile)

Nereus is a production-ready SNMP trap alerting system designed for enterprise monitoring environments. It provides intelligent event correlation, MIB-based parsing, and flexible notification delivery with high performance and reliability.

## 🚀 Key Features

### 📡 **Enterprise SNMP Processing**

- **SNMPv2c Support**: Full SNMPv2c trap processing with validation
- **High-Performance Parsing**: Concurrent trap processing with configurable worker pools
- **MIB Integration**: Automatic OID resolution using standard and custom MIB files
- **Hot Reload**: Dynamic MIB and configuration reloading without service restart

### 🧠 **Intelligent Event Correlation**

- **Deduplication**: Automatic duplicate event detection and suppression
- **Event Grouping**: Intelligent correlation of related events
- **Flapping Detection**: Automatic detection and handling of oscillating events
- **Severity Mapping**: Configurable severity assignment based on trap OIDs

### 🔔 **Flexible Notification System**

- **Multiple Formats**: Alertmanager, Prometheus, and custom webhook formats
- **Template Engine**: CUE-based templating for custom notification payloads
- **Retry Logic**: Exponential backoff with circuit breaker patterns
- **Multi-Destination**: Support for multiple webhook endpoints with filtering

### 📊 **Observability & Monitoring**

- **Prometheus Metrics**: Comprehensive metrics for all system components
- **Health Checks**: Kubernetes-ready health and readiness endpoints
- **Structured Logging**: JSON-formatted logs with configurable levels
- **Performance Tracking**: Detailed timing and throughput metrics

### 🏗️ **Production Ready**

- **High Availability**: Stateless design for horizontal scaling
- **Persistent Storage**: SQLite-based event storage with configurable retention
- **Configuration Management**: CUE schema validation with hot reload
- **Container Support**: Multi-stage Docker builds with security best practices

## 📋 Quick Start

### Prerequisites

- Go 1.25+ (for building from source)
- Docker (for containerized deployment)
- Standard SNMP MIB files (SNMPv2-SMI, SNMPv2-TC, SNMPv2-MIB)

### Installation Options

#### 🐳 Docker (Recommended)

```bash
# Pull and run the latest image
docker run -d \
  --name nereus \
  -p 162:162/udp \
  -p 9090:9090 \
  -v /path/to/config:/etc/nereus \
  -v /path/to/mibs:/usr/share/snmp/mibs \
  geekxflood/nereus:latest
```

#### 🔨 Build from Source

```bash
git clone https://github.com/geekxflood/nereus.git
cd nereus
go build -ldflags "-X main.version=$(git describe --tags)" -o nereus ./main.go
./nereus --config examples/config.yaml
```

## 📖 Documentation

| Topic             | Description                        | Link                                         |
| ----------------- | ---------------------------------- | -------------------------------------------- |
| **Configuration** | Complete configuration reference   | [docs/configuration/](docs/configuration.md) |
| **Architecture**  | System design and components       | [docs/architecture/](docs/architecture)      |
| **Deployment**    | Production deployment guides       | [docs/deployment/](docs/deployment.md)       |
| **Development**   | Contributing and development setup | [docs/development/](docs/development.md)     |
| **API Reference** | REST API and metrics endpoints     | [docs/api/](docs/api.md)                     |

## ⚡ Quick Configuration

Generate a sample configuration file:

```bash
./nereus generate --output config.yaml
```

Basic configuration example:

```yaml
# config.yaml
app:
  name: "nereus-snmp-listener"
  version: "1.0.0"

server:
  host: "0.0.0.0"
  port: 162
  buffer_size: 65536

mib:
  directories: ["/usr/share/snmp/mibs"]
  enable_hot_reload: true
  required_mibs: ["SNMPv2-SMI", "SNMPv2-TC", "SNMPv2-MIB"]

correlator:
  enable_correlation: true
  deduplication_window: "5m"
  correlation_window: "10m"

notifier:
  enable_notifications: true
  default_webhooks:
    - name: "alertmanager"
      url: "http://alertmanager:9093/api/v1/alerts"
      format: "alertmanager"
      timeout: "30s"

storage:
  connection_string: "./nereus_events.db"
  retention_days: 30

metrics:
  enabled: true
  listen_address: ":9090"

logging:
  level: "info"
  format: "json"
```

Validate your configuration:

```bash
./nereus validate --config config.yaml
```

## 🚀 Usage

### Command Line Interface

```bash
# Start the SNMP trap listener
./nereus --config config.yaml

# Generate sample configuration
./nereus generate --output config.yaml

# Validate configuration and MIB files
./nereus validate --config config.yaml --check-mibs

# Show version information
./nereus --version

# Show help
./nereus --help
```

### Docker Compose

```yaml
version: '3.8'
services:
  nereus:
    image: geekxflood/nereus:latest
    ports:
      - "162:162/udp"  # SNMP trap port
      - "9090:9090"    # Metrics port
    volumes:
      - ./config.yaml:/etc/nereus/config.yaml
      - ./mibs:/usr/share/snmp/mibs
      - ./data:/var/lib/nereus
    environment:
      - NEREUS_LOG_LEVEL=info
    restart: unless-stopped

  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nereus
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nereus
  template:
    metadata:
      labels:
        app: nereus
    spec:
      containers:
      - name: nereus
        image: geekxflood/nereus:latest
        ports:
        - containerPort: 162
          protocol: UDP
        - containerPort: 9090
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
        readinessProbe:
          httpGet:
            path: /ready
            port: 9090
```

## 🏗️ Architecture

Nereus follows a modular architecture with clear separation of concerns:

```txt
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

### Core Components

- **Listener**: SNMPv2c trap reception and validation
- **MIB Manager**: OID resolution and MIB file management
- **Event Processor**: Trap parsing and enrichment
- **Correlator**: Event deduplication and correlation
- **Notifier**: Webhook delivery with retry logic
- **Storage**: Event persistence and retention
- **Metrics**: Prometheus metrics and health monitoring

## 📊 Monitoring & Metrics

Nereus exposes comprehensive metrics on port 9090:

### Key Metrics

- `nereus_traps_received_total`: Total SNMP traps received
- `nereus_traps_processed_total`: Total traps successfully processed
- `nereus_events_correlated_total`: Total events after correlation
- `nereus_webhooks_sent_total`: Total webhook deliveries attempted
- `nereus_webhook_duration_seconds`: Webhook delivery latency
- `nereus_storage_operations_total`: Database operation counters

### Health Endpoints

- `GET /health`: Liveness probe (always returns 200 if running)
- `GET /ready`: Readiness probe (checks all components)
- `GET /metrics`: Prometheus metrics endpoint

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](docs/CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/geekxflood/nereus.git
cd nereus

# Install dependencies
go mod download

# Run tests
go test ./...

# Build for development
go build -o nereus ./main.go

# Run with sample configuration
./nereus --config examples/config.yaml
```

### Code Quality

- **Go Version**: 1.25+
- **Testing**: Comprehensive unit and integration tests
- **Linting**: golangci-lint with strict rules
- **Documentation**: GoDoc for all public APIs
- **Security**: gosec security scanning

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

# Nereus

Nereus is an SNMP trap alerting system designed to monitor, parse, and manage alerts with intelligent event correlation and notification capabilities.

## Features

### 🔔 SNMP Trap Management

- **Real-time Trap Reception**: Listens for SNMP traps
- **MIB-based OID Parsing**: Automatically parses Object Identifiers using MIB files
- **Event Registration**: Captures and registers trap events with full context
- **Intelligent Resolution**: Correlates resolve trap events to automatically close alerts

### 🔍 Advanced Parsing & Correlation

- **MIB File Support**: Load MIB files from configurable folder paths
- **OID Resolution**: Translate numeric OIDs to human-readable names and descriptions
- **Event Correlation**: Link related trap events for comprehensive incident tracking
- **Context Preservation**: Maintain full trap data including varbinds and timestamps

### 🚨 Alert Notifications

- **Prometheus Integration**: Native support for Prometheus alert format and Alertmanager
- **Webhook Integration**: Send alerts to external systems via HTTP webhooks
- **Multiple Formats**: Support for Prometheus, Alertmanager, and custom template formats
- **Configurable Templates**: Customize notification payloads and formats
- **Multiple Endpoints**: Support for multiple webhook destinations
- **Retry Logic**: Built-in retry mechanisms for reliable delivery

## Installation

### Prerequisites

- Go 1.25 or later
- Access to MIB files

### Build from Source

```bash
git clone https://github.com/geekxflood/nereus.git
cd nereus
go build -o nereus ./main.go
```

### Docker

```bash
docker build -t nereus .
docker run -p 162:162/udp nereus
```

## Configuration

Nereus uses CUE-based configuration for type-safe and validated settings:

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 162
  community: "public"

mibs:
  path: "/opt/mibs"
  auto_load: true

webhooks:
  - name: "alertmanager"
    url: "http://alertmanager:9093/api/v2/alerts"
    insecure: false
    timeout: "30s"
    retry_count: 3

logging:
  level: "info"
  format: "json"
  component: "nereus"
```

### Prometheus Integration

Nereus provides native support for Prometheus alert format, allowing you to send SNMP traps directly to Alertmanager or other Prometheus-compatible systems.

#### Configuration Example

```yaml
notifier:
  default_webhooks:
    - name: "alertmanager"
      url: "http://alertmanager:9093/api/v1/alerts"
      method: "POST"
      format: "alertmanager"  # Use Prometheus alert format
      enabled: true
      timeout: "10s"
      content_type: "application/json"
```

#### Alert Format

SNMP traps are automatically converted to Prometheus alerts with:

- **Labels**: `alertname`, `source_ip`, `severity`, `trap_oid`, `trap_name`, `correlation_id`
- **Annotations**: `summary`, `description`, `varbinds`, `metadata`, timestamps
- **Timing**: `StartsAt` set to trap timestamp, `EndsAt` for resolved alerts

See `examples/prometheus-config.yml` for a complete Prometheus integration example.

### CUE-Based Template System

Nereus uses embedded CUE templates for type-safe, validated notification formatting:

#### Built-in Templates

- **default**: Standard JSON notification format
- **slack**: Slack webhook format with rich attachments
- **pagerduty**: PagerDuty Events API v2 format
- **email**: HTML email template with styling

#### Template Configuration

```yaml
notifier:
  default_webhooks:
    - name: "slack-alerts"
      url: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
      template: "slack"  # Uses embedded Slack CUE template
      format: "custom"
```

#### Template Features

- **Type Safety**: CUE validation ensures template correctness
- **Field Validation**: Required/optional field checking
- **Severity Mapping**: Automatic color coding and priority mapping
- **Usage Examples**: Built-in examples and documentation
- **Performance**: Compiled templates with caching

## Usage

### Basic Commands

```bash
# Start the SNMP trap listener
nereus

# Validate configuration
nereus validate --config config.yaml

# Generate sample configuration
nereus generate --output config.yaml
```

### Command Line Options

```bash
nereus [command]

Available Commands:
  generate    Generate sample configuration files
  validate    Validate configuration and MIB files
  help        Help about any command

Flags:
  -c, --config string   Configuration file path
  -h, --help           Help for nereus
  -v, --version        Version information
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  nereus:
    image: nereus:latest
    ports:
      - "162:162/udp"
    privileged: true
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./mibs:/opt/mibs
    environment:
      - NEREUS_CONFIG=/app/config.yaml
    restart: unless-stopped
```

## Architecture

### Core Components

```text
nereus/
├── cmd/                 # CLI commands
│   ├── generate.go     # Configuration generation
│   ├── validate.go     # Configuration validation
│   └── schemas/        # CUE configuration schemas
├── internal/
│   └── helpers/        # Utility functions
└── main.go            # Application entry point
```

### Event Flow

1. **Trap Reception**: SNMP traps received
2. **MIB Parsing**: OIDs parsed using loaded MIB definitions
3. **Event Registration**: Trap data stored with correlation ID
4. **Alert Generation**: Webhook notifications sent to configured endpoints
5. **Resolution Handling**: Resolve traps automatically close related alerts

### Dependencies

- **geekxflood/common**: Logging and configuration management
- **spf13/cobra**: CLI framework
- **CUE**: Configuration schema validation

## Configuration Schema

The configuration is validated against a CUE schema ensuring type safety:

```cue
server: {
  host: string | *"0.0.0.0"
  port: int & >=1 & <=65535 | *162
  community: string | *"public"
}

mibs: {
  path: string
  auto_load: bool | *true
}

webhooks: [...{
  name: string
  url: string
  timeout: string | *"30s"
  retry_count: int | *3
}]
```

## Development

### Testing

```bash
go test ./...
```

### Building

```bash
go build -o nereus ./main.go
```

### Code Quality

```bash
# Run linter
golangci-lint run

# Security scan
gosec ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go coding standards
- Add tests for new features
- Update documentation
- Use structured logging
- Validate configurations

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For support and questions:

- Open an issue on GitHub
- Check the documentation
- Review example configurations

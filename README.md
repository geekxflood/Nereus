# Nereus

Nereus is an advanced SNMP trap alerting system designed to monitor, parse, and manage alerts with intelligent event correlation and notification capabilities.

## Features

### 🔔 SNMP Trap Management

- **Real-time Trap Reception**: Listens for SNMP traps from network devices
- **MIB-based OID Parsing**: Automatically parses Object Identifiers using MIB files
- **Event Registration**: Captures and registers trap events with full context
- **Intelligent Resolution**: Correlates resolve trap events to automatically close alerts

### 🔍 Advanced Parsing & Correlation

- **MIB File Support**: Load MIB files from configurable folder paths
- **OID Resolution**: Translate numeric OIDs to human-readable names and descriptions
- **Event Correlation**: Link related trap events for comprehensive incident tracking
- **Context Preservation**: Maintain full trap data including varbinds and timestamps

### 🚨 Alert Notifications

- **Webhook Integration**: Send alerts to external systems via HTTP webhooks
- **Configurable Templates**: Customize notification payloads and formats
- **Multiple Endpoints**: Support for multiple webhook destinations
- **Retry Logic**: Built-in retry mechanisms for reliable delivery

### ⚙️ Enterprise Features

- **Structured Logging**: Comprehensive logging with the `geekxflood/common/logging` library
- **Configuration Management**: CUE-based configuration with hot reload with the `geekxflood/common/config` library
- **Security**: Safe file operations and path validation

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
    timeout: "30s"
    retry_count: 3

logging:
  level: "info"
  format: "json"
  component: "nereus"
```

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

1. **Trap Reception**: SNMP traps received on UDP port 162
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

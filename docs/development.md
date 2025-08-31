# Development Guide

This document provides comprehensive information for developers contributing to Nereus.

## Development Environment Setup

### Prerequisites

- **Go**: Version 1.25 or later
- **Git**: For version control
- **Docker**: For containerized testing (optional)
- **Make**: For build automation (optional)

### Local Setup

```bash
# Clone the repository
git clone https://github.com/geekxflood/nereus.git
cd nereus

# Install dependencies
go mod download

# Verify setup
go version
go mod verify

# Run tests
go test ./...

# Build the application
go build -o nereus ./main.go
```

### Development Dependencies

```bash
# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
go install golang.org/x/tools/cmd/goimports@latest
```

## Project Structure

```
nereus/
├── cmd/                    # CLI commands and schemas
│   ├── generate.go        # Configuration generation command
│   ├── validate.go        # Configuration validation command
│   └── schemas/           # CUE configuration schemas
├── internal/              # Core application packages
│   ├── app/              # Application orchestration
│   ├── correlator/       # Event correlation and deduplication
│   ├── events/           # Event processing pipeline
│   ├── infra/            # Infrastructure services
│   ├── listener/         # SNMP trap listener
│   ├── metrics/          # Prometheus metrics
│   ├── mib/              # MIB management and SNMP parsing
│   ├── notifier/         # Webhook notifications
│   ├── storage/          # Database operations
│   └── types/            # Common types and interfaces
├── examples/             # Configuration examples
├── docs/                 # Documentation
├── test/                 # Integration and load tests
├── Dockerfile           # Container build configuration
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
└── main.go             # Application entry point
```

## Architecture Principles

### Package Design

1. **Single Responsibility**: Each package has one clear purpose
2. **Minimal Dependencies**: Packages have minimal internal dependencies
3. **Interface-Based**: Components interact through well-defined interfaces
4. **Testability**: All packages can be tested independently

### Dependency Rules

- `types` package has zero dependencies (pure types)
- Most packages depend only on `types` and external libraries
- `app` package orchestrates all other components
- No circular dependencies allowed

### Code Organization

```go
// Package structure example
package mib

// Public interfaces first
type Manager interface {
    LoadMIBs(directories []string) error
    ResolveName(oid string) (string, error)
}

// Public types
type Config struct {
    Directories []string
    EnableHotReload bool
}

// Implementation
type manager struct {
    config Config
    cache  map[string]string
}

// Constructor
func NewManager(config Config) Manager {
    return &manager{
        config: config,
        cache:  make(map[string]string),
    }
}
```

## Development Workflow

### Git Workflow

1. **Fork and Clone**:
   ```bash
   git clone https://github.com/yourusername/nereus.git
   cd nereus
   git remote add upstream https://github.com/geekxflood/nereus.git
   ```

2. **Create Feature Branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make Changes**:
   - Write code following style guidelines
   - Add tests for new functionality
   - Update documentation as needed

4. **Test Changes**:
   ```bash
   go test ./...
   golangci-lint run
   gosec ./...
   ```

5. **Commit and Push**:
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   git push origin feature/your-feature-name
   ```

6. **Create Pull Request**:
   - Use descriptive title and description
   - Reference related issues
   - Ensure CI passes

### Commit Message Format

Follow conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test additions or changes
- `chore`: Build process or auxiliary tool changes

Examples:
```
feat(listener): add support for SNMPv3 authentication
fix(correlator): resolve memory leak in event grouping
docs(api): update webhook configuration examples
```

## Testing Guidelines

### Unit Testing

```go
// Example unit test
func TestManager_ResolveName(t *testing.T) {
    tests := []struct {
        name    string
        oid     string
        want    string
        wantErr bool
    }{
        {
            name: "valid OID resolution",
            oid:  "1.3.6.1.2.1.1.1.0",
            want: "sysDescr.0",
            wantErr: false,
        },
        {
            name: "invalid OID",
            oid:  "invalid",
            want: "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewManager(Config{})
            got, err := m.ResolveName(tt.oid)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("ResolveName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if got != tt.want {
                t.Errorf("ResolveName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Testing

```go
// Example integration test
func TestEventProcessingPipeline(t *testing.T) {
    // Setup test environment
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create test components
    storage := setupTestStorage(t)
    correlator := setupTestCorrelator(t)
    processor := setupTestProcessor(t, storage, correlator)

    // Test event processing
    event := createTestEvent()
    err := processor.ProcessEvent(ctx, event)
    
    assert.NoError(t, err)
    
    // Verify results
    storedEvents := storage.GetEvents()
    assert.Len(t, storedEvents, 1)
}
```

### Test Coverage

Maintain high test coverage:

```bash
# Run tests with coverage
go test -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Coverage requirements
# - Unit tests: >90% coverage
# - Integration tests: >80% coverage
# - Critical paths: 100% coverage
```

## Code Quality Standards

### Linting Configuration

`.golangci.yml`:
```yaml
linters-settings:
  govet:
    check-shadowing: true
  golint:
    min-confidence: 0
  gocyclo:
    min-complexity: 15
  maligned:
    suggest-new: true
  dupl:
    threshold: 100
  goconst:
    min-len: 2
    min-occurrences: 2

linters:
  enable:
    - bodyclose
    - deadcode
    - depguard
    - dogsled
    - dupl
    - errcheck
    - gochecknoinits
    - goconst
    - gocyclo
    - gofmt
    - goimports
    - golint
    - gosec
    - gosimple
    - govet
    - ineffassign
    - interfacer
    - maligned
    - misspell
    - nakedret
    - scopelint
    - staticcheck
    - structcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - varcheck
    - whitespace
```

### Code Style Guidelines

1. **Naming Conventions**:
   - Use descriptive names
   - Follow Go naming conventions
   - Use consistent terminology across packages

2. **Error Handling**:
   ```go
   // Good: Wrap errors with context
   if err != nil {
       return fmt.Errorf("failed to process event: %w", err)
   }
   
   // Bad: Ignore or lose error context
   if err != nil {
       return err
   }
   ```

3. **Logging**:
   ```go
   // Good: Structured logging with context
   logger.Info("processing SNMP trap",
       "source_ip", sourceIP,
       "trap_oid", trapOID,
       "component", "listener")
   
   // Bad: Unstructured logging
   log.Printf("Got trap from %s", sourceIP)
   ```

4. **Documentation**:
   ```go
   // Package documentation
   // Package mib provides MIB loading, parsing, and OID resolution functionality.
   package mib
   
   // Function documentation
   // ResolveName converts a numeric OID to its symbolic name using loaded MIB definitions.
   // It returns an error if the OID is invalid or not found in any loaded MIB.
   func (m *Manager) ResolveName(oid string) (string, error) {
       // Implementation
   }
   ```

## Performance Guidelines

### Optimization Principles

1. **Measure First**: Use profiling before optimizing
2. **Optimize Hot Paths**: Focus on frequently executed code
3. **Memory Efficiency**: Minimize allocations in hot paths
4. **Concurrency**: Use goroutines and channels appropriately

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Benchmark tests
func BenchmarkEventProcessing(b *testing.B) {
    processor := setupBenchmarkProcessor()
    event := createBenchmarkEvent()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        processor.ProcessEvent(context.Background(), event)
    }
}
```

### Memory Management

```go
// Good: Reuse buffers
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 1024)
    },
}

func processPacket(data []byte) error {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    
    // Use buffer for processing
    return nil
}

// Good: Limit goroutine creation
func (p *Processor) Start(ctx context.Context) error {
    for i := 0; i < p.workerCount; i++ {
        go p.worker(ctx)
    }
    return nil
}
```

## Security Guidelines

### Input Validation

```go
// Validate all inputs
func validateSNMPPacket(packet []byte) error {
    if len(packet) < minPacketSize {
        return fmt.Errorf("packet too small: %d bytes", len(packet))
    }
    
    if len(packet) > maxPacketSize {
        return fmt.Errorf("packet too large: %d bytes", len(packet))
    }
    
    return nil
}
```

### Error Handling

```go
// Don't expose internal details in errors
func (s *Storage) GetEvent(id string) (*Event, error) {
    event, err := s.db.Query("SELECT * FROM events WHERE id = ?", id)
    if err != nil {
        // Log detailed error internally
        s.logger.Error("database query failed", "error", err, "id", id)
        
        // Return generic error to caller
        return nil, fmt.Errorf("failed to retrieve event")
    }
    
    return event, nil
}
```

### Dependency Management

```bash
# Regular security updates
go get -u ./...
go mod tidy

# Security scanning
gosec ./...

# Dependency vulnerability scanning
go list -json -m all | nancy sleuth
```

## Release Process

### Version Management

1. **Semantic Versioning**: Follow semver (MAJOR.MINOR.PATCH)
2. **Git Tags**: Tag releases with version numbers
3. **Changelog**: Maintain detailed changelog

### Build Process

```bash
# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=v1.0.0" -o nereus-linux-amd64 ./main.go
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=v1.0.0" -o nereus-darwin-amd64 ./main.go
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=v1.0.0" -o nereus-windows-amd64.exe ./main.go
```

### Docker Release

```bash
# Build and tag Docker image
docker build -t geekxflood/nereus:v1.0.0 .
docker build -t geekxflood/nereus:latest .

# Push to registry
docker push geekxflood/nereus:v1.0.0
docker push geekxflood/nereus:latest
```

## Contributing Guidelines

### Pull Request Process

1. **Fork the repository**
2. **Create a feature branch**
3. **Make your changes**
4. **Add tests and documentation**
5. **Ensure all checks pass**
6. **Submit pull request**

### Code Review Checklist

- [ ] Code follows style guidelines
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] No breaking changes (or properly documented)
- [ ] Security considerations addressed
- [ ] Performance impact considered

### Getting Help

- **GitHub Issues**: Report bugs and request features
- **GitHub Discussions**: Ask questions and discuss ideas
- **Documentation**: Check existing documentation first
- **Code Examples**: Review examples in the repository

## Development Tools

### Recommended IDE Setup

**VS Code Extensions**:
- Go extension
- golangci-lint extension
- GitLens
- Docker extension

**IntelliJ/GoLand**:
- Built-in Go support
- Database tools
- Docker integration

### Useful Commands

```bash
# Development workflow
make test          # Run all tests
make lint          # Run linters
make build         # Build binary
make docker        # Build Docker image
make clean         # Clean build artifacts

# Code generation
go generate ./...  # Run code generators

# Dependency management
go mod tidy        # Clean up dependencies
go mod vendor      # Vendor dependencies
```

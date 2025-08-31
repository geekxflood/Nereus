# Contributing to Nereus

Thank you for your interest in contributing to Nereus! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

## Getting Started

### Prerequisites

- **Go**: Version 1.25 or later
- **Git**: For version control
- **Docker**: For testing containerized deployments (optional)
- **Make**: For build automation (optional)

### Development Environment

1. **Fork and Clone**:
   ```bash
   # Fork the repository on GitHub
   git clone https://github.com/yourusername/nereus.git
   cd nereus
   git remote add upstream https://github.com/geekxflood/nereus.git
   ```

2. **Install Dependencies**:
   ```bash
   go mod download
   go mod verify
   ```

3. **Install Development Tools**:
   ```bash
   # Linting and security tools
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
   go install golang.org/x/tools/cmd/goimports@latest
   ```

4. **Verify Setup**:
   ```bash
   go test ./...
   golangci-lint run
   go build -o nereus ./main.go
   ```

## How to Contribute

### Reporting Issues

Before creating an issue, please:

1. **Search existing issues** to avoid duplicates
2. **Use the issue templates** provided
3. **Provide detailed information** including:
   - Operating system and version
   - Go version
   - Nereus version
   - Configuration details (sanitized)
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs or error messages

### Suggesting Features

For feature requests:

1. **Check existing feature requests** in issues and discussions
2. **Use the feature request template**
3. **Provide clear use cases** and benefits
4. **Consider implementation complexity** and maintenance burden
5. **Be open to discussion** and alternative approaches

### Contributing Code

#### 1. Choose an Issue

- Look for issues labeled `good first issue` for beginners
- Check issues labeled `help wanted` for areas needing assistance
- Comment on issues you'd like to work on to avoid duplication

#### 2. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-number-description
```

#### 3. Make Changes

Follow our [development guidelines](README.md#development-workflow):

- **Write tests** for new functionality
- **Update documentation** as needed
- **Follow code style** guidelines
- **Add appropriate logging** with structured context
- **Handle errors** properly with context

#### 4. Test Your Changes

```bash
# Run all tests
go test ./...

# Run linters
golangci-lint run

# Security scan
gosec ./...

# Test build
go build -o nereus ./main.go

# Integration test (if applicable)
./nereus validate --config examples/config.yaml
```

#### 5. Commit Changes

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```bash
git add .
git commit -m "feat(listener): add support for SNMPv3 authentication"
```

**Commit Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or modifying tests
- `chore`: Build process or auxiliary tool changes

#### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Create a pull request on GitHub with:
- **Clear title** describing the change
- **Detailed description** of what was changed and why
- **Reference to related issues** (e.g., "Fixes #123")
- **Testing information** describing how changes were tested

## Development Guidelines

### Code Style

#### Go Code Standards

1. **Follow Go conventions**:
   - Use `gofmt` for formatting
   - Use `goimports` for import organization
   - Follow standard naming conventions

2. **Package Organization**:
   ```go
   // Package documentation
   // Package mib provides MIB loading and SNMP parsing functionality.
   package mib
   
   // Imports (standard library first, then external, then internal)
   import (
       "context"
       "fmt"
       
       "github.com/external/package"
       
       "github.com/geekxflood/nereus/internal/types"
   )
   
   // Constants and variables
   const DefaultTimeout = 30 * time.Second
   
   // Interfaces
   type Manager interface {
       LoadMIBs(ctx context.Context) error
   }
   
   // Types
   type Config struct {
       Directories []string
   }
   
   // Implementation
   ```

3. **Error Handling**:
   ```go
   // Good: Wrap errors with context
   if err != nil {
       return fmt.Errorf("failed to load MIB file %s: %w", filename, err)
   }
   
   // Bad: Lose error context
   if err != nil {
       return err
   }
   ```

4. **Logging**:
   ```go
   // Good: Structured logging with context
   logger.Info("processing SNMP trap",
       "source_ip", sourceIP,
       "trap_oid", trapOID,
       "component", "listener",
       "duration", processingTime)
   
   // Bad: Unstructured logging
   log.Printf("Processing trap from %s", sourceIP)
   ```

#### Documentation Standards

1. **Package Documentation**:
   ```go
   // Package listener provides SNMP trap listening and validation functionality.
   // It handles UDP socket management, packet validation, and concurrent processing
   // of incoming SNMP traps with configurable worker pools.
   package listener
   ```

2. **Function Documentation**:
   ```go
   // ProcessTrap processes an incoming SNMP trap packet and converts it to an internal event.
   // It validates the packet format, extracts varbinds, and enriches the event with
   // metadata before passing it to the correlation engine.
   //
   // Returns an error if the packet is malformed or processing fails.
   func (l *Listener) ProcessTrap(ctx context.Context, packet []byte) error {
       // Implementation
   }
   ```

3. **Type Documentation**:
   ```go
   // Config contains configuration options for the SNMP listener.
   type Config struct {
       // Host is the IP address to bind to (default: "0.0.0.0")
       Host string `yaml:"host"`
       
       // Port is the UDP port to listen on (default: 162)
       Port int `yaml:"port"`
       
       // BufferSize is the UDP receive buffer size in bytes (default: 65536)
       BufferSize int `yaml:"buffer_size"`
   }
   ```

### Testing Guidelines

#### Unit Tests

1. **Test Structure**:
   ```go
   func TestManager_LoadMIBs(t *testing.T) {
       tests := []struct {
           name        string
           directories []string
           wantErr     bool
           wantCount   int
       }{
           {
               name:        "valid MIB directory",
               directories: []string{"testdata/mibs"},
               wantErr:     false,
               wantCount:   3,
           },
           {
               name:        "nonexistent directory",
               directories: []string{"nonexistent"},
               wantErr:     true,
               wantCount:   0,
           },
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               m := NewManager(Config{Directories: tt.directories})
               err := m.LoadMIBs(context.Background())
               
               if (err != nil) != tt.wantErr {
                   t.Errorf("LoadMIBs() error = %v, wantErr %v", err, tt.wantErr)
               }
               
               if got := m.GetLoadedCount(); got != tt.wantCount {
                   t.Errorf("LoadMIBs() loaded count = %v, want %v", got, tt.wantCount)
               }
           })
       }
   }
   ```

2. **Test Helpers**:
   ```go
   // setupTestManager creates a test manager with test data
   func setupTestManager(t *testing.T) *Manager {
       t.Helper()
       
       config := Config{
           Directories: []string{"testdata/mibs"},
           CacheSize:   100,
       }
       
       manager := NewManager(config)
       if err := manager.LoadMIBs(context.Background()); err != nil {
           t.Fatalf("failed to setup test manager: %v", err)
       }
       
       return manager
   }
   ```

3. **Integration Tests**:
   ```go
   func TestEventProcessingPipeline(t *testing.T) {
       if testing.Short() {
           t.Skip("skipping integration test in short mode")
       }
       
       // Setup test environment
       ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
       defer cancel()
       
       // Test full pipeline
       // ... implementation
   }
   ```

#### Test Coverage

- **Minimum Coverage**: 80% for all packages
- **Critical Paths**: 100% coverage for error handling and security-related code
- **Integration Tests**: Cover major workflows and component interactions

```bash
# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage by package
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

### Performance Guidelines

#### Optimization Principles

1. **Measure First**: Use profiling before optimizing
2. **Optimize Hot Paths**: Focus on frequently executed code
3. **Memory Efficiency**: Minimize allocations in critical paths

#### Profiling

```go
// Benchmark tests
func BenchmarkOIDResolution(b *testing.B) {
    manager := setupBenchmarkManager()
    oid := "1.3.6.1.2.1.1.1.0"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := manager.ResolveName(oid)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Memory allocation testing
func BenchmarkEventProcessing(b *testing.B) {
    processor := setupBenchmarkProcessor()
    event := createBenchmarkEvent()
    
    b.ReportAllocs()
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        processor.ProcessEvent(context.Background(), event)
    }
}
```

### Security Guidelines

#### Input Validation

```go
// Validate all external inputs
func validateSNMPPacket(packet []byte) error {
    if len(packet) < minPacketSize {
        return fmt.Errorf("packet too small: %d bytes", len(packet))
    }
    
    if len(packet) > maxPacketSize {
        return fmt.Errorf("packet too large: %d bytes", len(packet))
    }
    
    // Additional validation...
    return nil
}
```

#### Error Handling

```go
// Don't expose internal details
func (s *Storage) GetEvent(id string) (*Event, error) {
    event, err := s.db.QueryRow("SELECT * FROM events WHERE id = ?", id).Scan(...)
    if err != nil {
        // Log detailed error internally
        s.logger.Error("database query failed", "error", err, "id", id)
        
        // Return generic error to caller
        return nil, fmt.Errorf("failed to retrieve event")
    }
    
    return event, nil
}
```

## Pull Request Process

### Before Submitting

1. **Ensure all tests pass**:
   ```bash
   go test ./...
   golangci-lint run
   gosec ./...
   ```

2. **Update documentation** if needed
3. **Add changelog entry** for significant changes
4. **Rebase on latest main**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

### Pull Request Template

Use this template for pull requests:

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added and passing
- [ ] No breaking changes (or properly documented)
```

### Review Process

1. **Automated Checks**: CI must pass
2. **Code Review**: At least one maintainer review required
3. **Testing**: Reviewer may request additional testing
4. **Documentation**: Ensure documentation is complete and accurate

## Community

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Pull Requests**: Code contributions and reviews

### Getting Help

1. **Check Documentation**: Review existing documentation first
2. **Search Issues**: Look for similar questions or problems
3. **Ask Questions**: Use GitHub Discussions for questions
4. **Be Patient**: Maintainers are volunteers with limited time

### Recognition

Contributors are recognized in:
- **CONTRIBUTORS.md**: List of all contributors
- **Release Notes**: Acknowledgment of significant contributions
- **GitHub**: Contributor statistics and recognition

## Maintainer Guidelines

### For Maintainers

1. **Be Welcoming**: Help new contributors get started
2. **Provide Feedback**: Give constructive, actionable feedback
3. **Be Responsive**: Respond to issues and PRs in reasonable time
4. **Maintain Quality**: Ensure code quality and consistency
5. **Document Decisions**: Explain reasoning for significant decisions

### Release Process

1. **Version Planning**: Plan releases with community input
2. **Testing**: Thorough testing before releases
3. **Documentation**: Update documentation for releases
4. **Communication**: Announce releases and changes

Thank you for contributing to Nereus! Your contributions help make network monitoring better for everyone.

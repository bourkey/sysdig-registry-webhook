# Developer Guide

This guide covers the architecture, development workflow, and contribution guidelines for the Registry Webhook Scanner project.

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Project Structure](#project-structure)
- [Scanner Backend Architecture](#scanner-backend-architecture)
- [Development Setup](#development-setup)
- [Adding New Scanner Backends](#adding-new-scanner-backends)
- [Testing](#testing)
- [Contributing](#contributing)

---

## Architecture Overview

The Registry Webhook Scanner is built using a modular architecture with pluggable scanner backends:

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP Server Layer                        │
│  - Webhook endpoints (/webhook, /health, /ready)            │
│  - Authentication middleware (HMAC, Bearer)                  │
│  - Registry-specific parsers (Docker Hub, Harbor, GitLab)   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                     Event Queue Layer                        │
│  - In-memory buffered channel                                │
│  - Decouples webhook receipt from scanning                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                     Worker Pool Layer                        │
│  - Configurable number of worker goroutines                  │
│  - Concurrent scan execution                                 │
│  - Scanner backend factory                                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  Scanner Backend Layer                       │
│  ┌──────────────────┐         ┌──────────────────┐          │
│  │  CLI Scanner     │         │ Registry Scanner │          │
│  │  - Local exec    │         │ - API-based      │          │
│  │  - Image pull    │         │ - Async polling  │          │
│  └──────────────────┘         └──────────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

1. **HTTP Server**: Receives webhooks from registries, authenticates, and parses payloads
2. **Event Queue**: Buffers scan requests for asynchronous processing
3. **Worker Pool**: Manages concurrent scan execution
4. **Scanner Backends**: Pluggable implementations for different scanning methods

---

## Project Structure

```
.
├── cmd/
│   └── scanner-webhook/          # Main application entry point
│       └── main.go
├── pkg/
│   ├── config/                   # Configuration management
│   │   ├── types.go              # Configuration structs
│   │   └── loader.go             # YAML loading and validation
│   ├── scanner/                  # Scanner backend implementations
│   │   ├── backend.go            # ScannerBackend interface
│   │   ├── factory.go            # Scanner factory
│   │   ├── cli_scanner.go        # CLI Scanner implementation
│   │   ├── registry_scanner.go   # Registry Scanner implementation
│   │   ├── registry_api.go       # HTTP client with retry logic
│   │   └── errors.go             # Scanner-specific errors
│   ├── webhook/                  # Webhook handling
│   │   ├── parser.go             # Registry-specific parsers
│   │   └── auth.go               # Authentication middleware
│   ├── queue/                    # Event queue implementation
│   │   └── queue.go
│   └── worker/                   # Worker pool implementation
│       └── pool.go
├── internal/
│   └── models/                   # Shared data models
│       ├── scan.go               # Scan request/result models
│       └── webhook.go            # Webhook event models
├── docs/                         # Documentation
│   ├── CONFIGURATION.md
│   ├── TROUBLESHOOTING.md
│   └── DEVELOPER.md (this file)
├── deployments/                  # Kubernetes manifests
│   └── kubernetes/
├── test/                         # Integration tests
│   ├── mocks/
│   └── fixtures/
├── config.example.yaml           # Example configuration
├── go.mod                        # Go module dependencies
└── README.md                     # Project overview
```

---

## Scanner Backend Architecture

The scanner backend system uses the **Strategy Pattern** to allow pluggable scanning implementations.

### ScannerBackend Interface

All scanner backends must implement the `ScannerBackend` interface:

```go
// pkg/scanner/backend.go
type ScannerBackend interface {
    // Scan executes a scan for the given image and returns the result
    Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error)
    
    // Type returns the scanner type identifier (e.g., "cli", "registry")
    Type() string
    
    // ValidateConfig validates that the scanner backend is properly configured
    ValidateConfig() error
}
```

### Scanner Factory

The factory pattern is used to create scanner instances based on configuration:

```go
// pkg/scanner/factory.go
func NewScannerBackend(cfg *config.Config, registryName string, logger *logrus.Logger) (ScannerBackend, error)
```

**Scanner Type Determination Priority:**
1. Per-registry override (`registries[].scanner.type`)
2. Global default (`scanner.type`)
3. Fallback to "cli" for backward compatibility

### Existing Implementations

#### 1. CLI Scanner (`cli_scanner.go`)

**Characteristics:**
- Executes `sysdig-cli-scanner` binary locally
- Downloads container images to local storage
- Synchronous execution
- Self-contained (no external API dependencies)

**Key Methods:**
- `Scan()`: Executes CLI with image reference
- `ValidateConfig()`: Checks CLI binary exists and is executable
- `buildScanArgs()`: Constructs CLI arguments from configuration

**Configuration:**
```yaml
scanner:
  type: cli
  cli_path: /usr/local/bin/sysdig-cli-scanner
  sysdig_token: ${SYSDIG_API_TOKEN}
```

#### 2. Registry Scanner (`registry_scanner.go`)

**Characteristics:**
- Uses Sysdig Registry Scanner API
- No local image download (scans in Sysdig backend)
- Asynchronous flow: initiate → poll → result
- Requires network connectivity to Sysdig API

**Key Methods:**
- `Scan()`: Orchestrates async scan flow
- `initiateScan()`: POST to API to start scan
- `pollScanStatus()`: Polls GET endpoint until complete
- `getScanResult()`: Fetches scan result
- `ValidateConfig()`: Validates API URL and project ID

**Configuration:**
```yaml
scanner:
  type: registry
  sysdig_token: ${SYSDIG_API_TOKEN}
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s
```

---

## Adding New Scanner Backends

Follow these steps to add a new scanner backend (e.g., "api" scanner):

### Step 1: Define Scanner Type

Add the new type to configuration constants:

```go
// pkg/config/types.go
type ScannerType string

const (
    ScannerTypeCLI      ScannerType = "cli"
    ScannerTypeRegistry ScannerType = "registry"
    ScannerTypeAPI      ScannerType = "api"  // New type
)
```

### Step 2: Add Configuration Struct

Define backend-specific configuration:

```go
// pkg/config/types.go
type APIScannerConfig struct {
    APIURL     string `yaml:"api_url"`
    APIKey     string `yaml:"api_key"`
    VerifyTLS  bool   `yaml:"verify_tls"`
    // Add other fields as needed
}

// Add to ScannerConfig
type ScannerConfig struct {
    Type          ScannerType            `yaml:"type"`
    // ... existing fields ...
    APIScanner    *APIScannerConfig      `yaml:"api_scanner,omitempty"`  // New field
}
```

### Step 3: Add Configuration Defaults and Validation

```go
// pkg/config/loader.go

// In setDefaults():
if c.Scanner.Type == ScannerTypeAPI && c.Scanner.APIScanner != nil {
    if c.Scanner.APIScanner.APIURL == "" {
        c.Scanner.APIScanner.APIURL = "https://api.example.com"
    }
}

// In validateConfig():
if c.Scanner.Type == ScannerTypeAPI {
    if c.Scanner.APIScanner == nil {
        return fmt.Errorf("scanner.api_scanner configuration is required when scanner.type is 'api'")
    }
    if c.Scanner.APIScanner.APIKey == "" {
        return fmt.Errorf("scanner.api_scanner.api_key is required")
    }
}
```

### Step 4: Implement ScannerBackend Interface

Create a new scanner implementation:

```go
// pkg/scanner/api_scanner.go
package scanner

import (
    "context"
    "github.com/sirupsen/logrus"
    "github.com/sysdig/registry-webhook-scanner/internal/models"
    "github.com/sysdig/registry-webhook-scanner/pkg/config"
)

type APIScanner struct {
    config     *config.Config
    logger     *logrus.Logger
    httpClient *http.Client
}

func NewAPIScanner(cfg *config.Config, logger *logrus.Logger) *APIScanner {
    return &APIScanner{
        config: cfg,
        logger: logger,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (s *APIScanner) Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error) {
    s.logger.WithFields(logrus.Fields{
        "image_ref":    req.ImageRef,
        "request_id":   req.RequestID,
        "scanner_type": "api",
    }).Info("Starting API Scanner scan")
    
    // Implement your scanning logic here
    // Return *models.ScanResult with appropriate status
    
    return &models.ScanResult{
        ImageRef:  req.ImageRef,
        RequestID: req.RequestID,
        Status:    models.ScanStatusSuccess,
        // ... other fields
    }, nil
}

func (s *APIScanner) Type() string {
    return "api"
}

func (s *APIScanner) ValidateConfig() error {
    if s.config.Scanner.APIScanner == nil {
        return fmt.Errorf("API scanner configuration is missing")
    }
    
    if s.config.Scanner.APIScanner.APIURL == "" {
        return fmt.Errorf("API scanner API URL is required")
    }
    
    if s.config.Scanner.APIScanner.APIKey == "" {
        return fmt.Errorf("API scanner API key is required")
    }
    
    return nil
}
```

### Step 5: Register in Factory

Add the new scanner to the factory:

```go
// pkg/scanner/factory.go

func NewScannerBackend(cfg *config.Config, registryName string, logger *logrus.Logger) (ScannerBackend, error) {
    scannerType := determineScannerType(cfg, registryName)
    
    var backend ScannerBackend
    
    switch scannerType {
    case config.ScannerTypeCLI:
        backend = NewCLIScanner(cfg, logger)
        
    case config.ScannerTypeRegistry:
        backend = NewRegistryScanner(cfg, logger)
        
    case config.ScannerTypeAPI:  // New case
        backend = NewAPIScanner(cfg, logger)
        
    default:
        return nil, fmt.Errorf("unsupported scanner type: %s", scannerType)
    }
    
    // Validation happens here
    if err := backend.ValidateConfig(); err != nil {
        return nil, fmt.Errorf("scanner validation failed for type %s: %w", scannerType, err)
    }
    
    return backend, nil
}
```

### Step 6: Add Tests

Create comprehensive unit tests:

```go
// pkg/scanner/api_scanner_test.go
package scanner

import (
    "context"
    "testing"
    
    "github.com/sirupsen/logrus"
    "github.com/sysdig/registry-webhook-scanner/internal/models"
    "github.com/sysdig/registry-webhook-scanner/pkg/config"
)

func TestAPIScanner_Scan(t *testing.T) {
    cfg := &config.Config{
        Scanner: config.ScannerConfig{
            Type: config.ScannerTypeAPI,
            APIScanner: &config.APIScannerConfig{
                APIURL: "https://api.example.com",
                APIKey: "test-key",
            },
        },
    }
    
    scanner := NewAPIScanner(cfg, logrus.New())
    
    req := &models.ScanRequest{
        ImageRef:  "myregistry/myimage:v1.0.0",
        RequestID: "test-123",
    }
    
    result, err := scanner.Scan(context.Background(), req)
    
    if err != nil {
        t.Fatalf("Scan failed: %v", err)
    }
    
    if result.Status != models.ScanStatusSuccess {
        t.Errorf("Expected success status, got %s", result.Status)
    }
}

func TestAPIScanner_ValidateConfig(t *testing.T) {
    tests := []struct {
        name    string
        cfg     *config.Config
        wantErr bool
    }{
        {
            name: "valid configuration",
            cfg: &config.Config{
                Scanner: config.ScannerConfig{
                    APIScanner: &config.APIScannerConfig{
                        APIURL: "https://api.example.com",
                        APIKey: "test-key",
                    },
                },
            },
            wantErr: false,
        },
        {
            name: "missing API key",
            cfg: &config.Config{
                Scanner: config.ScannerConfig{
                    APIScanner: &config.APIScannerConfig{
                        APIURL: "https://api.example.com",
                    },
                },
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            scanner := NewAPIScanner(tt.cfg, logrus.New())
            err := scanner.ValidateConfig()
            
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Step 7: Update Documentation

Update the following documentation files:

1. **docs/CONFIGURATION.md**: Add API scanner configuration section
2. **README.md**: Add API scanner to feature list
3. **docs/TROUBLESHOOTING.md**: Add API scanner troubleshooting
4. **config.example.yaml**: Add commented API scanner example

### Step 8: Update config.example.yaml

```yaml
scanner:
  type: cli  # Options: "cli", "registry", "api"
  
  # API Scanner configuration (when type is "api")
  # api_scanner:
  #   api_url: https://api.example.com
  #   api_key: ${API_SCANNER_KEY}
  #   verify_tls: true
```

---

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker (for local testing)
- Kubernetes cluster (optional, for deployment testing)

### Clone and Build

```bash
# Clone repository
git clone https://github.com/sysdig/registry-webhook-scanner.git
cd registry-webhook-scanner

# Download dependencies
go mod download

# Build
go build -o bin/scanner-webhook cmd/scanner-webhook/main.go

# Run tests
go test ./...
```

### Local Development

```bash
# Create local configuration
cp config.example.yaml config.yaml
# Edit config.yaml with your values

# Run locally
export CONFIG_FILE=config.yaml
export SYSDIG_API_TOKEN=your-token
go run cmd/scanner-webhook/main.go
```

### Testing with ngrok

For webhook testing with external registries:

```bash
# Start ngrok
ngrok http 8080

# Use the ngrok URL in your registry webhook configuration
# Example: https://abc123.ngrok.io/webhook
```

---

## Testing

### Unit Tests

```bash
# Run all unit tests
go test ./pkg/...

# Run with coverage
go test -cover ./pkg/...

# Run specific package
go test ./pkg/scanner/...
```

### Integration Tests

```bash
# Run integration tests (requires mock services)
go test ./test/integration/...
```

### End-to-End Testing

1. Deploy to test Kubernetes cluster
2. Configure test registry webhook
3. Push test image
4. Verify scan execution in logs

---

## Contributing

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` for formatting
- Run `golangci-lint` before committing

### Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Ensure all tests pass: `go test ./...`
5. Update documentation if needed
6. Commit with descriptive messages
7. Push and create pull request

### Logging Guidelines

Always include structured fields in logs:

```go
logger.WithFields(logrus.Fields{
    "image_ref":    req.ImageRef,
    "request_id":   req.RequestID,
    "scanner_type": scanner.Type(),  // Important for debugging
}).Info("Scan completed successfully")
```

Required fields:
- `scanner_type`: Always include to identify which backend was used
- `image_ref`: For scan-related logs
- `request_id`: For tracking requests through the system
- `registry`: For registry-specific operations

---

## Architecture Decisions

### Why Pluggable Scanner Backends?

**Problem:** Different scanning approaches have trade-offs:
- CLI Scanner: Self-contained but requires local storage
- Registry Scanner: API-based but depends on Sysdig API availability

**Solution:** Abstraction layer (ScannerBackend interface) allows:
- Easy switching between scanner types
- Per-registry scanner configuration
- Adding new scanning methods without changing core code

### Why Worker Pool?

**Problem:** Webhook receipt must be fast (< 5s timeout), but scanning is slow (30s-5min)

**Solution:** Decouple webhook receipt from scanning:
1. Webhook handler quickly queues requests (< 100ms)
2. Worker pool processes scans concurrently
3. Configurable concurrency limits prevent resource exhaustion

### Why In-Memory Queue?

**Decision:** Use in-memory buffered channel instead of external queue (Redis, RabbitMQ)

**Rationale:**
- Simpler deployment (no external dependencies)
- Lower latency (no network round-trips)
- Sufficient for webhook use case (ephemeral events)

**Trade-off:** Events lost on restart (acceptable for this use case)

---

## Future Enhancements

Potential areas for contribution:

1. **Scanner Backends:**
   - Trivy scanner integration
   - Clair scanner support
   - Custom scanner implementations

2. **Queue Backends:**
   - Redis-backed persistent queue option
   - RabbitMQ/Kafka for high-volume scenarios

3. **Observability:**
   - Prometheus metrics export
   - OpenTelemetry tracing
   - Scan result webhooks

4. **Registry Support:**
   - Quay.io webhook parser
   - Azure Container Registry
   - JFrog Artifactory

5. **Authentication:**
   - mTLS support
   - OAuth2 token validation
   - IP allowlisting

---

## Resources

- [Sysdig CLI Scanner Documentation](https://docs.sysdig.com/en/docs/sysdig-secure/scanning/)
- [Go Best Practices](https://golang.org/doc/effective_go.html)
- [Container Registry Webhook Specifications](https://docs.docker.com/docker-hub/webhooks/)

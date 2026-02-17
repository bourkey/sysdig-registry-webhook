# Developer Guide

Guide for developers contributing to the Registry Webhook Scanner, including how to add support for new container registries.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Adding New Registry Support](#adding-new-registry-support)
- [Testing](#testing)
- [Code Style](#code-style)
- [Contributing Guidelines](#contributing-guidelines)

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker (for building images)
- kubectl (for Kubernetes testing)
- make (optional, for Makefile targets)

### Clone and Build

```bash
# Clone repository
git clone https://github.com/yourusername/registry-webhook-scanner.git
cd registry-webhook-scanner

# Install dependencies
go mod download

# Build binary
go build -o webhook-server cmd/webhook-server/main.go

# Run tests
go test ./...
```

### Run Locally

```bash
# Create test configuration
cp config.example.yaml config.yaml

# Edit config.yaml with your settings
export CONFIG_FILE=config.yaml
export SYSDIG_API_TOKEN=your-sysdig-token
export LOG_LEVEL=debug

# Run server
./webhook-server
```

### Development Tools

**Recommended:**
- **golangci-lint**: Linting
- **gofmt**: Code formatting
- **govulncheck**: Vulnerability scanning
- **gopls**: Language server for IDE integration

```bash
# Install tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run linter
golangci-lint run ./...

# Format code
gofmt -s -w .

# Check for vulnerabilities
govulncheck ./...
```

## Project Structure

```
registry-webhook-scanner/
├── cmd/
│   └── webhook-server/
│       └── main.go                 # Application entry point
├── pkg/
│   ├── auth/                       # Authentication (HMAC, bearer)
│   │   ├── hmac.go
│   │   ├── bearer.go
│   │   └── middleware.go
│   ├── config/                     # Configuration management
│   │   ├── types.go
│   │   ├── loader.go
│   │   ├── env.go
│   │   └── secrets.go
│   ├── logging/                    # Structured logging
│   │   └── logger.go
│   ├── queue/                      # Event queue and workers
│   │   ├── queue.go
│   │   ├── worker.go
│   │   ├── dedup.go
│   │   └── retry.go
│   ├── scanner/                    # Sysdig CLI integration
│   │   ├── scanner.go
│   │   ├── credentials.go
│   │   └── results.go
│   ├── shutdown/                   # Graceful shutdown
│   │   └── shutdown.go
│   └── webhook/                    # Webhook handling
│       ├── server.go
│       └── parsers/                # Registry-specific parsers
│           ├── dockerhub.go
│           ├── harbor.go
│           ├── gitlab.go
│           ├── registry.go         # Parser registry
│           ├── normalize.go
│           └── utils.go
├── internal/
│   └── models/                     # Data models
│       ├── scan.go
│       └── webhook.go
├── test/
│   ├── fixtures/                   # Mock webhook payloads
│   ├── mocks/                      # Mock implementations
│   └── integration/                # Integration tests
├── deployments/
│   └── kubernetes/                 # Kubernetes manifests
├── docs/                           # Documentation
└── openspec/                       # Design artifacts
```

## Adding New Registry Support

To add support for a new container registry, follow these steps:

### 1. Understand the Registry's Webhook Format

First, study the registry's webhook documentation:

- **Webhook payload structure**: What JSON fields are sent?
- **Event types**: Does it support push events? What's the event type field?
- **Image information**: Where are image name, tag, digest, registry URL?
- **Authentication**: HMAC signatures, bearer tokens, custom headers?

**Example registries:**
- Docker Hub: https://docs.docker.com/docker-hub/webhooks/
- Harbor: https://goharbor.io/docs/latest/working-with-projects/project-configuration/configure-webhooks/
- GitLab: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html

### 2. Create Parser Implementation

Create a new parser file in `pkg/webhook/parsers/`:

**File: `pkg/webhook/parsers/myregistry.go`**

```go
package parsers

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// MyRegistryParser implements WebhookParser for MyRegistry
type MyRegistryParser struct{}

// Parse parses MyRegistry webhook payload
func (p *MyRegistryParser) Parse(body io.Reader, registryURL string) ([]models.WebhookEvent, error) {
	// Define payload structure
	var payload struct {
		EventType string `json:"event_type"`
		Image     struct {
			Repository string `json:"repository"`
			Tag        string `json:"tag"`
			Digest     string `json:"digest"`
		} `json:"image"`
		Timestamp int64 `json:"timestamp"`
	}

	// Decode JSON
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode MyRegistry webhook: %w", err)
	}

	// Validate event type (only process push events)
	if payload.EventType != "image.push" {
		return nil, fmt.Errorf("unsupported event type: %s", payload.EventType)
	}

	// Extract image information
	imageRef := NormalizeImageReference(
		registryURL,               // Registry URL
		payload.Image.Repository,  // Repository path
		payload.Image.Tag,         // Tag
		payload.Image.Digest,      // Digest
	)

	// Create webhook event
	event := models.WebhookEvent{
		ImageRef:   imageRef,
		Repository: payload.Image.Repository,
		Tag:        payload.Image.Tag,
		Digest:     payload.Image.Digest,
		EventType:  payload.EventType,
		Timestamp:  payload.Timestamp,
		RequestID:  GenerateRequestID(),
	}

	// Return as slice (some registries send multiple images in one webhook)
	return []models.WebhookEvent{event}, nil
}

// Type returns the registry type identifier
func (p *MyRegistryParser) Type() string {
	return "myregistry"
}
```

### 3. Handle Multiple Images (if applicable)

Some registries send multiple tags/images in a single webhook:

```go
func (p *MyRegistryParser) Parse(body io.Reader, registryURL string) ([]models.WebhookEvent, error) {
	var payload struct {
		Images []struct {
			Repository string `json:"repository"`
			Tag        string `json:"tag"`
		} `json:"images"`
	}

	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, err
	}

	// Process multiple images
	events := make([]models.WebhookEvent, 0, len(payload.Images))
	for _, img := range payload.Images {
		imageRef := NormalizeImageReference(registryURL, img.Repository, img.Tag, "")
		events = append(events, models.WebhookEvent{
			ImageRef:   imageRef,
			Repository: img.Repository,
			Tag:        img.Tag,
			RequestID:  GenerateRequestID(),
		})
	}

	return events, nil
}
```

### 4. Register the Parser

Add your parser to the registry in `pkg/webhook/parsers/registry.go`:

```go
// RegisterDefaultParsers registers all built-in parsers
func RegisterDefaultParsers() {
	Register("dockerhub", &DockerHubParser{})
	Register("harbor", &HarborParser{})
	Register("gitlab", &GitLabParser{})
	Register("myregistry", &MyRegistryParser{})  // Add this line
}
```

### 5. Add Configuration Support

Update configuration types if needed. Most registries use the existing config structure:

```yaml
registries:
  - name: myregistry-prod
    type: myregistry              # Your registry type
    url: https://registry.example.com
    auth:
      type: bearer                 # or hmac
      token: ${MYREGISTRY_TOKEN}
```

**If custom configuration is needed**, update `pkg/config/types.go`:

```go
type RegistryConfig struct {
	Name     string               `yaml:"name"`
	Type     string               `yaml:"type"`
	URL      string               `yaml:"url,omitempty"`
	Auth     AuthConfig           `yaml:"auth"`
	Scanner  ScannerOverride      `yaml:"scanner,omitempty"`
	// Add custom fields if needed
	MyRegistrySpecific string    `yaml:"myregistry_specific,omitempty"`
}
```

### 6. Write Tests

Create test file: `pkg/webhook/parsers/myregistry_test.go`

```go
package parsers

import (
	"strings"
	"testing"
)

func TestMyRegistryParser_Parse(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		registryURL string
		wantImageRef string
		wantErr     bool
	}{
		{
			name: "successful parse",
			payload: `{
				"event_type": "image.push",
				"image": {
					"repository": "myapp",
					"tag": "v1.0.0",
					"digest": "sha256:abc123"
				}
			}`,
			registryURL: "registry.example.com",
			wantImageRef: "registry.example.com/myapp:v1.0.0@sha256:abc123",
			wantErr: false,
		},
		{
			name: "unsupported event type",
			payload: `{
				"event_type": "image.delete",
				"image": {"repository": "myapp", "tag": "v1.0.0"}
			}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			payload: `{invalid json`,
			wantErr: true,
		},
	}

	parser := &MyRegistryParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := parser.Parse(strings.NewReader(tt.payload), tt.registryURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(events) != 1 {
					t.Errorf("Parse() returned %d events, want 1", len(events))
					return
				}

				if events[0].ImageRef != tt.wantImageRef {
					t.Errorf("Parse() imageRef = %v, want %v", events[0].ImageRef, tt.wantImageRef)
				}
			}
		})
	}
}

func TestMyRegistryParser_Type(t *testing.T) {
	parser := &MyRegistryParser{}
	if got := parser.Type(); got != "myregistry" {
		t.Errorf("Type() = %v, want %v", got, "myregistry")
	}
}
```

### 7. Create Test Fixtures

Add sample webhook payload in `test/fixtures/myregistry-webhook.json`:

```json
{
  "event_type": "image.push",
  "image": {
    "repository": "myproject/myapp",
    "tag": "v1.0.0",
    "digest": "sha256:abc123def456"
  },
  "pusher": "testuser",
  "timestamp": 1620000000
}
```

### 8. Add Documentation

Update registry setup documentation in `docs/REGISTRY_SETUP.md`:

```markdown
## MyRegistry

MyRegistry supports webhooks for push events.

### Prerequisites
- MyRegistry account with admin access
- Webhook endpoint accessible from MyRegistry

### Setup Steps

1. **Navigate to Webhooks Settings**
   - Log in to MyRegistry dashboard
   - Go to Settings → Webhooks

2. **Create Webhook**
   - Click "Add Webhook"
   - **URL**: `https://your-webhook-scanner.example.com/webhook`
   - **Events**: Select "Image Push"
   - **Token**: Generate and save a bearer token

3. **Test Webhook**
   ```bash
   docker push registry.example.com/myapp:test
   ```

### Scanner Configuration

```yaml
registries:
  - name: myregistry-prod
    type: myregistry
    url: https://registry.example.com
    auth:
      type: bearer
      token: ${MYREGISTRY_WEBHOOK_TOKEN}
    scanner:
      timeout: 300s
      credentials:
        username: ${MYREGISTRY_USERNAME}
        password: ${MYREGISTRY_PASSWORD}
```

### Webhook Payload Example

```json
{
  "event_type": "image.push",
  "image": {
    "repository": "myapp",
    "tag": "v1.0.0"
  }
}
```
```

### 9. Update README

Add your registry to the supported registries table in `README.md`:

```markdown
## Supported Registries

| Registry | Type Value | Authentication |
|----------|------------|----------------|
| Docker Hub | `dockerhub` | HMAC signature or none |
| Harbor | `harbor` | Bearer token |
| GitLab Registry | `gitlab` | Token |
| MyRegistry | `myregistry` | Bearer token |
```

### 10. Test End-to-End

1. **Build and run locally:**
   ```bash
   go build -o webhook-server cmd/webhook-server/main.go
   ./webhook-server
   ```

2. **Send test webhook:**
   ```bash
   curl -X POST http://localhost:8080/webhook \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer test-token" \
     -d @test/fixtures/myregistry-webhook.json
   ```

3. **Verify logs:**
   ```bash
   # Should see:
   # - "webhook received" from myregistry
   # - "image extracted" with correct image reference
   # - "scan request enqueued"
   # - "scan started" and "scan completed"
   ```

### 11. Submit Pull Request

1. Create feature branch: `git checkout -b feature/add-myregistry-support`
2. Commit changes with descriptive messages
3. Push branch and open pull request
4. Include in PR description:
   - Registry name and documentation link
   - Authentication method
   - Test results
   - Example webhook payload

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

```bash
# Requires integration tag
go test -tags=integration ./test/integration/...

# With verbose output
go test -tags=integration -v ./test/integration/...
```

### Test Structure

**Unit test naming:**
```go
func TestFunctionName(t *testing.T)                    // Basic test
func TestFunctionName_Scenario(t *testing.T)           // Specific scenario
func TestStructName_MethodName(t *testing.T)           // Method test
```

**Table-driven tests:**
```go
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid input", input: "test", want: "result", wantErr: false},
		{name: "invalid input", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

### Mocking

Use interfaces for testability:

```go
// Production code uses interface
type Scanner interface {
	Scan(imageRef string) (*ScanResult, error)
}

// Test uses mock implementation
type MockScanner struct {
	ScanFunc func(string) (*ScanResult, error)
}

func (m *MockScanner) Scan(imageRef string) (*ScanResult, error) {
	return m.ScanFunc(imageRef)
}
```

## Code Style

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing

### Naming Conventions

- **Packages**: Lowercase, no underscores (e.g., `webhook`, `scanner`)
- **Exported names**: CamelCase (e.g., `ParseWebhook`)
- **Unexported names**: camelCase (e.g., `parsePayload`)
- **Constants**: CamelCase or SCREAMING_SNAKE_CASE for enums
- **Interfaces**: Use `-er` suffix (e.g., `Parser`, `Scanner`)

### Error Handling

```go
// Wrap errors with context
if err != nil {
	return fmt.Errorf("failed to parse webhook: %w", err)
}

// Check specific error types
if errors.Is(err, ErrNotFound) {
	// Handle not found
}

// Custom error types
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}
```

### Logging

```go
// Use structured logging
log.WithFields(logrus.Fields{
	"registry": "harbor",
	"image":    imageRef,
}).Info("webhook received")

// Log levels
log.Debug("detailed debugging info")
log.Info("normal operation")
log.Warn("non-fatal issue")
log.Error("error requiring attention")
```

### Comments

```go
// Package comment (package-level doc)
// Package webhook handles webhook receipt and processing.
package webhook

// Exported function/type gets doc comment
// Parse parses the webhook payload and extracts image information.
// It returns an error if the payload format is invalid.
func Parse(body io.Reader) (*Event, error) {
	// Internal comments explain why, not what
	// Use lowercase, no period at end
	return nil, nil
}
```

## Contributing Guidelines

### Before Submitting

1. **Run tests**: `go test ./...`
2. **Run linter**: `golangci-lint run ./...`
3. **Format code**: `gofmt -s -w .`
4. **Update docs**: Add/update relevant documentation
5. **Add tests**: Ensure new code has test coverage

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding tests
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `chore`: Maintenance tasks

**Examples:**
```
feat(parser): add support for AWS ECR webhooks

Implements ECR EventBridge webhook parser with JSON decoding
and image reference extraction.

Closes #42

fix(auth): handle missing Authorization header gracefully

Previously crashed with nil pointer when header was missing.
Now returns proper 401 Unauthorized error.

docs: update REGISTRY_SETUP.md with ACR configuration

test(queue): add deduplication cache tests
```

### Pull Request Template

When opening a PR, include:

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Tests pass locally
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
```

### Code Review

Expect reviewers to check:
- **Correctness**: Does it work as intended?
- **Tests**: Is there adequate test coverage?
- **Style**: Does it follow Go conventions?
- **Documentation**: Are changes documented?
- **Performance**: Any potential bottlenecks?
- **Security**: Any security implications?

## Advanced Topics

### Adding Authentication Methods

To add a new authentication method (e.g., API key):

1. Create `pkg/auth/apikey.go`
2. Implement verification function
3. Update `pkg/auth/middleware.go` to route to new method
4. Add config type in `pkg/config/types.go`

### Custom Queue Backends

To add persistent queue (Redis, database):

1. Define interface in `pkg/queue/queue.go`
2. Implement interface with new backend
3. Add configuration options
4. Update initialization in `cmd/webhook-server/main.go`

### Metrics and Observability

To add Prometheus metrics:

1. Import `github.com/prometheus/client_golang`
2. Define metrics in `pkg/metrics/metrics.go`
3. Instrument code with counter/histogram/gauge
4. Add `/metrics` endpoint in server

## Resources

- **Go Documentation**: https://golang.org/doc/
- **Project Issues**: https://github.com/yourusername/registry-webhook-scanner/issues
- **Sysdig CLI Docs**: https://docs.sysdig.com/en/docs/sysdig-secure/scanning/
- **Webhook Specs**: See registry-specific documentation

## Questions?

Open an issue on GitHub or reach out to maintainers. We're happy to help!

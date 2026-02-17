# CLAUDE.md

This file provides guidance to Claude Code when working with the Registry Webhook Scanner codebase.

<base-instructions>
    <rule id="1" name="No Hardcoding">Use configuration files or environment variables. Enums are allowed.</rule>
    <rule id="2" name="Fix Root Causes">No workarounds; address issues properly.</rule>
    <rule id="3" name="Remove Legacy Code">Delete old code when replacing it; no shims or compatibility layers.</rule>
    <rule id="4" name="Minimal Comments">Code must be self-explanatory; use comments only when necessary.</rule>
    <rule id="5" name="Avoid Circular Dependencies">Keep modules loosely coupled.</rule>
    <rule id="6" name="Use Logging">Always use logging facilities; no echo statements in scripts. Always write to the logs folder inside the project, and datestamp and timestamp them.</rule>
    <rule id="7" name="Mandatory Testing">Ensure comprehensive testing coverage, including edge cases. Use real API validation.</rule>
    <rule id="8" name="No Unused Resources">Remove any modules, configurations, or dependencies that are not in use.</rule>
    <rule id="9" name="Follow Documentation">Verify implementation against documentation.</rule>
    <rule id="10" name="Version Consistency">Always update the version when making significant changes. Ensure version consistency across all documentation.</rule>
</base-instructions>

## Project Overview

Registry Webhook Scanner is a standalone Go service that automatically triggers Sysdig CLI Scanner when container images are pushed to registries. It bridges the gap between container registry webhooks and security scanning by receiving webhook events, parsing them, and invoking the Sysdig CLI to scan images for vulnerabilities.

**Tech Stack:**
- Language: Go
- Architecture: HTTP service with internal event queue
- Deployment: Docker container with embedded Sysdig CLI binary
- Configuration: Environment variables + YAML config files
- Target Environment: Kubernetes (primary), Docker, or any container platform

## Project Structure

```
.
├── openspec/                      # OpenSpec change management
│   ├── config.yaml               # OpenSpec configuration
│   └── changes/
│       └── registry-webhook-scanner/  # Current change artifacts
│           ├── proposal.md           # Why and what changes
│           ├── design.md             # Architecture and decisions
│           └── specs/                # Capability specifications
│               ├── webhook-receiver/      # Webhook ingestion specs
│               └── scanner-integration/   # Sysdig CLI integration specs
├── cmd/                           # Main application entry points (to be created)
├── internal/                      # Private application code (to be created)
│   ├── webhook/                  # Webhook receiver implementation
│   ├── scanner/                  # Scanner integration
│   ├── queue/                    # Event queue management
│   └── config/                   # Configuration management
├── pkg/                          # Public libraries (to be created)
├── deployments/                  # Kubernetes manifests (to be created)
├── Dockerfile                    # Container build (to be created)
├── go.mod                        # Go module definition (to be created)
└── README.md                     # User-facing documentation
```

## Development Guidelines

### Code Organization

- **`cmd/`**: Main application entry points. Use `cmd/webhook-server/` for the HTTP service
- **`internal/`**: Private application code that shouldn't be imported by external projects
  - `webhook/`: HTTP handlers, authentication, payload parsing
  - `scanner/`: Sysdig CLI invocation, process management, result handling
  - `queue/`: Internal event queue (Go channels + worker pool)
  - `config/`: Configuration loading and validation
- **`pkg/`**: Reusable public libraries (if needed)
- **`deployments/`**: Kubernetes manifests, Helm charts, docker-compose files

### Coding Standards

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (enforced in CI)
- Use `golangci-lint` for linting
- Write table-driven tests for webhook parsers (multiple registry formats)
- Use structured logging (logrus or zap) with contextual fields
- Handle errors explicitly - wrap errors with context using `fmt.Errorf` with `%w`

### Key Design Decisions

Refer to [design.md](openspec/changes/registry-webhook-scanner/design.md) for detailed architectural decisions:

1. **HTTP Service with Internal Queue**: Standalone service with Go channels for event processing
2. **Go Implementation**: For performance, concurrency, and containerization
3. **HMAC + Token Authentication**: Configurable per registry for webhook validation
4. **In-Memory Queue Initially**: Go channels with worker pool, persistent queue planned for future
5. **Environment + YAML Config**: Simple settings in env vars, complex config in YAML
6. **Direct CLI Execution**: Use `os/exec` to invoke Sysdig CLI binary
7. **Single Container**: Service + Sysdig CLI binary in one Docker image

### Configuration Management

**Environment Variables:**
- `PORT`: HTTP server port (default: 8080)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `CONFIG_FILE`: Path to YAML configuration file
- `SYSDIG_CLI_PATH`: Path to Sysdig CLI binary (default: `/usr/local/bin/sysdig-cli-scanner`)

**YAML Configuration File:**
```yaml
registries:
  - name: docker-hub
    type: dockerhub
    auth:
      type: hmac
      secret: ${DOCKERHUB_WEBHOOK_SECRET}
  - name: harbor-prod
    type: harbor
    url: https://harbor.company.com
    auth:
      type: bearer
      token: ${HARBOR_WEBHOOK_TOKEN}
    scanner:
      timeout: 600s
      credentials:
        username: ${HARBOR_USERNAME}
        password: ${HARBOR_PASSWORD}

scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}
  default_timeout: 300s
  max_concurrent: 5

queue:
  buffer_size: 100
  workers: 3
```

### Webhook Parsing

Each registry has its own webhook format. Implement parsers as:

```go
type WebhookParser interface {
    Parse(body []byte) (*ImageEvent, error)
    Validate(headers http.Header, body []byte) error
}

type ImageEvent struct {
    Registry  string
    Image     string
    Tag       string
    Digest    string    // Optional
    PushedAt  time.Time
}
```

Supported registries (see [specs](openspec/changes/registry-webhook-scanner/specs/)):
- Docker Hub
- Harbor
- GitLab Container Registry

### Sysdig CLI Integration

The scanner integration must:
- Execute CLI with proper arguments: `sysdig-cli-scanner scan <image>`
- Use Go's `context.Context` for timeout management
- Capture stdout/stderr for logging
- Differentiate between scan failures and vulnerability findings
- Handle process cleanup on timeout or service shutdown

Example scanner invocation:
```go
ctx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()

cmd := exec.CommandContext(ctx, scannerPath, "scan", imageRef)
cmd.Env = append(os.Environ(), "SYSDIG_API_TOKEN="+apiToken)

output, err := cmd.CombinedOutput()
```

### Testing Strategy

- **Unit Tests**: Test webhook parsers, config loading, queue logic independently
- **Integration Tests**: Test full webhook → queue → scanner flow with mocked CLI
- **E2E Tests**: Deploy service and send real webhooks, verify CLI invocation
- **Registry Tests**: Table-driven tests for each supported registry format

### Security Considerations

- **Never log secrets**: Mask webhook secrets, API tokens, registry credentials in logs
- **Validate webhook authenticity**: Always authenticate before processing
- **Limit payload size**: Set maximum request body size (default: 1MB)
- **Process isolation**: Use Go contexts for timeout and cancellation
- **Secret management**: Support reading secrets from files (Kubernetes Secrets as volumes)

### Observability

**Structured Logging:**
```go
logger.WithFields(log.Fields{
    "registry": "harbor-prod",
    "image": "myapp:v1.2.3",
    "event_id": "abc-123",
}).Info("webhook received")
```

**Health Endpoints:**
- `GET /health`: Always returns 200 if service is running
- `GET /ready`: Returns 200 if service can accept webhooks, 503 if overloaded

**Metrics (Future):**
- Webhook count by registry and status
- Scanner invocation count, duration, success/failure rate
- Queue depth and processing time

### Deployment

**Dockerfile Structure:**
```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o webhook-server cmd/webhook-server/main.go

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /app/webhook-server /usr/local/bin/
COPY sysdig-cli-scanner /usr/local/bin/  # Downloaded separately
ENTRYPOINT ["/usr/local/bin/webhook-server"]
```

**Kubernetes Deployment:**
- Use ConfigMap for registry configurations (non-sensitive)
- Use Secrets for webhook secrets, API tokens, registry credentials
- Set resource limits (CPU, memory) appropriate for concurrent scans
- Use liveness and readiness probes pointing to `/health` and `/ready`
- Consider HorizontalPodAutoscaler for scaling based on queue depth (future)

### Common Development Tasks

**Run locally:**
```bash
export CONFIG_FILE=config.yaml
export SYSDIG_API_TOKEN=your-token
go run cmd/webhook-server/main.go
```

**Test webhook locally:**
```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=..." \
  -d @test-payloads/dockerhub.json
```

**Build Docker image:**
```bash
docker build -t registry-webhook-scanner:latest .
```

**Run in Docker:**
```bash
docker run -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  -e CONFIG_FILE=/config.yaml \
  -e SYSDIG_API_TOKEN=your-token \
  registry-webhook-scanner:latest
```

**Deploy to Kubernetes:**
```bash
kubectl create namespace webhook-scanner
kubectl create secret generic scanner-secrets \
  --from-literal=sysdig-token=your-token \
  --from-literal=harbor-token=harbor-token \
  -n webhook-scanner
kubectl apply -f deployments/k8s/ -n webhook-scanner
```

### Git Workflow

- Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- Create feature branches from `main`
- Keep commits focused and atomic
- Write descriptive commit messages explaining the "why"

### OpenSpec Workflow

This project uses OpenSpec for change management. Key commands:

- **Start new change**: `/opsx:new` - Create a new change with proposal
- **Continue change**: `/opsx:continue` - Generate next artifact (design, specs, tasks)
- **Implement tasks**: `/opsx:apply` - Work through implementation tasks
- **Verify change**: `/opsx:verify` - Check implementation completeness
- **Archive change**: `/opsx:archive` - Finalize and archive the change

Current change artifacts are in [openspec/changes/registry-webhook-scanner/](openspec/changes/registry-webhook-scanner/).

### Additional Resources

- **OpenSpec Proposal**: [openspec/changes/registry-webhook-scanner/proposal.md](openspec/changes/registry-webhook-scanner/proposal.md)
- **Architecture Design**: [openspec/changes/registry-webhook-scanner/design.md](openspec/changes/registry-webhook-scanner/design.md)
- **Webhook Receiver Spec**: [openspec/changes/registry-webhook-scanner/specs/webhook-receiver/spec.md](openspec/changes/registry-webhook-scanner/specs/webhook-receiver/spec.md)
- **Scanner Integration Spec**: [openspec/changes/registry-webhook-scanner/specs/scanner-integration/spec.md](openspec/changes/registry-webhook-scanner/specs/scanner-integration/spec.md)
- **Sysdig CLI Scanner Docs**: https://docs.sysdig.com/en/docs/sysdig-secure/scanning/

## Common Issues and Solutions

### Issue: Webhook authentication failures
- **Cause**: Incorrect HMAC secret or token configuration
- **Solution**: Verify webhook secret in registry settings matches config.yaml

### Issue: Scanner binary not found
- **Cause**: Sysdig CLI not installed or not in PATH
- **Solution**: Ensure CLI is installed at expected path, set `SYSDIG_CLI_PATH` env var

### Issue: Scans timing out
- **Cause**: Large images or slow network
- **Solution**: Increase `scanner.default_timeout` in config, consider per-registry timeouts

### Issue: Event loss on restart
- **Cause**: In-memory queue discarded on crash/restart
- **Solution**: This is expected behavior in v1. Plan to add persistent queue (Redis) in future

### Issue: High memory usage with many concurrent scans
- **Cause**: Too many scanner processes running simultaneously
- **Solution**: Lower `scanner.max_concurrent` in config, add resource limits in Kubernetes

## Future Enhancements

Planned improvements (see [design.md](openspec/changes/registry-webhook-scanner/design.md#migration-plan)):

1. **Persistent Queue**: Add Redis or database-backed queue for event durability
2. **Metrics Endpoint**: Expose Prometheus metrics for observability
3. **Circuit Breaker**: Prevent cascading failures when scanner repeatedly fails
4. **Horizontal Scaling**: Support multiple service instances with shared queue
5. **Additional Registries**: Add parsers for AWS ECR, Google GCR, Azure ACR
6. **Webhook Deduplication**: Avoid scanning same image:tag multiple times in short window
7. **Result Caching**: Cache scan results to reduce redundant scans

## Questions?

For questions about this project or contributions, please refer to the [README.md](README.md) or contact the project maintainers.

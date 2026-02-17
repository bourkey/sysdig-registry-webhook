## 1. Project Setup

- [x] 1.1 Initialize Go module with `go mod init`
- [x] 1.2 Create project directory structure (cmd/, pkg/, internal/)
- [x] 1.3 Add core dependencies (HTTP router, YAML parser, structured logging library)
- [x] 1.4 Set up .gitignore for Go projects
- [x] 1.5 Create initial README.md with project description and goals

## 2. Configuration Management

- [x] 2.1 Define configuration schema for YAML config file (registries, secrets, scanner settings)
- [x] 2.2 Implement YAML config file loader with validation
- [x] 2.3 Implement environment variable parser for simple settings (port, log level, concurrency)
- [x] 2.4 Add configuration validation on startup
- [x] 2.5 Create example configuration file (config.example.yaml)
- [x] 2.6 Implement secret loading from files (for Kubernetes Secret volumes)

## 3. Core Data Models

- [x] 3.1 Define ScanRequest struct (image reference, registry, metadata)
- [x] 3.2 Define RegistryConfig struct (name, type, auth config, webhook secret)
- [x] 3.3 Define WebhookParser interface for registry-specific parsing
- [x] 3.4 Define ScanResult struct (status, output, timestamp, duration)

## 4. Webhook Receiver - HTTP Server

- [x] 4.1 Implement HTTP server setup with graceful shutdown
- [x] 4.2 Add webhook POST endpoint handler (/webhook)
- [x] 4.3 Add health check endpoint (/health)
- [x] 4.4 Add readiness check endpoint (/ready)
- [x] 4.5 Implement request size limit middleware
- [x] 4.6 Add structured logging middleware for all requests
- [x] 4.7 Implement HTTP method validation (POST only for webhook endpoint)

## 5. Webhook Authentication

- [x] 5.1 Implement HMAC signature verification function
- [x] 5.2 Implement bearer token validation function
- [x] 5.3 Add authentication middleware that routes to HMAC or token validation based on config
- [x] 5.4 Add authentication failure logging (without exposing secrets)
- [x] 5.5 Test authentication with sample requests

## 6. Webhook Parsing - Registry Adapters

- [x] 6.1 Implement Docker Hub webhook parser (parse repository, tag, digest)
- [x] 6.2 Implement Harbor webhook parser (parse project, repository, tag, digest)
- [x] 6.3 Implement GitLab Container Registry webhook parser (parse path, tag, digest)
- [x] 6.4 Add parser registry/factory to select parser based on config or webhook format
- [x] 6.5 Implement image reference normalization (registry/repository:tag format)
- [x] 6.6 Add support for extracting multiple tags from single webhook
- [x] 6.7 Implement validation for required fields in parsed webhooks

## 7. Event Processing - Queue Implementation

- [x] 7.1 Implement in-memory queue using Go channels
- [x] 7.2 Add queue depth tracking and metrics
- [x] 7.3 Implement queue capacity limits with HTTP 503 response when full
- [x] 7.4 Add thread-safe enqueue operation
- [x] 7.5 Implement FIFO dequeue for worker pool

## 8. Event Processing - Worker Pool

- [x] 8.1 Implement worker pool initialization with configurable concurrency
- [x] 8.2 Add worker goroutines that consume from queue
- [x] 8.3 Implement worker lifecycle management (start, stop, monitor)
- [x] 8.4 Add graceful shutdown for workers (wait for in-flight scans)
- [x] 8.5 Implement worker error handling and recovery

## 9. Event Processing - Deduplication

- [x] 9.1 Implement time-window deduplication cache (map with TTL)
- [x] 9.2 Add deduplication key generation (image:tag or digest-based)
- [x] 9.3 Implement cache cleanup/eviction for expired entries
- [x] 9.4 Add deduplication logging and metrics

## 10. Event Processing - Retry Logic

- [x] 10.1 Implement retry counter in ScanRequest
- [x] 10.2 Add exponential backoff calculator (1s, 2s, 4s, 8s, ...)
- [x] 10.3 Implement retry decision logic (transient vs permanent failures)
- [x] 10.4 Add maximum retry limit configuration
- [x] 10.5 Implement re-enqueue logic for failed scans with backoff delay
- [x] 10.6 Add retry metrics and logging

## 11. Scanner Integration - CLI Execution

- [x] 11.1 Implement Sysdig CLI invocation using os/exec
- [x] 11.2 Add command builder to format CLI arguments from ScanRequest
- [x] 11.3 Implement stdout/stderr capture
- [x] 11.4 Add exit code handling (0=success, non-zero=check if scan completed or failed)
- [x] 11.5 Implement process timeout using context.WithTimeout
- [x] 11.6 Add process cleanup on timeout (SIGTERM then SIGKILL)

## 12. Scanner Integration - Credential Management

- [x] 12.1 Implement Sysdig API token injection (environment variable or CLI flag)
- [x] 12.2 Add registry credential provider for private registries
- [x] 12.3 Implement credential validation on startup
- [x] 12.4 Ensure credentials are never logged

## 13. Scanner Integration - Result Processing

- [x] 13.1 Implement scan output parser to extract key information
- [x] 13.2 Add scan result logging with structured fields
- [x] 13.3 Implement scan metrics (count, duration, success/failure rate)
- [x] 13.4 Add optional result caching to avoid duplicate scans

## 14. Observability - Logging

- [x] 14.1 Set up structured logging library (logrus, zap, or slog)
- [x] 14.2 Implement log level configuration (debug, info, warn, error)
- [x] 14.3 Add request ID generation and propagation for tracing
- [x] 14.4 Add lifecycle event logging (startup, shutdown, configuration loaded)
- [x] 14.5 Ensure all error paths are logged with context

## 15. Observability - Metrics (Optional)

- [ ] 15.1 Add Prometheus metrics endpoint (/metrics)
- [ ] 15.2 Implement webhook counter (by registry, status code)
- [ ] 15.3 Implement scan counter (by registry, result)
- [ ] 15.4 Add queue depth gauge
- [ ] 15.5 Add scan duration histogram

## 16. Graceful Shutdown

- [x] 16.1 Implement signal handler for SIGTERM and SIGINT
- [x] 16.2 Stop accepting new webhooks on shutdown signal
- [x] 16.3 Wait for worker pool to complete in-flight scans (with timeout)
- [x] 16.4 Close queue and cleanup resources
- [x] 16.5 Add shutdown logging and metrics flush

## 17. Docker Image

- [x] 17.1 Create multi-stage Dockerfile (build Go binary, copy to minimal image)
- [x] 17.2 Add Sysdig CLI Scanner binary to Docker image
- [x] 17.3 Configure container to run as non-root user
- [x] 17.4 Add health check configuration in Dockerfile
- [x] 17.5 Document Sysdig CLI version pinning strategy
- [x] 17.6 Build and test Docker image locally

## 18. Kubernetes Deployment

- [x] 18.1 Create Kubernetes Deployment manifest
- [x] 18.2 Create ConfigMap for configuration file
- [x] 18.3 Create Secret for webhook secrets and registry credentials
- [x] 18.4 Create Service for webhook endpoint
- [x] 18.5 Create Ingress for external webhook access with TLS
- [x] 18.6 Add resource limits and requests
- [x] 18.7 Configure health and readiness probes
- [x] 18.8 Add deployment documentation

## 19. Testing - Unit Tests

- [x] 19.1 Write unit tests for webhook authentication (HMAC and token)
- [x] 19.2 Write unit tests for registry parsers (Docker Hub, Harbor, GitLab)
- [x] 19.3 Write unit tests for retry logic and exponential backoff
- [x] 19.4 Write unit tests for deduplication cache
- [x] 19.5 Write unit tests for configuration loading and validation
- [x] 19.6 Write unit tests for image reference normalization

## 20. Testing - Integration Tests

- [x] 20.1 Create mock webhook payloads for each registry type
- [x] 20.2 Write integration test for end-to-end webhook to scan flow
- [x] 20.3 Create mock Sysdig CLI for testing (script that simulates CLI behavior)
- [x] 20.4 Write test for authentication failure scenarios
- [x] 20.5 Write test for queue overflow handling
- [x] 20.6 Write test for graceful shutdown with in-flight scans

## 21. Documentation

- [x] 21.1 Complete README with architecture overview and component descriptions
- [x] 21.2 Document configuration options (YAML schema, environment variables)
- [x] 21.3 Add registry setup guide (how to configure webhooks in Docker Hub, Harbor, GitLab)
- [x] 21.4 Document deployment instructions for Kubernetes
- [x] 21.5 Add troubleshooting guide (common errors, log interpretation)
- [x] 21.6 Document how to add support for new registry types
- [x] 21.7 Add security considerations (secret management, network policies)

## 22. Manual Testing & Validation

- [ ] 22.1 Deploy to test Kubernetes cluster
- [ ] 22.2 Configure test registries to send webhooks
- [ ] 22.3 Push test images and verify webhooks are received
- [ ] 22.4 Verify scans are triggered successfully
- [ ] 22.5 Test authentication failure handling
- [ ] 22.6 Test retry logic with simulated failures
- [ ] 22.7 Validate graceful shutdown behavior
- [ ] 22.8 Load test with multiple concurrent webhooks

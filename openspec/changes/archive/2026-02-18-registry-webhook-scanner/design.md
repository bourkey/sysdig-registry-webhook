## Context

This is a new standalone service that bridges container registry webhooks with Sysdig CLI Scanner. The service addresses the security gap where container images are pushed to registries without automated vulnerability scanning.

**Current State:**
- Container registries (Docker Hub, Harbor, GitLab Registry, etc.) emit webhooks on image push/update events
- Sysdig CLI Scanner exists and can scan container images, but requires manual invocation
- No automated connection between registry events and security scanning

**Constraints:**
- Must support multiple container registry types with different webhook formats
- Sysdig CLI Scanner is installed as a binary and invoked via command-line
- Service must be reliable enough for production use (handle failures, retries, observability)
- Should run in containerized environments (Kubernetes, Docker)

## Goals / Non-Goals

**Goals:**
- Automatically trigger Sysdig CLI Scanner when container images are pushed to registries
- Support multiple container registry providers (Docker Hub, Harbor, GitLab Registry, others)
- Provide reliable event processing with retry logic for transient failures
- Make the service observable through structured logging and health endpoints
- Allow flexible configuration for different registry credentials and webhook authentication

**Non-Goals:**
- Not a replacement for Sysdig's native registry scanning capabilities
- Not responsible for storing or managing scan results long-term (Sysdig handles this)
- Not a general-purpose webhook processing framework
- Not implementing custom vulnerability scanning logic
- Not providing a UI or dashboard (logs and external monitoring tools suffice)

## Decisions

### 1. Service Architecture: HTTP Service with Internal Queue

**Decision:** Build as a standalone HTTP service with an internal event queue.

**Alternatives Considered:**
- **Serverless (AWS Lambda, Cloud Functions):** Would reduce operational overhead but adds cloud provider lock-in and complexity in managing Sysdig CLI binary in serverless environment
- **Message Queue-based (RabbitMQ, Kafka):** Would provide better durability but adds infrastructure complexity and operational burden for the initial version

**Rationale:** An HTTP service with internal queueing strikes the right balance between simplicity and reliability. It can run anywhere containers run, doesn't require external message queue infrastructure, and is easier to develop and debug.

### 2. Implementation Language: Go

**Decision:** Implement the service in Go.

**Alternatives Considered:**
- **Python:** Easier to write initially, rich ecosystem, but slower runtime and more complex containerization
- **Node.js:** Good for HTTP services and async processing, but less common for infrastructure tools in this space

**Rationale:** Go is well-suited for building HTTP services and CLI tools, has excellent concurrency primitives for queue processing, produces small self-contained binaries for easy containerization, and is commonly used in infrastructure and security tooling.

### 3. Webhook Authentication: Configurable HMAC + Token Validation

**Decision:** Support both HMAC signature verification (e.g., GitHub-style) and bearer token authentication, configurable per registry.

**Alternatives Considered:**
- **mTLS only:** More secure but harder to configure and not supported by all registries
- **No authentication:** Simpler but unacceptable for production security

**Rationale:** Most container registries support either HMAC-based signatures or static tokens for webhook authentication. Making it configurable per registry provides flexibility while maintaining security.

### 4. Event Queue: In-Memory Initially, Persistent Later

**Decision:** Start with an in-memory queue (Go channels + worker pool), with design to support persistent queue (Redis, database) in future versions.

**Alternatives Considered:**
- **Redis from the start:** More durable but adds deployment complexity
- **Database queue:** More durable but adds latency and complexity

**Rationale:** For the initial version, an in-memory queue simplifies deployment and is sufficient if restarts are infrequent. The service should be designed so a persistent queue can be added later without major refactoring.

### 5. Configuration: Environment Variables + Config File

**Decision:** Use environment variables for simple settings (port, log level) and a YAML config file for complex settings (registry configurations, webhook secrets).

**Alternatives Considered:**
- **Environment variables only:** Gets unwieldy with many registries and secrets
- **Config service (etcd, Consul):** Adds infrastructure complexity

**Rationale:** This follows 12-factor app principles while keeping complex structured configuration manageable. Config file can be mounted as a Kubernetes ConfigMap/Secret.

### 6. Sysdig CLI Invocation: Direct Execution

**Decision:** Invoke the Sysdig CLI Scanner binary directly using Go's `os/exec` package.

**Alternatives Considered:**
- **Wrap Sysdig API directly:** Would avoid subprocess overhead but duplicates existing CLI functionality and adds maintenance burden
- **REST API wrapper around CLI:** Adds unnecessary abstraction layer

**Rationale:** The Sysdig CLI already exists and is maintained. Direct execution is simpler and ensures we benefit from CLI updates without additional work.

### 7. Deployment Model: Single Container with CLI Binary

**Decision:** Package service and Sysdig CLI binary in a single Docker container image.

**Alternatives Considered:**
- **Sidecar pattern:** Service in one container, CLI in another - adds orchestration complexity
- **CLI installed at runtime:** Slower startup, less reproducible

**Rationale:** Single container is simpler to deploy and ensures the CLI version is locked and tested with the service version.

## Risks / Trade-offs

**[Event Loss on Service Restart]** → In-memory queue means unprocessed events are lost if the service crashes or restarts.
- **Mitigation:** Keep processing queue small with quick scan invocation. Plan to add persistent queue in v2 if event loss becomes problematic in production.

**[Single Instance Bottleneck]** → A single service instance may struggle with high webhook volumes.
- **Mitigation:** Design the service to be stateless (except in-memory queue) so multiple instances can run with a load balancer. Consider adding rate limiting per registry.

**[Subprocess Management Complexity]** → Invoking CLI as subprocess requires careful handling of stdout/stderr, exit codes, timeouts, and resource limits.
- **Mitigation:** Use Go's context for timeouts, properly capture and log CLI output, implement process monitoring and cleanup.

**[Registry Format Variability]** → Different registries send different webhook payloads, which increases maintenance burden.
- **Mitigation:** Abstract webhook parsing behind a common interface. Each registry gets a parser implementation. Start with 2-3 popular registries and document how to add new ones.

**[Network Dependency]** → Service depends on network connectivity to registries (for validation) and Sysdig backend (for scanning).
- **Mitigation:** Implement exponential backoff retry logic. Use health checks to detect network issues. Consider circuit breaker pattern if repeated failures occur.

**[Secret Management]** → Webhook secrets and registry credentials must be securely stored and accessed.
- **Mitigation:** Support reading secrets from files (Kubernetes Secrets mounted as volumes) and environment variables. Never log secrets. Consider integration with secret managers (HashiCorp Vault) in future.

**[Scanner CLI Versioning]** → Sysdig CLI updates may change behavior or flags.
- **Mitigation:** Pin CLI version in container image. Test new CLI versions before updating. Document CLI version compatibility in README.

## Migration Plan

**Initial Deployment:**
1. Build Docker image containing the Go service and Sysdig CLI binary
2. Deploy to Kubernetes with ConfigMap for registry configurations and Secret for webhook/registry credentials
3. Configure registries to send webhooks to the service endpoint (behind ingress with TLS)
4. Monitor logs and metrics to ensure webhooks are received and scans are triggered

**Rollback Strategy:**
- If the service fails, registries can be reconfigured to remove webhook endpoints
- No data loss risk since the service doesn't store critical data (scans are in Sysdig backend)
- Previous version can be deployed by rolling back container image

**Future Evolution:**
- Add persistent queue (Redis/database) if event loss becomes an issue
- Add metrics endpoint (Prometheus) for observability
- Support additional container registries
- Add circuit breaker for scan invocation failures
- Consider horizontal pod autoscaling based on queue depth

## Open Questions

1. **Should we support batch scanning?** If multiple images are pushed rapidly, should we queue them individually or batch them into a single CLI invocation?
   - Leaning toward individual invocations for simpler failure handling and better observability per image

2. **How should we handle scan failures?** If the Sysdig CLI returns a non-zero exit code (e.g., critical vulnerabilities found), should we retry or treat it as success?
   - Likely treat as success (scan completed), but log/expose the result for monitoring

3. **What level of webhook validation?** Should we verify image signatures or just trust the registry webhook?
   - Start with trusting authenticated webhooks, add signature verification if required later

4. **Should we deduplicate webhook events?** Same image might trigger multiple webhooks.
   - Implement simple time-window deduplication (ignore same image:tag within N seconds)

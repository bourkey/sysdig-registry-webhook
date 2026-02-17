# Registry Webhook Scanner

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)

Automatically trigger Sysdig CLI Scanner when container images are pushed to registries.

## Overview

Registry Webhook Scanner is a standalone service that bridges container registry webhooks with Sysdig CLI Scanner. When a container image is pushed to a registry (Docker Hub, Harbor, GitLab Registry, etc.), the registry sends a webhook to this service, which automatically invokes the Sysdig CLI to scan the image for vulnerabilities.

**Key Features:**
- 🔔 Receives webhooks from multiple container registry providers
- 🔐 Authenticates webhooks using HMAC signatures or bearer tokens
- 🔍 Automatically triggers Sysdig CLI Scanner for pushed images
- ⚡ Concurrent scanning with configurable worker pools
- 📊 Structured logging for audit trails and troubleshooting
- 🏥 Health and readiness endpoints for Kubernetes
- 🐳 Single Docker container with embedded Sysdig CLI

## Why This Exists

Container registries emit webhooks when images are pushed, but there's no built-in mechanism to trigger security scanning on these events. This creates a security gap where container images may be deployed to production without vulnerability assessment. This service closes that gap by automatically scanning images as they're pushed to registries.

## Architecture

```
┌─────────────────┐
│ Container       │
│ Registry        │ Webhook (push event)
│ (Harbor, etc.)  │────────────────┐
└─────────────────┘                │
                                   ▼
                        ┌──────────────────────┐
                        │  Webhook Receiver    │
                        │  - Authenticate      │
                        │  - Parse payload     │
                        │  - Extract image ref │
                        └──────────┬───────────┘
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  Event Queue         │
                        │  (in-memory)         │
                        └──────────┬───────────┘
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  Scanner Worker Pool │
                        │  - Invoke CLI        │
                        │  - Manage processes  │
                        │  - Log results       │
                        └──────────┬───────────┘
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  Sysdig CLI Scanner  │
                        │  (embedded binary)   │
                        └──────────────────────┘
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  Sysdig Backend      │
                        │  (scan results)      │
                        └──────────────────────┘
```

## Quick Start

### Prerequisites

- Docker or Kubernetes cluster
- Sysdig API token (get from Sysdig Secure)
- Container registry with webhook support

### 1. Configuration

Create a `config.yaml` file:

```yaml
registries:
  - name: dockerhub
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

### 2. Run with Docker

```bash
docker run -d \
  --name webhook-scanner \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  -e CONFIG_FILE=/config.yaml \
  -e SYSDIG_API_TOKEN=your-sysdig-token \
  registry-webhook-scanner:latest
```

### 3. Configure Registry Webhooks

Configure your container registry to send webhooks to:
```
http://your-server:8080/webhook
```

**Docker Hub:**
- Go to repository → Webhooks → Add webhook
- Add URL and optional secret

**Harbor:**
- Go to Project → Webhooks → New Webhook
- Add endpoint URL and authentication token

**GitLab Registry:**
- Go to Settings → Webhooks → Add webhook
- Add URL and configure push events

### 4. Verify Setup

Check the service is running:
```bash
curl http://localhost:8080/health
# Response: {"status": "healthy"}

curl http://localhost:8080/ready
# Response: {"status": "ready"}
```

Watch logs to see webhook processing:
```bash
docker logs -f webhook-scanner
```

## Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  webhook-scanner:
    image: registry-webhook-scanner:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/config.yaml
    environment:
      - CONFIG_FILE=/config.yaml
      - SYSDIG_API_TOKEN=${SYSDIG_API_TOKEN}
      - LOG_LEVEL=info
    restart: unless-stopped
```

### Kubernetes

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: webhook-scanner
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: scanner-config
  namespace: webhook-scanner
data:
  config.yaml: |
    registries:
      - name: harbor-prod
        type: harbor
        url: https://harbor.company.com
        auth:
          type: bearer
          token: ${HARBOR_WEBHOOK_TOKEN}
    scanner:
      sysdig_token: ${SYSDIG_API_TOKEN}
      default_timeout: 300s
      max_concurrent: 5
---
apiVersion: v1
kind: Secret
metadata:
  name: scanner-secrets
  namespace: webhook-scanner
type: Opaque
stringData:
  sysdig-token: your-sysdig-api-token
  harbor-token: your-harbor-webhook-token
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook-scanner
  namespace: webhook-scanner
spec:
  replicas: 2
  selector:
    matchLabels:
      app: webhook-scanner
  template:
    metadata:
      labels:
        app: webhook-scanner
    spec:
      containers:
      - name: scanner
        image: registry-webhook-scanner:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_FILE
          value: /config/config.yaml
        - name: SYSDIG_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: scanner-secrets
              key: sysdig-token
        - name: HARBOR_WEBHOOK_TOKEN
          valueFrom:
            secretKeyRef:
              name: scanner-secrets
              key: harbor-token
        volumeMounts:
        - name: config
          mountPath: /config
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: scanner-config
---
apiVersion: v1
kind: Service
metadata:
  name: webhook-scanner
  namespace: webhook-scanner
spec:
  selector:
    app: webhook-scanner
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

Deploy:
```bash
kubectl apply -f deployments/kubernetes/
```

## Configuration Reference

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `CONFIG_FILE` | Path to YAML configuration file | `config.yaml` |
| `SYSDIG_CLI_PATH` | Path to Sysdig CLI binary | `/usr/local/bin/sysdig-cli-scanner` |

### YAML Configuration

#### Registry Configuration

```yaml
registries:
  - name: string              # Unique identifier for this registry
    type: string              # Registry type: dockerhub, harbor, gitlab
    url: string               # Registry URL (optional for Docker Hub)
    auth:
      type: string            # Authentication: hmac or bearer
      secret: string          # HMAC secret (for type: hmac)
      token: string           # Bearer token (for type: bearer)
    scanner:
      timeout: duration       # Scanner timeout for this registry
      credentials:            # Registry credentials (for private registries)
        username: string
        password: string
```

#### Scanner Configuration

```yaml
scanner:
  sysdig_token: string        # Sysdig API token (required)
  default_timeout: duration   # Default scan timeout (e.g., "300s", "5m")
  max_concurrent: int         # Max concurrent scans (default: 5)
```

#### Queue Configuration

```yaml
queue:
  buffer_size: int            # Event queue buffer size (default: 100)
  workers: int                # Number of worker goroutines (default: 3)
```

## Supported Registries

| Registry | Type Value | Authentication |
|----------|------------|----------------|
| Docker Hub | `dockerhub` | HMAC signature or none |
| Harbor | `harbor` | Bearer token |
| GitLab Registry | `gitlab` | Token |

Need support for another registry? [Open an issue](https://github.com/yourusername/registry-webhook-scanner/issues) or submit a PR!

## API Endpoints

### Webhook Endpoint

**POST /webhook**

Receives webhook events from container registries.

**Headers:**
- `Content-Type: application/json`
- `X-Hub-Signature-256: sha256=...` (for HMAC auth)
- `Authorization: Bearer <token>` (for bearer auth)

**Response:**
- `200 OK`: Webhook accepted and queued for processing
- `400 Bad Request`: Invalid payload or missing required fields
- `401 Unauthorized`: Authentication failed
- `405 Method Not Allowed`: Invalid HTTP method
- `413 Payload Too Large`: Payload exceeds size limit

### Health Endpoint

**GET /health**

Returns service health status.

**Response:**
```json
{
  "status": "healthy"
}
```

### Readiness Endpoint

**GET /ready**

Returns service readiness status.

**Response:**
```json
{
  "status": "ready"
}
```

## Monitoring and Observability

### Structured Logging

The service uses structured JSON logging:

```json
{
  "level": "info",
  "time": "2024-01-15T10:30:45Z",
  "msg": "webhook received",
  "registry": "harbor-prod",
  "image": "myapp:v1.2.3",
  "event_id": "abc-123"
}
```

### Log Levels

- `debug`: Detailed execution flow and data
- `info`: Webhook receipts, scan starts/completions
- `warn`: Non-fatal issues (timeouts, retries)
- `error`: Fatal errors requiring attention

### Metrics (Coming Soon)

Future versions will expose Prometheus metrics:
- `webhook_requests_total`: Total webhook requests by registry and status
- `scan_invocations_total`: Total scanner invocations by status
- `scan_duration_seconds`: Scanner execution duration histogram
- `queue_depth`: Current event queue depth

## Troubleshooting

### Webhook Not Received

**Symptoms:** No logs showing webhook receipt

**Checks:**
1. Verify service is accessible from registry (network, firewall)
2. Check registry webhook configuration (URL, headers)
3. Review registry webhook delivery logs
4. Test with curl: `curl -X POST http://your-server:8080/webhook -d '...'`

### Authentication Failures

**Symptoms:** `401 Unauthorized` responses

**Solutions:**
1. Verify webhook secret/token matches config.yaml
2. For HMAC: Check signature header name and format
3. For bearer tokens: Ensure `Authorization` header is sent
4. Enable debug logging: `LOG_LEVEL=debug`

### Scanner Not Found

**Symptoms:** Logs show "scanner binary not found"

**Solutions:**
1. Verify Sysdig CLI is installed in container
2. Check `SYSDIG_CLI_PATH` environment variable
3. Ensure binary has execute permissions

### Scans Timing Out

**Symptoms:** Logs show "scan timeout exceeded"

**Solutions:**
1. Increase `scanner.default_timeout` in config
2. Set per-registry timeout for large images
3. Check network connectivity to Sysdig backend
4. Verify Sysdig API token is valid

### High Memory Usage

**Symptoms:** Container OOM killed or high memory consumption

**Solutions:**
1. Reduce `scanner.max_concurrent` to limit concurrent scans
2. Increase container memory limits in Kubernetes
3. Lower `queue.buffer_size` to reduce memory footprint

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/yourusername/registry-webhook-scanner.git
cd registry-webhook-scanner

# Build binary
go build -o webhook-server cmd/webhook-server/main.go

# Run locally
export CONFIG_FILE=config.yaml
export SYSDIG_API_TOKEN=your-token
./webhook-server
```

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests
go test -tags=integration ./...

# Coverage
go test -cover ./...
```

### Building Docker Image

```bash
docker build -t registry-webhook-scanner:latest .
```

## Security Considerations

- **Webhook Authentication**: Always configure webhook authentication (HMAC or bearer tokens)
- **Secrets Management**: Use Kubernetes Secrets or environment variables for sensitive data
- **Network Policies**: Restrict ingress to webhook endpoint and egress to registries/Sysdig
- **TLS**: Use HTTPS for webhook endpoints (configure ingress/load balancer with TLS)
- **Resource Limits**: Set memory and CPU limits to prevent resource exhaustion

## Limitations (v1)

- **In-Memory Queue**: Events are lost on service restart (persistent queue planned for v2)
- **Single Instance**: Service is designed to run as single instance (horizontal scaling planned)
- **Limited Registry Support**: Currently supports Docker Hub, Harbor, GitLab (more coming)
- **No Result Storage**: Scan results are only in Sysdig backend (caching planned)

## Roadmap

- [ ] Persistent event queue (Redis/database)
- [ ] Prometheus metrics endpoint
- [ ] Horizontal scaling support with shared queue
- [ ] Additional registry support (AWS ECR, Google GCR, Azure ACR)
- [ ] Webhook deduplication
- [ ] Result caching
- [ ] Circuit breaker pattern
- [ ] Web UI for status and configuration

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

See [CLAUDE.md](CLAUDE.md) for development guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Sysdig](https://sysdig.com) for the CLI Scanner
- Container registry webhook documentation and examples
- The Go community for excellent libraries and tools

## Support

- **Issues**: [GitHub Issues](https://github.com/yourusername/registry-webhook-scanner/issues)
- **Documentation**: [OpenSpec Design Docs](openspec/changes/registry-webhook-scanner/)
- **Sysdig Docs**: https://docs.sysdig.com/en/docs/sysdig-secure/scanning/

---

**Built with ❤️ for automated container security**

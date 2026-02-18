# Configuration Guide

Complete reference for configuring the Registry Webhook Scanner.

## Configuration Methods

The scanner supports multiple configuration methods:

1. **YAML Configuration File** - For complex settings (registries, scanner config)
2. **Environment Variables** - For simple settings and overrides
3. **Secret Files** - For Kubernetes Secrets mounted as volumes

## Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | HTTP server port | `8080` | No |
| `LOG_LEVEL` | Logging level: debug, info, warn, error | `info` | No |
| `CONFIG_FILE` | Path to YAML configuration file | `config.yaml` | Yes |
| `SYSDIG_CLI_PATH` | Path to Sysdig CLI Scanner binary | `/usr/local/bin/sysdig-cli-scanner` | No |

**Example:**
```bash
export PORT=8080
export LOG_LEVEL=debug
export CONFIG_FILE=/app/config/config.yaml
export SYSDIG_CLI_PATH=/usr/local/bin/sysdig-cli-scanner
```

## YAML Configuration Schema

### Complete Example

```yaml
# HTTP Server Configuration
server:
  port: 8080                    # HTTP port (default: 8080)
  read_timeout: 30s             # Request read timeout
  write_timeout: 30s            # Response write timeout
  max_request_size: 10485760    # Max payload size in bytes (10MB)
  shutdown_timeout: 30s         # Graceful shutdown timeout

# Registry Configurations
registries:
  - name: dockerhub             # Unique registry identifier
    type: dockerhub             # Registry type: dockerhub, harbor, gitlab
    auth:
      type: hmac                # Auth type: hmac, bearer, or none
      secret: ${DOCKERHUB_SECRET}  # Webhook secret (env var or ${FILE:secret-name})

  - name: harbor-prod
    type: harbor
    url: https://harbor.company.com
    auth:
      type: bearer
      secret: ${HARBOR_TOKEN}
    scanner:
      timeout: 600s             # Registry-specific scan timeout
      credentials:              # For private registries
        username: ${HARBOR_USER}
        password: ${HARBOR_PASS}

  - name: gitlab-registry
    type: gitlab
    url: https://registry.gitlab.com
    auth:
      type: bearer
      secret: ${GITLAB_TOKEN}
    scanner:
      timeout: 300s
      credentials:
        username: ${GITLAB_USER}
        password: ${GITLAB_PASS}

# Sysdig Scanner Configuration
scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}  # Required: Sysdig API token
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s              # Default scan timeout (5 minutes)
  max_concurrent: 5                  # Max concurrent scans

# Event Queue Configuration
queue:
  buffer_size: 100                   # Queue capacity
  workers: 3                         # Number of worker goroutines
```

### Server Configuration

```yaml
server:
  port: 8080                    # HTTP server port
  read_timeout: 30s             # Max time to read request
  write_timeout: 30s            # Max time to write response
  max_request_size: 10485760    # Max payload size (bytes)
  shutdown_timeout: 30s         # Graceful shutdown wait time
```

**Defaults:**
- `port`: 8080
- `read_timeout`: 30s
- `write_timeout`: 30s
- `max_request_size`: 10MB (10485760 bytes)
- `shutdown_timeout`: 30s

### Registry Configuration

Each registry requires:

```yaml
registries:
  - name: string              # REQUIRED: Unique identifier
    type: string              # REQUIRED: dockerhub, harbor, or gitlab
    url: string               # Optional: Registry URL (not needed for Docker Hub)
    auth:                     # REQUIRED: Authentication config
      type: string            # REQUIRED: hmac, bearer, or none
      secret: string          # Required if type is hmac or bearer
    scanner:                  # Optional: Registry-specific settings
      timeout: duration       # Override default scan timeout
      credentials:            # For private registries
        username: string
        password: string
```

**Supported Registry Types:**

| Type | Description | URL Required | Auth Methods |
|------|-------------|--------------|--------------|
| `dockerhub` | Docker Hub | No | hmac, none |
| `harbor` | Harbor Registry | Yes | bearer |
| `gitlab` | GitLab Container Registry | Yes | bearer |

### Scanner Configuration

The scanner supports two scanning backends: **CLI Scanner** (default) and **Registry Scanner**.

#### Scanner Type Selection

```yaml
scanner:
  type: string                # Scanner type: "cli" (default) or "registry"
  sysdig_token: string        # REQUIRED: Sysdig API token
  default_timeout: duration   # Default scan timeout
  max_concurrent: int         # Max parallel scans

  # CLI Scanner specific settings (used when type=cli)
  cli_path: string            # Path to CLI binary

  # Registry Scanner specific settings (required when type=registry)
  registry_scanner:
    api_url: string           # Sysdig API endpoint
    project_id: string        # REQUIRED: Sysdig project ID
    verify_tls: bool          # Verify TLS certificates (default: true)
    poll_interval: duration   # Poll interval for scan status (default: 5s)
```

**Defaults:**
- `type`: `cli` (backward compatible)
- `cli_path`: `/usr/local/bin/sysdig-cli-scanner`
- `default_timeout`: `300s` (5 minutes)
- `max_concurrent`: 5
- `registry_scanner.api_url`: `https://secure.sysdig.com`
- `registry_scanner.poll_interval`: `5s`
- `registry_scanner.verify_tls`: `true`

**Timeout Format:** Use Go duration format: `300s`, `5m`, `1h`

#### Scanner Type Comparison

| Feature | CLI Scanner | Registry Scanner |
|---------|-------------|------------------|
| **Image Download** | Yes (pulls image locally) | No (scans in registry) |
| **Bandwidth Usage** | High (downloads full image) | Low (API calls only) |
| **Scan Speed** | Slower for large images | Faster (parallel scanning) |
| **Storage Required** | Yes (local image cache) | No |
| **Best For** | On-premise, air-gapped | Cloud-native, high throughput |
| **Requirements** | Sysdig CLI binary | Sysdig project ID |

#### When to Use Each Scanner Type

**Use CLI Scanner when:**
- Running in on-premise or air-gapped environments
- You need to scan images from registries not accessible to Sysdig
- You have local storage and bandwidth available
- You're already using Sysdig CLI in your workflow

**Use Registry Scanner when:**
- Running in cloud or cloud-native environments
- Scanning many large images (reduces bandwidth)
- Registry is accessible to Sysdig API
- You want faster scan throughput
- Storage is limited or expensive

#### Per-Registry Scanner Override

You can override the global scanner type for specific registries:

```yaml
scanner:
  type: cli  # Global default

registries:
  - name: harbor-prod
    type: harbor
    scanner:
      type: registry  # Override: use Registry Scanner for this registry
      timeout: 600s
```

**Example: CLI Scanner Configuration**

```yaml
scanner:
  type: cli
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  max_concurrent: 5
```

**Example: Registry Scanner Configuration**

```yaml
scanner:
  type: registry
  sysdig_token: ${SYSDIG_API_TOKEN}
  default_timeout: 300s
  max_concurrent: 10  # Registry Scanner can handle more concurrent scans
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s
```

**Example: Mixed Configuration**

```yaml
scanner:
  type: cli  # Default to CLI Scanner
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  registry_scanner:  # Configure Registry Scanner for overrides
    project_id: ${SYSDIG_PROJECT_ID}

registries:
  - name: dockerhub
    type: dockerhub
    # Uses CLI Scanner (global default)

  - name: harbor-prod
    type: harbor
    scanner:
      type: registry  # Use Registry Scanner for Harbor
      timeout: 600s
```

### Queue Configuration

```yaml
queue:
  buffer_size: int            # Event queue capacity
  workers: int                # Worker goroutine count
```

**Defaults:**
- `buffer_size`: 100
- `workers`: 3

**Sizing Guidelines:**
- `buffer_size`: Set based on expected webhook volume
  - Low volume (<10/min): 50-100
  - Medium volume (10-50/min): 100-200
  - High volume (>50/min): 200-500
- `workers`: Set based on scan concurrency needs
  - Match or exceed `scanner.max_concurrent`
  - Consider CPU/memory resources

## Secret Management

### Environment Variables

Reference environment variables in config:

```yaml
scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}
```

Set before starting:
```bash
export SYSDIG_API_TOKEN=your-token-here
```

### Kubernetes Secrets

For secrets mounted as files:

```yaml
scanner:
  sysdig_token: ${FILE:sysdig-api-token}
```

The service will read from `/app/secrets/sysdig-api-token` (configurable).

**Creating Kubernetes Secret:**
```bash
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=sysdig-api-token='your-token'
```

**Mount in Deployment:**
```yaml
volumeMounts:
- name: secrets
  mountPath: /app/secrets
  readOnly: true
volumes:
- name: secrets
  secret:
    secretName: scanner-secrets
```

## Validation

The service validates configuration on startup and will fail fast if:

- No registries are configured
- `sysdig_token` is missing or a placeholder
- Invalid registry types
- Duplicate registry names
- Invalid duration formats
- Invalid authentication configurations

Check logs for validation errors:
```
level=error msg="invalid configuration" error="scanner.sysdig_token is required"
```

## Configuration Best Practices

1. **Use Environment Variables for Secrets** - Never commit secrets to config files
2. **Use Kubernetes Secrets in Production** - Mount secrets as files
3. **Start with Defaults** - Only override what you need
4. **Test Locally First** - Validate config before deploying
5. **Monitor Queue Depth** - Adjust `buffer_size` and `workers` based on metrics
6. **Set Appropriate Timeouts** - Large images need longer scan timeouts
7. **Use Unique Registry Names** - Makes logs easier to filter

## Troubleshooting Configuration

### Config File Not Found

```
Error: failed to read config file: no such file or directory
```

**Solution:** Verify `CONFIG_FILE` environment variable points to correct path.

### Environment Variable Not Expanded

```
Error: sysdig_token is not set (still contains placeholder)
```

**Solution:** Set the environment variable before starting:
```bash
export SYSDIG_API_TOKEN=your-token
```

### Invalid Duration Format

```
Error: invalid scanner.default_timeout: time: invalid duration
```

**Solution:** Use Go duration format: `300s`, `5m`, `1h`

### Registry Type Not Supported

```
Error: invalid registry type 'unknown', must be one of: dockerhub, harbor, gitlab
```

**Solution:** Check `type` field in registry configuration.

## Example Configurations

### Minimal Configuration

```yaml
registries:
  - name: dockerhub
    type: dockerhub
    auth:
      type: none

scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}
```

### Production Configuration

```yaml
server:
  shutdown_timeout: 60s

registries:
  - name: harbor-prod
    type: harbor
    url: https://harbor.company.com
    auth:
      type: bearer
      secret: ${FILE:harbor-token}
    scanner:
      timeout: 900s  # 15 minutes for large images
      credentials:
        username: ${FILE:harbor-username}
        password: ${FILE:harbor-password}

scanner:
  sysdig_token: ${FILE:sysdig-api-token}
  max_concurrent: 10

queue:
  buffer_size: 200
  workers: 10
```

### Multi-Registry Configuration

```yaml
registries:
  - name: dockerhub-public
    type: dockerhub
    auth:
      type: hmac
      secret: ${DOCKERHUB_SECRET}

  - name: harbor-dev
    type: harbor
    url: https://harbor-dev.company.com
    auth:
      type: bearer
      secret: ${HARBOR_DEV_TOKEN}

  - name: harbor-prod
    type: harbor
    url: https://harbor-prod.company.com
    auth:
      type: bearer
      secret: ${HARBOR_PROD_TOKEN}
    scanner:
      timeout: 900s

  - name: gitlab-registry
    type: gitlab
    url: https://registry.gitlab.com
    auth:
      type: bearer
      secret: ${GITLAB_TOKEN}

scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}
  max_concurrent: 8

queue:
  buffer_size: 300
  workers: 8
```

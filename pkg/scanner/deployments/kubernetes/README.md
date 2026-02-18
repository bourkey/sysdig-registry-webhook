# Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the Registry Webhook Scanner.

## Quick Start

### 1. Create Namespace

```bash
kubectl create namespace webhook-scanner
```

### 2. Configure Secrets

Edit `03-secret.yaml` with your actual credentials:

```yaml
stringData:
  SYSDIG_API_TOKEN: "your-actual-sysdig-token"
  SYSDIG_PROJECT_ID: "your-actual-project-id"  # Required if using Registry Scanner
  # ... other secrets
```

Then apply:

```bash
kubectl apply -f 03-secret.yaml
```

### 3. Configure Scanner Type

The scanner type is configured in `02-configmap.yaml`. Choose between two options:

#### Option A: CLI Scanner (Default)

**Use when:**
- You have local storage available
- You prefer self-contained scanning
- Network connectivity to Sysdig API is unreliable

**Configuration:**

```yaml
scanner:
  type: cli
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  max_concurrent: 5
```

No additional configuration needed. The CLI scanner binary is embedded in the container image.

#### Option B: Registry Scanner

**Use when:**
- You want to avoid pulling images locally (saves bandwidth and storage)
- You want faster scan initiation
- You're scanning very large images
- You have reliable network connectivity to Sysdig API

**Configuration:**

1. Update scanner type in `02-configmap.yaml`:

```yaml
scanner:
  type: registry  # Change from "cli" to "registry"
  sysdig_token: ${SYSDIG_API_TOKEN}
  default_timeout: 300s
  max_concurrent: 5
  
  registry_scanner:  # Uncomment and configure
    api_url: https://secure.sysdig.com  # Or EU: https://eu1.app.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s
```

2. Ensure `SYSDIG_PROJECT_ID` is set in secrets (step 2 above)

3. Verify network connectivity to Sysdig API from your cluster

#### Option C: Mixed Scanner Types (Per-Registry Override)

You can use different scanner types for different registries:

```yaml
scanner:
  type: cli  # Global default
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  max_concurrent: 5
  
  # Registry Scanner config (for registries that override)
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s

registries:
  - name: dockerhub
    type: dockerhub
    # Uses global default (cli)
    auth:
      type: hmac
      secret: ${DOCKERHUB_WEBHOOK_SECRET}
  
  - name: gitlab-registry
    type: gitlab
    scanner:
      type: registry  # Override: use Registry Scanner for GitLab
    auth:
      type: bearer
      secret: ${GITLAB_WEBHOOK_TOKEN}
```

### 4. Apply ConfigMap

```bash
kubectl apply -f 02-configmap.yaml
```

### 5. Deploy Application

```bash
kubectl apply -f 01-deployment.yaml
kubectl apply -f 04-service.yaml
```

### 6. Verify Deployment

```bash
# Check pod status
kubectl get pods -n webhook-scanner

# Check logs
kubectl logs -f deployment/webhook-scanner -n webhook-scanner

# Check health endpoint
kubectl port-forward -n webhook-scanner svc/webhook-scanner 8080:8080
curl http://localhost:8080/health
# Expected: {"status": "healthy"}
```

---

## Configuration Details

### Scanner Type Selection

The scanner type is determined by this priority:

1. **Per-registry override** (`registries[].scanner.type`)
2. **Global default** (`scanner.type`)
3. **Fallback** to "cli" (backward compatibility)

### CLI Scanner Requirements

**Container Image:** Must include Sysdig CLI Scanner binary

**Storage:** Requires ephemeral or persistent storage for image downloads

**Resources:**
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
```

**Volume Mounts:**
```yaml
volumeMounts:
  - name: scanner-cache
    mountPath: /var/lib/scanner
volumes:
  - name: scanner-cache
    emptyDir: {}
```

### Registry Scanner Requirements

**Network:** Requires outbound HTTPS (443) to Sysdig API

**No storage required** - images are not pulled locally

**Minimal resources:**
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

**API Endpoints:**
- US: `https://secure.sysdig.com` or `https://us2.app.sysdig.com`
- EU: `https://eu1.app.sysdig.com`

### Environment Variables

All environment variables are loaded from the Secret:

| Variable | Required | Scanner Type | Description |
|----------|----------|--------------|-------------|
| `SYSDIG_API_TOKEN` | Yes | Both | Sysdig API authentication token |
| `SYSDIG_PROJECT_ID` | Conditional | Registry | Required when using Registry Scanner |
| `DOCKERHUB_WEBHOOK_SECRET` | Optional | Both | Docker Hub webhook HMAC secret |
| `HARBOR_WEBHOOK_TOKEN` | Optional | Both | Harbor webhook bearer token |
| `GITLAB_WEBHOOK_TOKEN` | Optional | Both | GitLab webhook bearer token |
| `HARBOR_USERNAME` | Optional | CLI | Registry credentials for private images |
| `HARBOR_PASSWORD` | Optional | CLI | Registry credentials for private images |

---

## Deployment Scenarios

### Scenario 1: All CLI Scanner

**Use case:** Air-gapped or restricted network environment

**Configuration:**
```yaml
scanner:
  type: cli
  cli_path: /usr/local/bin/sysdig-cli-scanner
```

**Network requirements:** None (to Sysdig API during scan - CLI handles it)

### Scenario 2: All Registry Scanner

**Use case:** Scanning very large images, minimizing bandwidth

**Configuration:**
```yaml
scanner:
  type: registry
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
```

**Network requirements:** Outbound HTTPS to `secure.sysdig.com:443`

### Scenario 3: Mixed (CLI for Public, Registry for Private)

**Use case:** Optimize based on registry characteristics

**Configuration:**
```yaml
scanner:
  type: cli  # Default for public registries
  
  registry_scanner:  # Available for overrides
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}

registries:
  - name: dockerhub
    type: dockerhub
    # Uses CLI (public, fast to pull)
  
  - name: internal-harbor
    type: harbor
    scanner:
      type: registry  # Use Registry Scanner (private, large images)
```

---

## Troubleshooting

### CLI Scanner Issues

**Problem:** `Sysdig CLI scanner not found`

**Solution:** Verify CLI binary is in container image:
```bash
kubectl exec -it deployment/webhook-scanner -n webhook-scanner -- which sysdig-cli-scanner
```

**Problem:** `failed to pull image: authentication required`

**Solution:** Add registry credentials to ConfigMap:
```yaml
registries:
  - name: my-registry
    scanner:
      credentials:
        username: ${REGISTRY_USERNAME}
        password: ${REGISTRY_PASSWORD}
```

### Registry Scanner Issues

**Problem:** `failed to send request: dial tcp: lookup secure.sysdig.com`

**Solution:** Verify network connectivity:
```bash
kubectl exec -it deployment/webhook-scanner -n webhook-scanner -- curl -I https://secure.sysdig.com
```

**Problem:** `API returned status 401: Unauthorized`

**Solution:** Verify `SYSDIG_API_TOKEN` is correct in Secret:
```bash
kubectl get secret webhook-scanner-secrets -n webhook-scanner -o yaml
# Check SYSDIG_API_TOKEN value (base64 encoded)
```

**Problem:** `scanner.registry_scanner.project_id is required`

**Solution:** Add `SYSDIG_PROJECT_ID` to Secret and update ConfigMap:
```yaml
# In Secret:
SYSDIG_PROJECT_ID: "your-project-id"

# In ConfigMap:
scanner:
  type: registry
  registry_scanner:
    project_id: ${SYSDIG_PROJECT_ID}
```

### General Debugging

**Enable debug logging:**
```yaml
# In ConfigMap
log_level: debug
```

**Check pod logs:**
```bash
kubectl logs -f deployment/webhook-scanner -n webhook-scanner

# Look for scanner_type field in logs:
# {"level":"info","scanner_type":"cli","message":"Scan completed"}
# {"level":"info","scanner_type":"registry","message":"Scan completed"}
```

**Check scanner configuration:**
```bash
kubectl exec -it deployment/webhook-scanner -n webhook-scanner -- cat /config/config.yaml
```

---

## Upgrading

### From CLI-only to Registry Scanner

1. Update ConfigMap with Registry Scanner configuration:
```bash
kubectl edit configmap webhook-scanner-config -n webhook-scanner
```

2. Add `SYSDIG_PROJECT_ID` to Secret:
```bash
kubectl edit secret webhook-scanner-secrets -n webhook-scanner
```

3. Restart deployment to pick up changes:
```bash
kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
```

4. Verify scanner type in logs:
```bash
kubectl logs -f deployment/webhook-scanner -n webhook-scanner | grep scanner_type
```

### Rolling Back

To revert to CLI Scanner:

1. Change `scanner.type` back to `cli` in ConfigMap
2. Restart deployment: `kubectl rollout restart deployment/webhook-scanner -n webhook-scanner`

---

## Security Considerations

### CLI Scanner

- **Image pull credentials:** Stored in ConfigMap (consider using imagePullSecrets instead)
- **Local storage:** Ephemeral volumes cleared on pod restart
- **Binary integrity:** Verify CLI scanner binary checksum in container build

### Registry Scanner

- **API token:** Requires broad Sysdig API access (use least-privilege tokens)
- **Network exposure:** Outbound HTTPS to Sysdig API (review firewall rules)
- **TLS verification:** Always use `verify_tls: true` in production

### General

- **Webhook secrets:** Use strong random values (minimum 32 characters)
- **Secret management:** Consider external-secrets operator or sealed-secrets
- **RBAC:** Restrict ServiceAccount permissions to minimum required
- **Network policies:** Limit ingress/egress to required ports and destinations

---

## Monitoring and Observability

### Logs

All logs include structured fields:

```json
{
  "level": "info",
  "scanner_type": "registry",
  "image_ref": "myregistry/myimage:v1.0.0",
  "request_id": "abc-123",
  "duration_ms": 45000,
  "message": "Scan completed successfully"
}
```

**Key fields:**
- `scanner_type`: "cli" or "registry"
- `duration_ms`: Scan duration for performance monitoring
- `request_id`: Trace requests through the system

### Metrics (Future)

Planned Prometheus metrics:

- `webhook_scanner_scans_total{scanner_type, status}` - Total scans by type and status
- `webhook_scanner_scan_duration_seconds{scanner_type}` - Scan duration histogram
- `webhook_scanner_api_requests_total{scanner_type, status_code}` - API requests (Registry Scanner)
- `webhook_scanner_queue_length` - Current queue depth

---

## Advanced Configuration

### Resource Tuning

**For CLI Scanner (more resource-intensive):**
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2000m"

scanner:
  max_concurrent: 3  # Limit concurrent scans
```

**For Registry Scanner (lightweight):**
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

scanner:
  max_concurrent: 10  # Higher concurrency possible
```

### Horizontal Pod Autoscaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: webhook-scanner-hpa
  namespace: webhook-scanner
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: webhook-scanner
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Network Policies

**For Registry Scanner (requires egress to Sysdig API):**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: webhook-scanner-netpol
  namespace: webhook-scanner
spec:
  podSelector:
    matchLabels:
      app: webhook-scanner
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS to Sysdig API
```

---

## Getting Help

For issues or questions:

1. Check logs: `kubectl logs -f deployment/webhook-scanner -n webhook-scanner`
2. Review configuration: `kubectl get configmap webhook-scanner-config -o yaml`
3. Check [TROUBLESHOOTING.md](../../docs/TROUBLESHOOTING.md)
4. Open GitHub issue with logs and configuration (redact secrets)

# Troubleshooting Guide

Comprehensive troubleshooting guide for the Registry Webhook Scanner.

## Table of Contents

- [Common Issues](#common-issues)
  - [Webhook Delivery](#webhook-delivery)
  - [Authentication](#authentication)
  - [Scanning](#scanning)
  - [Performance](#performance)
  - [Configuration](#configuration)
- [Log Interpretation](#log-interpretation)
- [Debugging Procedures](#debugging-procedures)
- [Health Check Diagnostics](#health-check-diagnostics)
- [Queue and Worker Issues](#queue-and-worker-issues)
- [Network Connectivity](#network-connectivity)
- [Getting Help](#getting-help)

## Common Issues

### Webhook Delivery

#### Webhooks Not Received

**Symptoms:**
- No logs showing webhook receipt
- Registry shows webhook delivery failures
- Test webhooks from registry fail

**Diagnostic Steps:**

1. **Verify service is running and accessible:**
   ```bash
   # Kubernetes
   kubectl get pods -n webhook-scanner
   kubectl logs -f deployment/webhook-scanner -n webhook-scanner

   # Docker
   docker ps | grep webhook-scanner
   docker logs -f webhook-scanner
   ```

2. **Test endpoint accessibility:**
   ```bash
   # From outside the cluster/network
   curl -v https://your-webhook-endpoint.com/health

   # Expected: 200 OK with {"status":"healthy"}
   ```

3. **Check ingress/load balancer configuration:**
   ```bash
   # Kubernetes
   kubectl get ingress -n webhook-scanner
   kubectl describe ingress webhook-scanner -n webhook-scanner

   # Look for: Address field populated, backend service healthy
   ```

4. **Verify DNS resolution:**
   ```bash
   nslookup your-webhook-endpoint.com
   dig your-webhook-endpoint.com
   ```

5. **Test with manual webhook:**
   ```bash
   curl -X POST https://your-webhook-endpoint.com/webhook \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer test-token" \
     -d @test/fixtures/harbor-webhook.json
   ```

**Common Solutions:**

| Problem | Solution |
|---------|----------|
| Service not accessible from internet | Check firewall rules, security groups, network policies |
| Ingress not routing traffic | Verify ingress controller is running, check ingress annotations |
| DNS not resolving | Update DNS records, wait for propagation |
| Port not exposed | Check Service type (LoadBalancer/NodePort), verify port mappings |
| TLS certificate issues | Check cert-manager, verify certificate validity |

#### Webhook Timeouts

**Symptoms:**
- Registry shows timeout errors in webhook delivery logs
- Webhooks eventually succeed but take too long
- HTTP 504 Gateway Timeout responses

**Diagnostic Steps:**

1. **Check queue depth:**
   ```bash
   # Look for queue full messages in logs
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "queue full"
   ```

2. **Check worker pool status:**
   ```bash
   # Look for worker availability
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "workers"
   ```

3. **Review scan durations:**
   ```bash
   # Look for long-running scans
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "scan completed" | grep "duration="
   ```

**Solutions:**

1. **Increase queue buffer size** in config:
   ```yaml
   queue:
     buffer_size: 200  # Increase from default 100
     workers: 5        # Increase concurrent workers
   ```

2. **Adjust ingress timeout:**
   ```yaml
   # Kubernetes Ingress annotation
   nginx.ingress.kubernetes.io/proxy-read-timeout: "60"
   nginx.ingress.kubernetes.io/proxy-send-timeout: "60"
   ```

3. **Optimize scan timeouts:**
   ```yaml
   scanner:
     default_timeout: 600s  # Increase if scans timeout frequently
     max_concurrent: 3      # Reduce if too many concurrent scans
   ```

### Authentication

#### 401 Unauthorized Errors

**Symptoms:**
- All webhooks return 401 Unauthorized
- Registry shows authentication failed in delivery logs
- Logs show "authentication failed" messages

**Diagnostic Steps:**

1. **Verify authentication type matches:**
   ```bash
   # Check config
   kubectl get configmap scanner-config -n webhook-scanner -o yaml

   # Look for: auth.type should be "hmac" or "bearer"
   ```

2. **Check secret values:**
   ```bash
   # Kubernetes - decode secret
   kubectl get secret scanner-secrets -n webhook-scanner -o jsonpath='{.data.harbor-webhook-token}' | base64 -d

   # Docker - check environment variables
   docker exec webhook-scanner env | grep TOKEN
   ```

3. **Enable debug logging:**
   ```bash
   # Set LOG_LEVEL=debug to see authentication details
   kubectl set env deployment/webhook-scanner LOG_LEVEL=debug -n webhook-scanner
   ```

4. **Check authentication headers:**
   ```bash
   # Test with curl showing headers
   curl -v -X POST https://your-endpoint/webhook \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -d @test/fixtures/harbor-webhook.json
   ```

**Common Solutions:**

| Auth Type | Issue | Solution |
|-----------|-------|----------|
| HMAC | Signature mismatch | Verify secret matches registry configuration exactly |
| HMAC | Header not found | Check for X-Hub-Signature-256 or X-Signature header |
| HMAC | Algorithm mismatch | Ensure HMAC-SHA256 is used (not SHA1) |
| Bearer | Token mismatch | Verify token string matches (no extra spaces) |
| Bearer | Missing Authorization header | Check registry sends "Authorization: Bearer <token>" |
| Bearer | Case sensitivity | Ensure "Bearer" is capitalized correctly |

**Fix Mismatched Secrets:**

```bash
# Kubernetes - update secret
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=harbor-webhook-token='CORRECT_TOKEN_HERE' \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart deployment
kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
```

#### Intermittent Authentication Failures

**Symptoms:**
- Some webhooks succeed, others fail with 401
- Authentication works initially but fails later
- Different registries have different success rates

**Possible Causes:**

1. **Multiple registries with different auth configs** - Check each registry configuration
2. **Token rotation** - Registry tokens changed but config not updated
3. **Request body consumption** - HMAC verification consumes body, must be restored

**Solution:**

1. Review config for all registries:
   ```yaml
   registries:
     - name: harbor-prod
       auth:
         type: bearer
         token: ${HARBOR_TOKEN}    # Verify this matches registry

     - name: dockerhub
       auth:
         type: hmac
         secret: ${DOCKERHUB_SECRET}  # Verify this matches registry
   ```

2. Test each registry individually with manual webhooks

### Scanning

#### Scans Not Triggering

**Symptoms:**
- Webhooks received (200 OK) but scans don't start
- No scan logs after webhook receipt
- Queue appears empty but scans not processing

**Diagnostic Steps:**

1. **Check worker pool started:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "worker pool"
   # Expected: "worker pool started" with worker count
   ```

2. **Verify Sysdig CLI binary exists:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- ls -l /usr/local/bin/sysdig-cli-scanner
   # Expected: Binary exists with execute permissions
   ```

3. **Check SYSDIG_API_TOKEN is set:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- env | grep SYSDIG
   # Expected: SYSDIG_API_TOKEN=<your-token>
   ```

4. **Review queue enqueue logs:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "enqueued"
   ```

**Solutions:**

1. **Verify Sysdig token:**
   ```bash
   # Test Sysdig CLI manually
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- \
     /usr/local/bin/sysdig-cli-scanner nginx:latest
   ```

2. **Check scanner configuration:**
   ```yaml
   scanner:
     sysdig_token: ${SYSDIG_API_TOKEN}  # Must be set
     default_timeout: 300s
     max_concurrent: 5
   ```

#### Scan Timeouts

**Symptoms:**
- Logs show "scan timeout exceeded"
- Large images never complete
- Scans work for small images but fail for large ones

**Diagnostic Steps:**

1. **Review timeout configuration:**
   ```bash
   kubectl get configmap scanner-config -n webhook-scanner -o yaml | grep timeout
   ```

2. **Check scan duration patterns:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | \
     grep "scan completed" | awk '{print $NF}' | sort -n
   ```

3. **Verify network connectivity to Sysdig:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- \
     curl -I https://secure.sysdig.com
   ```

**Solutions:**

1. **Increase timeout globally:**
   ```yaml
   scanner:
     default_timeout: 600s  # 10 minutes instead of 5
   ```

2. **Set per-registry timeout for large images:**
   ```yaml
   registries:
     - name: harbor-prod
       scanner:
         timeout: 900s  # 15 minutes for this registry
   ```

3. **Check Sysdig backend status:**
   - Visit https://status.sysdig.com
   - Test API connectivity: `curl -H "Authorization: Bearer $TOKEN" https://secure.sysdig.com/api/scanning/v1/health`

#### Scan Failures

**Symptoms:**
- Scans fail with error messages
- Exit codes indicate failure
- Sysdig CLI returns errors

**Common Error Messages:**

| Error | Cause | Solution |
|-------|-------|----------|
| "image not found" | Image doesn't exist or was deleted | Verify image exists in registry |
| "authentication failed" | Registry credentials invalid | Check scanner.credentials in config |
| "unauthorized" | Sysdig token invalid | Verify SYSDIG_API_TOKEN is correct |
| "network timeout" | Can't reach Sysdig backend | Check network connectivity, proxies |
| "rate limit exceeded" | Too many requests to Sysdig | Reduce max_concurrent, implement backoff |

**Diagnostic Steps:**

1. **Check specific error:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "scan failed"
   ```

2. **Test image pull manually:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- \
     /usr/local/bin/sysdig-cli-scanner <image-ref>
   ```

3. **Verify registry credentials:**
   ```yaml
   registries:
     - name: harbor-prod
       scanner:
         credentials:
           username: ${HARBOR_USERNAME}
           password: ${HARBOR_PASSWORD}
   ```

### Performance

#### High Memory Usage

**Symptoms:**
- Pods OOMKilled
- High memory consumption
- Service becomes unresponsive

**Diagnostic Steps:**

1. **Check current memory usage:**
   ```bash
   kubectl top pods -n webhook-scanner
   ```

2. **Review memory limits:**
   ```bash
   kubectl get deployment webhook-scanner -n webhook-scanner -o yaml | grep -A 5 resources
   ```

3. **Check concurrent scan count:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | \
     grep "scan started" | tail -20
   ```

**Solutions:**

1. **Reduce concurrent scans:**
   ```yaml
   scanner:
     max_concurrent: 3  # Reduce from 5
   ```

2. **Reduce queue buffer:**
   ```yaml
   queue:
     buffer_size: 50  # Reduce from 100
   ```

3. **Increase memory limits:**
   ```yaml
   resources:
     limits:
       memory: "1Gi"  # Increase from 512Mi
   ```

4. **Enable scan result caching** (reduces repeated scans):
   ```yaml
   scanner:
     cache_ttl: 3600  # Cache results for 1 hour
   ```

#### High CPU Usage

**Symptoms:**
- CPU throttling
- Slow scan processing
- High CPU metrics

**Solutions:**

1. **Increase CPU limits:**
   ```yaml
   resources:
     requests:
       cpu: "500m"
     limits:
       cpu: "1000m"
   ```

2. **Scale horizontally:**
   ```bash
   kubectl scale deployment webhook-scanner --replicas=3 -n webhook-scanner
   ```

3. **Reduce worker count per pod:**
   ```yaml
   queue:
     workers: 2  # Reduce from 3
   ```

### Configuration

#### Configuration Not Loading

**Symptoms:**
- Service fails to start
- Logs show "failed to load configuration"
- Default values used instead of config file

**Diagnostic Steps:**

1. **Verify config file exists:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- cat /config/config.yaml
   ```

2. **Check CONFIG_FILE environment variable:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- env | grep CONFIG_FILE
   ```

3. **Validate YAML syntax:**
   ```bash
   # Locally
   yamllint config.yaml

   # Or
   python -c "import yaml; yaml.safe_load(open('config.yaml'))"
   ```

**Common Issues:**

| Problem | Solution |
|---------|----------|
| File not found | Check volume mount path matches CONFIG_FILE |
| YAML syntax error | Use YAML validator, check indentation |
| Environment variable not expanded | Ensure variables are set before service starts |
| Secrets not loaded | Verify secret files exist at /var/secrets/ |

#### Environment Variable Expansion Fails

**Symptoms:**
- Config contains literal `${VAR}` instead of values
- Errors about missing tokens/secrets
- Authentication uses literal strings

**Solutions:**

1. **Verify environment variables are set:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- env
   ```

2. **Check secret mounting:**
   ```bash
   kubectl exec -n webhook-scanner deployment/webhook-scanner -- ls -la /var/secrets/
   ```

3. **Use ${FILE:secret-name} syntax for secrets:**
   ```yaml
   auth:
     token: ${FILE:harbor-webhook-token}
   ```

## Log Interpretation

### Log Levels

The service uses structured JSON logging with these levels:

- **debug**: Detailed execution flow (enable with LOG_LEVEL=debug)
- **info**: Normal operations (webhooks received, scans completed)
- **warn**: Non-fatal issues (retries, timeouts)
- **error**: Fatal errors requiring attention

### Key Log Messages

#### Successful Webhook Flow

```json
{"level":"info","msg":"webhook received","registry":"harbor-prod","event_id":"abc-123"}
{"level":"info","msg":"image extracted","image":"harbor.com/app:v1.0.0"}
{"level":"info","msg":"scan request enqueued","image":"harbor.com/app:v1.0.0","queue_depth":3}
{"level":"info","msg":"scan started","image":"harbor.com/app:v1.0.0","worker_id":2}
{"level":"info","msg":"scan completed successfully","image":"harbor.com/app:v1.0.0","duration":45.2}
```

#### Authentication Failure

```json
{"level":"warn","msg":"authentication failed","registry":"harbor-prod","reason":"invalid bearer token"}
```

**Action**: Verify token matches registry configuration

#### Queue Full

```json
{"level":"warn","msg":"queue full, scan request rejected","queue_depth":100,"buffer_size":100}
```

**Action**: Increase buffer_size or add more workers

#### Scan Timeout

```json
{"level":"warn","msg":"scan timeout exceeded","image":"large-app:latest","timeout":"300s"}
```

**Action**: Increase scanner timeout for this registry/image

#### Scanner Binary Not Found

```json
{"level":"error","msg":"scanner binary not found","path":"/usr/local/bin/sysdig-cli-scanner"}
```

**Action**: Verify Docker image includes Sysdig CLI

### Log Analysis Commands

**Count webhook receipts by registry:**
```bash
kubectl logs -n webhook-scanner deployment/webhook-scanner | \
  grep "webhook received" | jq -r '.registry' | sort | uniq -c
```

**Average scan duration:**
```bash
kubectl logs -n webhook-scanner deployment/webhook-scanner | \
  grep "scan completed" | jq -r '.duration' | \
  awk '{sum+=$1; count++} END {print sum/count}'
```

**Authentication failure rate:**
```bash
kubectl logs -n webhook-scanner deployment/webhook-scanner | \
  grep "authentication" | \
  jq -s 'group_by(.msg) | map({msg: .[0].msg, count: length})'
```

## Debugging Procedures

### Enable Debug Logging

**Kubernetes:**
```bash
kubectl set env deployment/webhook-scanner LOG_LEVEL=debug -n webhook-scanner
kubectl logs -f deployment/webhook-scanner -n webhook-scanner
```

**Docker:**
```bash
docker exec -it webhook-scanner sh -c 'export LOG_LEVEL=debug && kill -HUP 1'
```

### Trace a Specific Webhook

1. **Send webhook with known payload:**
   ```bash
   curl -X POST https://your-endpoint/webhook \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer test-token" \
     -d @test/fixtures/harbor-webhook.json | jq .event_id
   ```

2. **Filter logs by event_id:**
   ```bash
   kubectl logs -n webhook-scanner deployment/webhook-scanner | grep "abc-123"
   ```

### Test Scanner Directly

```bash
# Exec into pod
kubectl exec -it -n webhook-scanner deployment/webhook-scanner -- sh

# Test Sysdig CLI
export SYSDIG_API_TOKEN=your-token
/usr/local/bin/sysdig-cli-scanner nginx:latest --apiurl https://secure.sysdig.com
```

## Health Check Diagnostics

### Health Endpoint Returns Unhealthy

**Check logs for startup errors:**
```bash
kubectl logs -n webhook-scanner deployment/webhook-scanner | grep -i error
```

**Common causes:**
- Configuration validation failed
- Required environment variables missing
- Cannot connect to dependencies

### Readiness Probe Failures

**Symptoms:**
- Pods not marked as Ready
- No traffic routed to pod

**Check:**
```bash
kubectl describe pod -n webhook-scanner <pod-name> | grep -A 10 Readiness
```

**Fix:**
- Verify /ready endpoint returns 200
- Check if worker pool started
- Ensure queue initialized

## Queue and Worker Issues

### Queue Depth Always High

**Symptoms:**
- Queue never drains
- New webhooks rejected
- Workers not processing

**Diagnostic:**
```bash
kubectl logs -n webhook-scanner deployment/webhook-scanner | \
  grep "queue_depth" | tail -20
```

**Solutions:**
1. Increase worker count
2. Reduce scan timeout to fail faster
3. Check if scans are hanging
4. Scale deployment horizontally

### Workers Idle But Queue Full

**Possible Causes:**
- Worker panic/crash (check logs for panics)
- Deadlock in scan processing
- Workers waiting on external dependency

**Solution:**
```bash
# Restart deployment
kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
```

## Network Connectivity

### Cannot Reach Registry

**Test connectivity:**
```bash
kubectl exec -n webhook-scanner deployment/webhook-scanner -- \
  curl -I https://harbor.example.com
```

**Solutions:**
- Add network policies to allow egress to registry
- Configure proxy if required
- Verify DNS resolution

### Cannot Reach Sysdig Backend

**Test connectivity:**
```bash
kubectl exec -n webhook-scanner deployment/webhook-scanner -- \
  curl -I https://secure.sysdig.com
```

**Check proxy configuration:**
```bash
kubectl exec -n webhook-scanner deployment/webhook-scanner -- env | grep -i proxy
```

## Getting Help

### Collect Diagnostic Bundle

```bash
#!/bin/bash
# Kubernetes diagnostic bundle
kubectl get pods -n webhook-scanner > diagnostics/pods.txt
kubectl get deployment -n webhook-scanner -o yaml > diagnostics/deployment.yaml
kubectl get configmap scanner-config -n webhook-scanner -o yaml > diagnostics/config.yaml
kubectl logs -n webhook-scanner deployment/webhook-scanner --tail=1000 > diagnostics/logs.txt
kubectl describe pod -n webhook-scanner > diagnostics/pod-details.txt
kubectl top pods -n webhook-scanner > diagnostics/metrics.txt
```

### Report Issues

When reporting issues, include:

1. **Environment Details:**
   - Kubernetes version or Docker version
   - Scanner service version/image tag
   - Registry type (Harbor, Docker Hub, etc.)

2. **Configuration** (sanitized, no secrets):
   - config.yaml (remove tokens/passwords)
   - Environment variables (LOG_LEVEL, PORT, etc.)

3. **Logs:**
   - Last 100 lines of service logs
   - Specific error messages
   - Timestamps of failures

4. **Symptoms:**
   - What you expected to happen
   - What actually happened
   - Frequency (always, intermittent, specific images)

5. **Attempts:**
   - Troubleshooting steps already tried
   - Any workarounds found

### Support Resources

- **GitHub Issues**: https://github.com/yourusername/registry-webhook-scanner/issues
- **Documentation**: See [docs/](../) directory
- **Configuration Reference**: [CONFIGURATION.md](CONFIGURATION.md)
- **Registry Setup**: [REGISTRY_SETUP.md](REGISTRY_SETUP.md)
- **Sysdig Support**: https://docs.sysdig.com/en/docs/sysdig-secure/scanning/

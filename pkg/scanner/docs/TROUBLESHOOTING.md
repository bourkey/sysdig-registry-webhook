# Troubleshooting Guide

This guide covers common issues and solutions for the Registry Webhook Scanner.

## Table of Contents
- [Scanner Issues](#scanner-issues)
  - [CLI Scanner](#cli-scanner-issues)
  - [Registry Scanner](#registry-scanner-issues)
- [Webhook Issues](#webhook-issues)
- [Configuration Issues](#configuration-issues)
- [Performance Issues](#performance-issues)

---

## Scanner Issues

### CLI Scanner Issues

#### CLI Scanner Binary Not Found

**Symptom:** Error message: `Sysdig CLI scanner not found at /path/to/scanner`

**Solutions:**
1. Verify the CLI scanner is installed:
   ```bash
   which sysdig-cli-scanner
   ```
2. Update `scanner.cli_path` in `config.yaml` to match the actual binary location
3. Ensure the binary has execute permissions:
   ```bash
   chmod +x /usr/local/bin/sysdig-cli-scanner
   ```

#### CLI Scanner Timeout

**Symptom:** Scans fail with timeout errors after a long wait

**Solutions:**
1. Increase the timeout in `config.yaml`:
   ```yaml
   scanner:
     default_timeout: 600s  # Increase from 300s
   ```
2. For specific registries, set per-registry timeout:
   ```yaml
   registries:
     - name: my-registry
       scanner:
         timeout: 900s
   ```
3. Check if the image is very large - consider using Registry Scanner instead

#### CLI Scanner Authentication Failures

**Symptom:** Error: `failed to pull image: authentication required`

**Solutions:**
1. Add registry credentials to the configuration:
   ```yaml
   registries:
     - name: my-registry
       scanner:
         credentials:
           username: \${REGISTRY_USERNAME}
           password: \${REGISTRY_PASSWORD}
   ```
2. Verify credentials are correct
3. For private registries, ensure network connectivity

---

### Registry Scanner Issues

#### Registry Scanner API Connection Failures

**Symptom:** Error: `failed to send request: dial tcp: lookup secure.sysdig.com: no such host`

**Solutions:**
1. Verify network connectivity to the Sysdig API:
   ```bash
   curl -I https://secure.sysdig.com
   ```
2. Check if a proxy is required and configure accordingly
3. Verify DNS resolution is working
4. Check firewall rules allow outbound HTTPS (443)

#### Registry Scanner Authentication Failures

**Symptom:** Error: `API returned status 401: Unauthorized` or `API returned status 403: Forbidden`

**Solutions:**
1. Verify the Sysdig API token is correct:
   ```yaml
   scanner:
     sysdig_token: \${SYSDIG_API_TOKEN}
   ```
2. Check token has not expired
3. Verify token has required permissions in Sysdig Secure
4. Try generating a new API token from Sysdig UI

#### Registry Scanner Polling Timeouts

**Symptom:** Error: `scan timeout after N poll attempts`

**Solutions:**
1. Increase the polling timeout for slow scans:
   ```yaml
   scanner:
     default_timeout: 900s  # Increase from 300s
   ```
2. Adjust poll interval (default 5s):
   ```yaml
   scanner:
     registry_scanner:
       poll_interval: 10s  # Poll less frequently
   ```
3. Check Sysdig API status - may be experiencing delays
4. For very large images, consider using CLI Scanner instead

#### Registry Scanner Project ID Issues

**Symptom:** Error: `scanner.registry_scanner.project_id is required when scanner.type is 'registry'`

**Solutions:**
1. Add project ID to configuration:
   ```yaml
   scanner:
     type: registry
     registry_scanner:
       project_id: \${SYSDIG_PROJECT_ID}
   ```
2. Get project ID from Sysdig UI:
   - Go to Settings → Projects
   - Copy the project ID for your target project

#### Registry Scanner Rate Limiting

**Symptom:** Warning logs: `Rate limited by API` with 429 status codes

**Solutions:**
1. The scanner automatically handles rate limiting with exponential backoff
2. Monitor logs for `retry_after` duration
3. If persistent, reduce concurrent scanning:
   ```yaml
   scanner:
     max_concurrent: 2  # Reduce from 5
   ```
4. Consider upgrading Sysdig plan for higher rate limits

#### Registry Scanner TLS Verification Failures

**Symptom:** Error: `x509: certificate signed by unknown authority`

**Solutions:**
1. **Recommended:** Fix certificate chain or use valid certificates
2. **For testing only:** Disable TLS verification:
   ```yaml
   scanner:
     registry_scanner:
       verify_tls: false  # NOT recommended for production
   ```

#### Registry Scanner API Response Parsing Errors

**Symptom:** Error: `failed to parse scan response` or `failed to decode response`

**Solutions:**
1. Check API URL is correct:
   ```yaml
   scanner:
     registry_scanner:
       api_url: https://secure.sysdig.com  # Verify region
   ```
2. For EU regions, use: `https://eu1.app.sysdig.com`
3. For US regions, use: `https://secure.sysdig.com` or `https://us2.app.sysdig.com`
4. Check Sysdig API version compatibility

---

## Webhook Issues

### Webhook Authentication Failures

**Symptom:** Webhook requests return 401 Unauthorized

**Solutions:**
1. For HMAC authentication:
   ```yaml
   registries:
     - name: my-registry
       auth:
         type: hmac
         secret: \${WEBHOOK_SECRET}
   ```
2. For Bearer token authentication:
   ```yaml
   registries:
     - name: my-registry
       auth:
         type: bearer
         token: \${WEBHOOK_TOKEN}
   ```
3. Verify the secret/token matches what's configured in the registry

### Webhook Not Received

**Symptom:** Webhook events are not triggering scans

**Solutions:**
1. Check webhook configuration in registry UI
2. Verify webhook URL is correct: `http://your-server:8080/webhook`
3. Check server logs for incoming requests:
   ```bash
   kubectl logs -f deployment/webhook-scanner -n webhook-scanner
   ```
4. Verify network connectivity from registry to webhook server
5. Check firewall rules allow inbound HTTP/HTTPS

---

## Configuration Issues

### Scanner Type Selection

**Issue:** Unsure which scanner type to use

**Guidance:**

**Use CLI Scanner when:**
- You need to scan images not yet pushed to a registry
- You have local storage available for image downloads
- You prefer self-contained scanning without API dependencies
- Network connectivity to Sysdig API is unreliable

**Use Registry Scanner when:**
- You want to avoid pulling images locally (saves bandwidth and storage)
- You want faster scan initiation (no image download)
- You're scanning very large images
- You have reliable network connectivity to Sysdig API
- You prefer API-based scanning workflow

### Invalid Scanner Type

**Symptom:** Error: `unsupported scanner type: xyz`

**Solution:**
Use valid scanner types: `cli` or `registry`
```yaml
scanner:
  type: cli  # or "registry"
```

### Mixed Scanner Configuration

**Issue:** Want to use different scanner types for different registries

**Solution:**
Set per-registry scanner type overrides:
```yaml
scanner:
  type: cli  # Global default

registries:
  - name: public-registry
    # Uses global default (cli)

  - name: large-images-registry
    scanner:
      type: registry  # Override for this registry
```

---

## Performance Issues

### High Memory Usage

**Symptom:** Scanner pod using excessive memory

**Solutions:**
1. Reduce concurrent scans:
   ```yaml
   scanner:
     max_concurrent: 3  # Reduce from 5
   ```
2. Use Registry Scanner instead of CLI Scanner to avoid image downloads
3. Increase memory limits in Kubernetes deployment

### Slow Scan Performance

**Symptom:** Scans taking longer than expected

**Solutions:**
1. For CLI Scanner: Check local disk I/O performance
2. For Registry Scanner: Check network latency to Sysdig API
3. Consider switching scanner types based on bottleneck
4. Increase worker pool size:
   ```yaml
   queue:
     workers: 5  # Increase from 3
   ```

### Queue Backlog

**Symptom:** Event queue filling up, scans delayed

**Solutions:**
1. Increase queue buffer size:
   ```yaml
   queue:
     buffer_size: 200  # Increase from 100
   ```
2. Increase worker count:
   ```yaml
   queue:
     workers: 5  # Increase from 3
   ```
3. Increase concurrent scanner capacity:
   ```yaml
   scanner:
     max_concurrent: 10  # Increase from 5
   ```

---

## Debugging Tips

### Enable Debug Logging

Set log level to debug for more detailed logs:
```yaml
log_level: debug
```

### Check Scanner Type in Logs

All log entries include `scanner_type` field:
```json
{
  "level": "info",
  "scanner_type": "registry",
  "image_ref": "myregistry/myimage:latest",
  "message": "Scan completed successfully"
}
```

### Test Scanner Configuration

Validate your configuration before deploying:
1. Check YAML syntax is valid
2. Verify all required fields are present
3. Test API connectivity manually:
   ```bash
   curl -H "Authorization: Bearer \$SYSDIG_API_TOKEN" \\
     https://secure.sysdig.com/api/scanning/v1/registry/scan
   ```

### Common Log Messages

| Log Message | Meaning | Action |
|-------------|---------|--------|
| `Scanner backend created and validated` | Scanner initialized successfully | None needed |
| `Rate limited by API` | Hit Sysdig API rate limit | Reduce concurrent scans or wait |
| `Scan timeout during polling` | Registry Scanner poll timeout | Increase timeout or check API |
| `Retrying API request` | Temporary API failure | Monitor - automatic retry in progress |
| `failed to initiate scan` | Could not start scan | Check API credentials and connectivity |
| `TLS verification disabled` | Running without TLS verification | Fix certificates (security risk) |

---

## Getting Help

If issues persist after trying these solutions:

1. Check logs with debug level enabled
2. Verify configuration against `config.example.yaml`
3. Review Sysdig API status page
4. Open an issue on GitHub with:
   - Configuration (redacted secrets)
   - Error logs
   - Scanner type in use
   - Steps to reproduce

# Registry Setup Guide

How to configure webhooks in different container registries to work with the Registry Webhook Scanner.

## Quick Reference

| Registry | Webhook Support | Authentication | Configuration Time |
|----------|----------------|----------------|-------------------|
| Docker Hub | ✅ Yes | HMAC or None | 2 minutes |
| Harbor | ✅ Yes | Bearer Token | 3 minutes |
| GitLab Registry | ✅ Yes | Bearer Token | 3 minutes |
| AWS ECR | ❌ No native support | - | Use EventBridge |
| Google GCR | ❌ No native support | - | Use Pub/Sub |
| Azure ACR | ✅ Yes (webhooks) | Token | 3 minutes |

## Docker Hub

Docker Hub supports webhooks for repository push events.

### Prerequisites
- Docker Hub account
- Repository created
- Webhook scanner endpoint accessible from internet

### Setup Steps

1. **Navigate to Repository**
   - Go to https://hub.docker.com
   - Select your repository
   - Click on "Webhooks" tab

2. **Add Webhook**
   - Click "Create Webhook"
   - **Webhook name**: `sysdig-scanner`
   - **Webhook URL**: `https://your-webhook-scanner.example.com/webhook`

3. **Configure Authentication (Optional)**
   - Docker Hub doesn't provide built-in webhook secrets
   - Use IP allowlisting or deploy without authentication for public repos
   - For HMAC: Configure in Docker Hub webhook settings (if available)

4. **Test Webhook**
   ```bash
   docker push your-username/your-repo:test-tag
   ```

5. **Verify in Scanner Logs**
   ```bash
   kubectl logs -f deployment/webhook-scanner -n webhook-scanner | grep dockerhub
   ```

### Scanner Configuration

```yaml
registries:
  - name: dockerhub
    type: dockerhub
    auth:
      type: none  # Or hmac if configured
      # secret: ${DOCKERHUB_SECRET}
```

### Webhook Payload Example

```json
{
  "push_data": {
    "pushed_at": 1620000000,
    "tag": "v1.0.0",
    "pusher": "username"
  },
  "repository": {
    "repo_name": "username/repository",
    "namespace": "username",
    "name": "repository"
  }
}
```

## Harbor

Harbor provides comprehensive webhook support with multiple event types.

### Prerequisites
- Harbor v2.0+
- Project admin access
- Webhook scanner endpoint accessible

### Setup Steps

1. **Navigate to Project**
   - Log in to Harbor
   - Go to your project
   - Click "Webhooks" in left menu

2. **Create Webhook**
   - Click "+ NEW WEBHOOK"
   - **Name**: `Sysdig Scanner`
   - **Notify Type**: `http`
   - **Event Type**: Check `Artifact pushed`
   - **Endpoint URL**: `https://your-webhook-scanner.example.com/webhook`

3. **Configure Authentication**
   - **Auth Header**: Enter a secure token
   - Example: `Bearer abc123xyz789` (save this token)

4. **Test Webhook**
   - Click "TEST ENDPOINT"
   - Should see 200 OK response
   - Or push an image:
   ```bash
   docker tag myimage harbor.company.com/project/myapp:latest
   docker push harbor.company.com/project/myapp:latest
   ```

5. **Verify Webhook Deliveries**
   - Harbor UI → Project → Webhooks → View webhook → Executions
   - Check delivery status and response codes

### Scanner Configuration

```yaml
registries:
  - name: harbor-prod
    type: harbor
    url: https://harbor.company.com
    auth:
      type: bearer
      secret: ${HARBOR_WEBHOOK_TOKEN}  # The token you set in Harbor
    scanner:
      timeout: 600s
      credentials:  # For pulling private images
        username: ${HARBOR_USERNAME}
        password: ${HARBOR_PASSWORD}
```

### Webhook Payload Example

```json
{
  "type": "PUSH_ARTIFACT",
  "occur_at": 1620000000,
  "operator": "admin",
  "event_data": {
    "resources": [
      {
        "digest": "sha256:abc123...",
        "tag": "v1.0.0",
        "resource_url": "harbor.company.com/project/app:v1.0.0"
      }
    ],
    "repository": {
      "name": "project/app",
      "namespace": "project",
      "repo_full_name": "project/app"
    }
  }
}
```

## GitLab Container Registry

GitLab supports webhooks for push events including container registry pushes.

### Prerequisites
- GitLab instance (self-hosted or gitlab.com)
- Project with Container Registry enabled
- Maintainer or Owner role on project

### Setup Steps

1. **Navigate to Project Settings**
   - Go to your GitLab project
   - Settings → Webhooks

2. **Add Webhook**
   - **URL**: `https://your-webhook-scanner.example.com/webhook`
   - **Secret token**: Generate a secure token (save this)
   - **Trigger**: Check `Push events`
   - **SSL verification**: Enable (recommended)

3. **Test Webhook**
   - Click "Test" → "Push events"
   - Or push an image:
   ```bash
   docker login registry.gitlab.com
   docker tag myimage registry.gitlab.com/group/project/app:latest
   docker push registry.gitlab.com/group/project/app:latest
   ```

4. **View Webhook Logs**
   - Settings → Webhooks → Edit webhook
   - Scroll to "Recent Events"
   - Check Response code and headers

### Scanner Configuration

```yaml
registries:
  - name: gitlab-registry
    type: gitlab
    url: https://registry.gitlab.com  # Or your self-hosted URL
    auth:
      type: bearer
      secret: ${GITLAB_WEBHOOK_TOKEN}  # Token from webhook config
    scanner:
      timeout: 300s
      credentials:  # For private repositories
        username: ${GITLAB_USERNAME}
        password: ${GITLAB_TOKEN}  # Personal access token
```

### Notes

- GitLab webhooks are triggered on git push, not specifically on image push
- The scanner extracts image information from git tags/commits
- For better integration, configure webhooks at project level for Container Registry events

## AWS ECR (via EventBridge)

AWS ECR doesn't support webhooks directly, but can use EventBridge.

### Setup with EventBridge

1. **Create EventBridge Rule**
   ```bash
   aws events put-rule \
     --name ecr-image-push \
     --event-pattern '{
       "source": ["aws.ecr"],
       "detail-type": ["ECR Image Action"],
       "detail": {
         "action-type": ["PUSH"],
         "result": ["SUCCESS"]
       }
     }'
   ```

2. **Add Target (API Gateway → Scanner)**
   - Create API Gateway HTTP API
   - Point to webhook scanner endpoint
   - Add as EventBridge target

3. **Configure Scanner**
   ```yaml
   registries:
     - name: aws-ecr
       type: harbor  # Use harbor parser (similar format)
       url: ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
       auth:
         type: bearer  # From API Gateway auth
         secret: ${ECR_WEBHOOK_TOKEN}
   ```

## Troubleshooting Webhook Setup

### Webhook Not Triggering

**Symptoms:** No webhooks received after image push

**Checks:**
1. Verify webhook URL is correct and accessible
   ```bash
   curl -X POST https://your-webhook-scanner.example.com/webhook \
     -H "Content-Type: application/json" \
     -d '{"test":"data"}'
   ```

2. Check registry webhook delivery logs
   - Docker Hub: No delivery logs available
   - Harbor: Project → Webhooks → Executions
   - GitLab: Settings → Webhooks → Recent Events

3. Verify network connectivity
   - Scanner must be accessible from registry
   - Check firewall rules
   - Verify DNS resolution

4. Check scanner logs
   ```bash
   kubectl logs -f deployment/webhook-scanner -n webhook-scanner
   ```

### Authentication Failures

**Symptoms:** Webhooks received but return 401 Unauthorized

**Solutions:**
1. Verify token/secret matches in both registry and scanner config
2. Check authentication type (hmac vs bearer)
3. For HMAC: Verify signature algorithm matches
4. For Bearer: Verify token format (`Bearer <token>`)

### Webhook Timeouts

**Symptoms:** Registry shows timeout errors

**Solutions:**
1. Verify scanner is running and healthy
   ```bash
   curl https://your-webhook-scanner.example.com/health
   ```

2. Check queue is not full
   - Monitor queue depth metrics
   - Increase `queue.buffer_size` if needed

3. Verify ingress timeout settings
   ```yaml
   # Kubernetes Ingress annotation
   nginx.ingress.kubernetes.io/proxy-read-timeout: "30"
   ```

## Security Best Practices

1. **Use HTTPS** - Always use TLS for webhook endpoints
2. **Enable Authentication** - Use HMAC or bearer tokens
3. **Rotate Secrets** - Regularly rotate webhook tokens/secrets
4. **IP Allowlisting** - Restrict to registry IP ranges if possible
5. **Monitor Failed Authentications** - Alert on repeated failures
6. **Use Strong Tokens** - Generate cryptographically random tokens
   ```bash
   openssl rand -base64 32
   ```

## Testing Webhooks

### Manual Testing

```bash
# Test Docker Hub webhook
curl -X POST https://your-scanner.example.com/webhook \
  -H "Content-Type: application/json" \
  -d @test/fixtures/dockerhub-webhook.json

# Test Harbor webhook with authentication
curl -X POST https://your-scanner.example.com/webhook \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d @test/fixtures/harbor-webhook.json
```

### Verify Webhook Processing

1. Check scanner logs for webhook receipt
2. Verify scan was queued
3. Check scan completion
4. Verify results in Sysdig dashboard

```bash
# Follow logs
kubectl logs -f deployment/webhook-scanner -n webhook-scanner

# Look for:
# level=info msg="Webhook received" registry=harbor-prod
# level=info msg="Scan request enqueued" image_ref=harbor.com/app:v1
# level=info msg="Scan completed successfully" duration=45.2
```

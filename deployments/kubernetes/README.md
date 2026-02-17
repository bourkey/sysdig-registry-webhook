# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying the Registry Webhook Scanner.

## Prerequisites

- Kubernetes cluster (v1.24+)
- `kubectl` configured to access your cluster
- Sysdig API token
- Container registry webhook secrets/tokens

## Quick Start

### 1. Create Namespace

```bash
kubectl apply -f 01-namespace.yaml
```

### 2. Create Secrets

**Option A: Using the template (Update with your values first)**

Edit `03-secret.yaml` and replace placeholder values, then:

```bash
kubectl apply -f 03-secret.yaml
```

**Option B: Using kubectl create secret (Recommended)**

```bash
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=sysdig-api-token='YOUR_SYSDIG_API_TOKEN' \
  --from-literal=harbor-webhook-token='YOUR_HARBOR_TOKEN' \
  --from-literal=harbor-username='YOUR_HARBOR_USERNAME' \
  --from-literal=harbor-password='YOUR_HARBOR_PASSWORD' \
  --from-literal=dockerhub-webhook-secret='YOUR_DOCKERHUB_SECRET' \
  --from-literal=gitlab-webhook-token='YOUR_GITLAB_TOKEN' \
  --from-literal=gitlab-username='YOUR_GITLAB_USERNAME' \
  --from-literal=gitlab-password='YOUR_GITLAB_PASSWORD'
```

### 3. Create ConfigMap

Edit `02-configmap.yaml` to configure your registries, then:

```bash
kubectl apply -f 02-configmap.yaml
```

### 4. Deploy the Application

```bash
kubectl apply -f 04-deployment.yaml
kubectl apply -f 05-service.yaml
```

### 5. Configure Ingress (Optional)

Edit `06-ingress.yaml` to set your domain, then:

```bash
kubectl apply -f 06-ingress.yaml
```

**Or deploy all at once:**

```bash
kubectl apply -f .
```

## Verification

### Check Deployment Status

```bash
kubectl get pods -n webhook-scanner
kubectl get deployment -n webhook-scanner
kubectl get service -n webhook-scanner
kubectl get ingress -n webhook-scanner
```

### Check Pod Logs

```bash
kubectl logs -f deployment/webhook-scanner -n webhook-scanner
```

### Test Health Endpoints

```bash
# Port-forward to test locally
kubectl port-forward -n webhook-scanner deployment/webhook-scanner 8080:8080

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### Check Pod Events

```bash
kubectl describe pod -n webhook-scanner -l app.kubernetes.io/name=registry-webhook-scanner
```

## Configuration

### Update Configuration

1. Edit the ConfigMap:
   ```bash
   kubectl edit configmap scanner-config -n webhook-scanner
   ```

2. Restart pods to pick up changes:
   ```bash
   kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
   ```

### Update Secrets

```bash
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=sysdig-api-token='NEW_TOKEN' \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
```

### Scale Deployment

```bash
# Scale to 3 replicas
kubectl scale deployment webhook-scanner --replicas=3 -n webhook-scanner

# Or edit the deployment
kubectl edit deployment webhook-scanner -n webhook-scanner
```

## Resource Management

### Current Resource Limits

- **Requests**: 250m CPU, 256Mi memory
- **Limits**: 500m CPU, 512Mi memory

### Adjust Resources

Edit `04-deployment.yaml` and modify the `resources` section:

```yaml
resources:
  requests:
    memory: "512Mi"  # Increase for higher load
    cpu: "500m"
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

Then apply:

```bash
kubectl apply -f 04-deployment.yaml
```

## Monitoring

### View Metrics

```bash
# Pod metrics
kubectl top pods -n webhook-scanner

# Deployment metrics
kubectl top deployment webhook-scanner -n webhook-scanner
```

### View Logs

```bash
# Follow logs from all pods
kubectl logs -f -n webhook-scanner -l app.kubernetes.io/name=registry-webhook-scanner

# Logs from specific pod
kubectl logs -f -n webhook-scanner <pod-name>

# Previous pod logs (after restart)
kubectl logs -n webhook-scanner <pod-name> --previous
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n webhook-scanner

# Describe pod for events
kubectl describe pod <pod-name> -n webhook-scanner

# Check logs
kubectl logs <pod-name> -n webhook-scanner
```

Common issues:
- **ImagePullBackOff**: Docker image not accessible
- **CrashLoopBackOff**: Application failing to start (check logs)
- **Pending**: Resource constraints or node selector issues

### Configuration Issues

```bash
# Verify ConfigMap
kubectl get configmap scanner-config -n webhook-scanner -o yaml

# Verify Secrets exist (values won't be shown)
kubectl get secret scanner-secrets -n webhook-scanner

# Check environment variables in pod
kubectl exec -n webhook-scanner <pod-name> -- env | grep -E '(PORT|LOG_LEVEL|CONFIG_FILE)'
```

### Webhook Not Receiving Requests

1. **Check Ingress**:
   ```bash
   kubectl get ingress -n webhook-scanner
   kubectl describe ingress webhook-scanner -n webhook-scanner
   ```

2. **Test Service**:
   ```bash
   kubectl port-forward -n webhook-scanner svc/webhook-scanner 8080:80
   curl -X POST http://localhost:8080/webhook -d '{"test":"data"}'
   ```

3. **Check Registry Configuration**:
   - Verify webhook URL points to your ingress domain
   - Verify webhook secrets match configuration
   - Check registry webhook delivery logs

### Performance Issues

```bash
# Check resource usage
kubectl top pods -n webhook-scanner

# Check if pods are throttled
kubectl describe pod -n webhook-scanner <pod-name> | grep -i throttl

# Increase resources or scale up
kubectl scale deployment webhook-scanner --replicas=3 -n webhook-scanner
```

## Backup and Restore

### Backup Configuration

```bash
kubectl get configmap scanner-config -n webhook-scanner -o yaml > scanner-config-backup.yaml
kubectl get secret scanner-secrets -n webhook-scanner -o yaml > scanner-secrets-backup.yaml
```

### Restore Configuration

```bash
kubectl apply -f scanner-config-backup.yaml
kubectl apply -f scanner-secrets-backup.yaml
```

## Upgrading

### Rolling Update

1. Update the image tag in `04-deployment.yaml`
2. Apply the changes:
   ```bash
   kubectl apply -f 04-deployment.yaml
   ```

3. Monitor the rollout:
   ```bash
   kubectl rollout status deployment/webhook-scanner -n webhook-scanner
   ```

### Rollback

```bash
# View rollout history
kubectl rollout history deployment/webhook-scanner -n webhook-scanner

# Rollback to previous version
kubectl rollout undo deployment/webhook-scanner -n webhook-scanner

# Rollback to specific revision
kubectl rollout undo deployment/webhook-scanner -n webhook-scanner --to-revision=2
```

## Cleanup

### Delete All Resources

```bash
kubectl delete namespace webhook-scanner
```

### Delete Specific Resources

```bash
kubectl delete -f 06-ingress.yaml
kubectl delete -f 05-service.yaml
kubectl delete -f 04-deployment.yaml
kubectl delete -f 03-secret.yaml
kubectl delete -f 02-configmap.yaml
kubectl delete -f 01-namespace.yaml
```

## Security Best Practices

1. **Use Secrets Management**: Consider using external secrets managers (Vault, AWS Secrets Manager)
2. **Network Policies**: Restrict traffic to/from the scanner pods
3. **RBAC**: Use dedicated service account with minimal permissions
4. **TLS**: Always use HTTPS for webhook endpoints
5. **Image Scanning**: Scan the Docker image for vulnerabilities before deployment
6. **Regular Updates**: Keep Sysdig CLI and scanner service up to date

## Support

- **Issues**: Check pod logs and events first
- **Documentation**: See main [README.md](../../README.MD) for service details
- **OpenSpec**: Design documents in [openspec/changes/registry-webhook-scanner/](../../openspec/changes/registry-webhook-scanner/)

# Security Considerations

Comprehensive security guide for deploying and operating the Registry Webhook Scanner.

## Table of Contents

- [Security Architecture](#security-architecture)
- [Authentication](#authentication)
- [Secret Management](#secret-management)
- [Network Security](#network-security)
- [Container Security](#container-security)
- [Kubernetes Security](#kubernetes-security)
- [Input Validation](#input-validation)
- [Audit and Logging](#audit-and-logging)
- [Threat Model](#threat-model)
- [Security Best Practices](#security-best-practices)
- [Incident Response](#incident-response)
- [Security Checklist](#security-checklist)

## Security Architecture

### Defense in Depth

The Registry Webhook Scanner implements multiple security layers:

```
┌─────────────────────────────────────────────────────────┐
│ Layer 1: Network - TLS, Firewall, Network Policies     │
├─────────────────────────────────────────────────────────┤
│ Layer 2: Authentication - HMAC/Bearer Token            │
├─────────────────────────────────────────────────────────┤
│ Layer 3: Authorization - Registry-specific tokens      │
├─────────────────────────────────────────────────────────┤
│ Layer 4: Input Validation - Payload size, JSON parsing │
├─────────────────────────────────────────────────────────┤
│ Layer 5: Container - Non-root, read-only filesystem    │
├─────────────────────────────────────────────────────────┤
│ Layer 6: Audit - Structured logging, request tracking  │
└─────────────────────────────────────────────────────────┘
```

### Security Principles

1. **Least Privilege**: Run with minimal permissions required
2. **Defense in Depth**: Multiple security controls at different layers
3. **Fail Secure**: Reject requests on authentication/validation failures
4. **Audit Everything**: Log all security-relevant events
5. **Secure by Default**: Security features enabled out of the box

## Authentication

### Webhook Authentication

Always enable webhook authentication to prevent unauthorized scan requests.

#### HMAC Signature Verification

**How it works:**
1. Registry signs webhook payload with shared secret using HMAC-SHA256
2. Signature sent in `X-Hub-Signature-256` or `X-Signature` header
3. Scanner recomputes signature and compares using constant-time comparison

**Configuration:**
```yaml
registries:
  - name: dockerhub
    type: dockerhub
    auth:
      type: hmac
      secret: ${DOCKERHUB_WEBHOOK_SECRET}  # Strong random value
```

**Security considerations:**
- **Secret strength**: Use cryptographically random secrets (min 32 bytes)
- **Secret rotation**: Rotate secrets periodically (e.g., every 90 days)
- **Timing attacks**: Signature comparison uses constant-time algorithm
- **Replay attacks**: Consider implementing nonce/timestamp validation

**Generate strong secrets:**
```bash
# Generate 32-byte random secret
openssl rand -base64 32

# Or
head -c 32 /dev/urandom | base64
```

#### Bearer Token Authentication

**How it works:**
1. Registry sends token in `Authorization: Bearer <token>` header
2. Scanner validates token matches configured value using constant-time comparison

**Configuration:**
```yaml
registries:
  - name: harbor
    type: harbor
    auth:
      type: bearer
      token: ${HARBOR_WEBHOOK_TOKEN}  # Strong random token
```

**Security considerations:**
- **Token strength**: Use long random tokens (min 32 characters)
- **Token transmission**: Only accept tokens over HTTPS
- **Token storage**: Store in Kubernetes Secrets, not ConfigMaps
- **Token rotation**: Rotate regularly, coordinate with registry

#### No Authentication (NOT RECOMMENDED)

```yaml
registries:
  - name: public-registry
    auth:
      type: none  # Only for testing or IP-restricted environments
```

**Only acceptable when:**
- Testing in isolated environment
- IP allowlisting restricts access to known registries
- Behind internal firewall with no internet access

### Sysdig API Token Security

The Sysdig API token grants access to scan images and view results.

**Protection measures:**
1. **Never commit tokens**: Use environment variables or Secrets
2. **Restrict permissions**: Use service-specific token with minimal permissions
3. **Rotate regularly**: Establish token rotation policy
4. **Monitor usage**: Track token usage in Sysdig dashboard
5. **Revoke compromised tokens**: Immediately revoke if exposed

**Kubernetes Secret:**
```bash
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=sysdig-api-token='YOUR_TOKEN_HERE'
```

**Token sanitization in logs:**
The scanner automatically sanitizes tokens in logs:
```
# Actual token: abc123def456ghi789
# Logged as:    ab*********************89
```

## Secret Management

### Environment Variables

**Pros:**
- Simple for development and Docker
- Supported by all orchestration platforms

**Cons:**
- Visible in process listings
- Included in container inspect output
- May be logged in crash dumps

**Usage:**
```bash
export SYSDIG_API_TOKEN=your-token
export HARBOR_WEBHOOK_TOKEN=your-token
./webhook-server
```

### Kubernetes Secrets

**Recommended for production Kubernetes deployments.**

**Create Secret:**
```bash
kubectl create secret generic scanner-secrets \
  --namespace=webhook-scanner \
  --from-literal=sysdig-api-token='YOUR_TOKEN' \
  --from-literal=harbor-token='HARBOR_TOKEN'
```

**Mount as environment variables:**
```yaml
env:
- name: SYSDIG_API_TOKEN
  valueFrom:
    secretKeyRef:
      name: scanner-secrets
      key: sysdig-api-token
```

**Mount as files (more secure):**
```yaml
volumeMounts:
- name: secrets
  mountPath: /var/secrets
  readOnly: true
volumes:
- name: secrets
  secret:
    secretName: scanner-secrets
    defaultMode: 0400  # Read-only for owner
```

**Use ${FILE:secret-name} in config:**
```yaml
registries:
  - name: harbor
    auth:
      token: ${FILE:harbor-webhook-token}  # Reads from /var/secrets/harbor-webhook-token
```

### External Secret Managers

For enhanced security, integrate with external secret management:

#### HashiCorp Vault

```yaml
# Using Vault Secrets Operator
apiVersion: v1
kind: Secret
metadata:
  name: scanner-secrets
  annotations:
    vault.security.banzaicloud.io/vault-addr: "https://vault:8200"
    vault.security.banzaicloud.io/vault-role: "scanner"
    vault.security.banzaicloud.io/vault-path: "secret"
type: Opaque
data:
  sysdig-token: vault:secret/data/scanner#sysdig-token
```

#### AWS Secrets Manager

```yaml
# Using External Secrets Operator
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: scanner-secrets
spec:
  secretStoreRef:
    name: aws-secrets-manager
  target:
    name: scanner-secrets
  data:
  - secretKey: sysdig-api-token
    remoteRef:
      key: prod/scanner/sysdig-token
```

### Secret Rotation

**Rotation procedure:**

1. **Generate new secret:**
   ```bash
   NEW_SECRET=$(openssl rand -base64 32)
   ```

2. **Update registry configuration** (webhook secret/token)

3. **Update Kubernetes Secret:**
   ```bash
   kubectl create secret generic scanner-secrets \
     --namespace=webhook-scanner \
     --from-literal=harbor-token="$NEW_SECRET" \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

4. **Restart pods** to load new secret:
   ```bash
   kubectl rollout restart deployment/webhook-scanner -n webhook-scanner
   ```

5. **Verify** webhooks work with new secret

6. **Remove old secret** from registry

## Network Security

### TLS/HTTPS

**Always use HTTPS for webhook endpoints.**

```yaml
# Kubernetes Ingress with TLS
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: webhook-scanner
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - webhook.example.com
    secretName: webhook-scanner-tls
  rules:
  - host: webhook.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: webhook-scanner
            port:
              number: 80
```

**TLS best practices:**
- Use TLS 1.2 or higher
- Use strong cipher suites
- Enable HSTS (HTTP Strict Transport Security)
- Use valid certificates (Let's Encrypt, commercial CA)

### Network Policies

**Restrict traffic to/from scanner pods.**

**Ingress policy** (only allow ingress controller):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: webhook-scanner-ingress
  namespace: webhook-scanner
spec:
  podSelector:
    matchLabels:
      app: webhook-scanner
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
```

**Egress policy** (only allow Sysdig and registries):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: webhook-scanner-egress
  namespace: webhook-scanner
spec:
  podSelector:
    matchLabels:
      app: webhook-scanner
  policyTypes:
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow HTTPS to Sysdig
  - to:
    - podSelector: {}
    ports:
    - protocol: TCP
      port: 443
  # Allow registry access
  - to:
    - podSelector: {}
    ports:
    - protocol: TCP
      port: 443
```

### Firewall Rules

**Cloud provider firewall rules:**

**Ingress:**
- Allow HTTPS (443) from registry IP ranges
- Allow HTTP (80) for Let's Encrypt validation (temporary)
- Deny all other ingress

**Egress:**
- Allow HTTPS (443) to Sysdig backend (secure.sysdig.com, us2.app.sysdig.com, etc.)
- Allow HTTPS (443) to container registries
- Allow DNS (53/UDP)
- Deny all other egress

### IP Allowlisting

**If registry provides static IPs**, restrict webhook endpoint access:

**Nginx Ingress:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: webhook-scanner
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "192.0.2.0/24,198.51.100.0/24"
spec:
  # ...
```

**Cloud Load Balancer:**
```bash
# AWS ALB
aws elbv2 modify-rule \
  --rule-arn arn:aws:... \
  --conditions Field=source-ip,SourceIpConfig={"Values"=["192.0.2.0/24"]}
```

## Container Security

### Non-Root User

**Always run as non-root user.**

```dockerfile
# Dockerfile
FROM alpine:3.19
RUN adduser -D -u 1000 scanner
USER scanner
```

**Kubernetes:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
```

### Read-Only Root Filesystem

**Prevent container filesystem modification.**

```yaml
securityContext:
  readOnlyRootFilesystem: true
```

**If temp files needed:**
```yaml
volumeMounts:
- name: tmp
  mountPath: /tmp
volumes:
- name: tmp
  emptyDir: {}
```

### Drop Capabilities

**Remove all Linux capabilities.**

```yaml
securityContext:
  capabilities:
    drop:
    - ALL
```

### Image Scanning

**Scan Docker image for vulnerabilities before deployment.**

```bash
# Using Sysdig CLI
sysdig-cli-scanner your-registry/webhook-scanner:latest

# Using Trivy
trivy image your-registry/webhook-scanner:latest

# Using Grype
grype your-registry/webhook-scanner:latest
```

**Fix vulnerabilities:**
- Update base image to latest patch version
- Update Go dependencies: `go get -u && go mod tidy`
- Rebuild image with fixes

### Image Signing and Verification

**Sign images with Sigstore/Cosign:**

```bash
# Sign image
cosign sign --key cosign.key your-registry/webhook-scanner:v1.0.0

# Verify signature
cosign verify --key cosign.pub your-registry/webhook-scanner:v1.0.0
```

**Enforce signature verification in Kubernetes** (using Kyverno):
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-images
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-signature
    match:
      resources:
        kinds:
        - Pod
    verifyImages:
    - image: "your-registry/webhook-scanner:*"
      key: |-
        -----BEGIN PUBLIC KEY-----
        ...
        -----END PUBLIC KEY-----
```

## Kubernetes Security

### RBAC

**Use minimal RBAC permissions.**

**Service Account:**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: webhook-scanner
  namespace: webhook-scanner
```

**Role** (if needed for health checks):
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: webhook-scanner
  namespace: webhook-scanner
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
```

**RoleBinding:**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: webhook-scanner
  namespace: webhook-scanner
subjects:
- kind: ServiceAccount
  name: webhook-scanner
roleRef:
  kind: Role
  name: webhook-scanner
  apiGroup: rbac.authorization.k8s.io
```

### Pod Security Standards

**Apply Pod Security Standards** (Kubernetes 1.25+):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: webhook-scanner
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

### Resource Limits

**Prevent resource exhaustion attacks.**

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

**Namespace-level limits:**
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: webhook-scanner-quota
  namespace: webhook-scanner
spec:
  hard:
    requests.cpu: "2"
    requests.memory: 2Gi
    limits.cpu: "4"
    limits.memory: 4Gi
```

## Input Validation

### Payload Size Limits

**Prevent memory exhaustion from large payloads.**

```go
// HTTP handler with size limit
http.MaxBytesReader(w, r.Body, 1<<20)  // 1MB limit
```

**Nginx Ingress:**
```yaml
annotations:
  nginx.ingress.kubernetes.io/proxy-body-size: "1m"
```

### JSON Validation

**Strict JSON parsing with limited nesting.**

```go
decoder := json.NewDecoder(io.LimitReader(body, maxPayloadSize))
decoder.DisallowUnknownFields()  // Reject unexpected fields
```

### Image Reference Validation

**Validate image references to prevent injection.**

```go
func ValidateImageReference(ref string) error {
	// Check for shell metacharacters
	if strings.ContainsAny(ref, ";|&$`()") {
		return errors.New("invalid characters in image reference")
	}

	// Validate format
	if !imageRefPattern.MatchString(ref) {
		return errors.New("invalid image reference format")
	}

	return nil
}
```

### Rate Limiting

**Prevent DoS attacks.**

**Nginx Ingress:**
```yaml
annotations:
  nginx.ingress.kubernetes.io/limit-rps: "10"
  nginx.ingress.kubernetes.io/limit-rpm: "100"
```

**Application-level** (using golang.org/x/time/rate):
```go
limiter := rate.NewLimiter(rate.Limit(10), 100)  // 10 req/s, burst 100

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

## Audit and Logging

### Security Event Logging

**All security-relevant events are logged:**

- Webhook authentication success/failure
- Invalid payloads
- Scan start/completion
- Configuration changes
- Errors and exceptions

**Example security logs:**
```json
{"level":"warn","msg":"authentication failed","registry":"harbor","reason":"invalid token","source_ip":"203.0.113.45"}
{"level":"warn","msg":"payload too large","size":2097152,"limit":1048576}
{"level":"error","msg":"invalid image reference","image":"test; rm -rf /","error":"invalid characters"}
```

### Log Retention

**Retain logs for security investigations:**

- **Minimum**: 30 days
- **Recommended**: 90 days
- **Regulated environments**: 1+ years

**Configure log aggregation:**
```yaml
# FluentD/Fluent Bit
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
data:
  fluent-bit.conf: |
    [OUTPUT]
        Name s3
        Match webhook-scanner.*
        bucket security-logs
        region us-east-1
        store_dir /var/log/fluentbit
```

### Immutable Logs

**Prevent log tampering:**

- Send logs to centralized logging (Splunk, ELK, CloudWatch)
- Use write-once storage (S3 with object lock)
- Implement log integrity verification (checksums)

## Threat Model

### Threat Actors

1. **External Attackers**: Attempt to trigger unauthorized scans, DoS
2. **Compromised Registry**: Sends malicious webhooks
3. **Insider Threat**: Misuse of scanner to scan unauthorized images
4. **Supply Chain Attack**: Malicious dependencies, backdoored base images

### Attack Vectors

| Attack | Impact | Mitigation |
|--------|--------|------------|
| Webhook spoofing | Unauthorized scans, resource exhaustion | HMAC/bearer authentication |
| Payload injection | Command injection, code execution | Input validation, no shell execution |
| DoS (large payloads) | Memory exhaustion, service crash | Payload size limits |
| DoS (flood) | Resource exhaustion | Rate limiting, queue limits |
| Secret exposure | Unauthorized registry/Sysdig access | Secret management, sanitized logs |
| Man-in-the-middle | Credential interception | TLS enforcement |
| Container escape | Host compromise | Non-root, read-only FS, drop capabilities |
| Privilege escalation | Kubernetes cluster access | RBAC, Pod Security Standards |

### Security Controls

| Control | Layer | Status |
|---------|-------|--------|
| HMAC/bearer authentication | Application | ✅ Implemented |
| TLS encryption | Network | ✅ Recommended |
| Input validation | Application | ✅ Implemented |
| Rate limiting | Network/Application | ⚠️ Optional |
| Network policies | Network | ⚠️ Optional |
| Non-root containers | Container | ✅ Implemented |
| Read-only filesystem | Container | ✅ Implemented |
| Secret management | Platform | ✅ Supported |
| RBAC | Platform | ⚠️ Optional |
| Audit logging | Application | ✅ Implemented |

## Security Best Practices

### Development

1. **Dependency management**:
   - Pin dependency versions in go.mod
   - Run `go mod verify` to check integrity
   - Use Dependabot/Renovate for updates
   - Review dependency licenses

2. **Static analysis**:
   ```bash
   golangci-lint run ./...
   gosec ./...
   go vet ./...
   ```

3. **Vulnerability scanning**:
   ```bash
   govulncheck ./...
   trivy fs .
   ```

### Deployment

1. **Use Kubernetes Secrets** for all sensitive data
2. **Enable TLS** for webhook endpoint (Let's Encrypt)
3. **Apply Network Policies** to restrict traffic
4. **Set resource limits** to prevent DoS
5. **Use Pod Security Standards** (restricted)
6. **Enable audit logging** in Kubernetes
7. **Monitor and alert** on security events

### Operations

1. **Rotate secrets** every 90 days
2. **Update base images** monthly
3. **Update dependencies** regularly
4. **Review logs** for suspicious activity
5. **Test disaster recovery** procedures
6. **Maintain incident response** plan

## Incident Response

### Suspected Compromise

1. **Immediate actions**:
   - Isolate affected pods: `kubectl delete pod <pod-name>`
   - Revoke all tokens (Sysdig API, webhook secrets)
   - Review logs for unauthorized activity
   - Preserve forensic evidence (pod logs, YAML manifests)

2. **Investigation**:
   - Identify entry point (logs, network traffic)
   - Determine scope (what was accessed)
   - Timeline reconstruction
   - Root cause analysis

3. **Recovery**:
   - Rotate all secrets
   - Rebuild images from clean source
   - Redeploy with enhanced security controls
   - Update documentation

4. **Post-incident**:
   - Document incident (timeline, impact, response)
   - Update threat model
   - Improve detection capabilities
   - Share lessons learned

### Security Contacts

- **Security issues**: security@example.com
- **PGP key**: [Link to public key]
- **Responsible disclosure**: 90-day disclosure policy

## Security Checklist

### Pre-Deployment

- [ ] Webhook authentication configured (HMAC or bearer)
- [ ] Sysdig API token stored in Secret
- [ ] TLS certificate configured for ingress
- [ ] Network policies defined and tested
- [ ] Container runs as non-root user
- [ ] Read-only root filesystem enabled
- [ ] Resource limits configured
- [ ] Image scanned for vulnerabilities
- [ ] Secrets not committed to version control
- [ ] Documentation reviewed

### Post-Deployment

- [ ] Webhook authentication tested and working
- [ ] TLS certificate valid and trusted
- [ ] Logs centralized and retained
- [ ] Monitoring and alerting configured
- [ ] Incident response plan documented
- [ ] Secret rotation schedule established
- [ ] Backup and recovery tested
- [ ] Security training provided to operators

### Ongoing

- [ ] Monthly: Update base images
- [ ] Monthly: Review logs for anomalies
- [ ] Quarterly: Rotate webhook secrets
- [ ] Quarterly: Rotate Sysdig API token
- [ ] Quarterly: Review and update threat model
- [ ] Annually: Security audit/penetration test
- [ ] Annually: Review and update security policies

## Reporting Security Issues

If you discover a security vulnerability:

1. **DO NOT** open a public GitHub issue
2. Email security@example.com with details
3. Include:
   - Description of vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)
4. Allow time for fix before disclosure (90 days)

We appreciate responsible disclosure and will acknowledge your contribution in release notes (unless you prefer anonymity).

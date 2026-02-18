## Context

The current implementation uses the Sysdig CLI Scanner exclusively, which downloads container images to scan them locally. The CLI Scanner works well but has limitations:
- Requires network bandwidth to pull images
- Slower for large images
- Requires local storage for image layers
- Not ideal for cloud-native, serverless deployments

Sysdig Registry Scanner provides an alternative that scans images directly in the registry via API without pulling images. This design adds Registry Scanner support while maintaining backward compatibility with the existing CLI Scanner.

**Current State:**
- `pkg/scanner/scanner.go`: Hardcoded to use `os/exec` to invoke CLI binary
- Scanner struct directly coupled to CLI implementation
- Configuration assumes CLI Scanner only

**Constraints:**
- Must maintain backward compatibility (existing users default to CLI Scanner)
- Both scanner types must share the same ScanRequest/ScanResult interfaces
- Configuration should allow per-registry scanner selection
- Should not increase deployment complexity (single container)

## Goals / Non-Goals

**Goals:**
- Add Sysdig Registry Scanner as an alternative scanning backend
- Allow users to choose scanner type per registry via configuration
- Maintain backward compatibility (default to CLI Scanner)
- Support mixed configurations (some registries use CLI, others use Registry Scanner)
- Provide clear configuration guidance on when to use each scanner type

**Non-Goals:**
- Automatic scanner selection based on image size or registry type
- Parallel scanning with both scanners for comparison
- Migration of existing CLI Scanner code (it remains as-is)
- Support for other third-party scanning APIs
- Distributed scanning across multiple scanner instances

## Decisions

### 1. Scanner Backend Abstraction

**Decision:** Create a `ScannerBackend` interface with two implementations: `CLIScanner` and `RegistryScanner`

**Rationale:**
- Decouples scanner logic from specific implementation
- Allows runtime selection based on configuration
- Makes testing easier (mock backend implementations)
- Enables future scanner types without code changes

**Alternative Considered:** Conditional logic in existing Scanner struct
- Rejected: Would make Scanner.go complex and hard to maintain
- Rejected: Tight coupling makes testing difficult

**Implementation:**
```go
type ScannerBackend interface {
    Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error)
    Type() string
}

type CLIScanner struct { /* existing implementation */ }
type RegistryScanner struct { /* new API client */ }
```

### 2. Configuration Schema

**Decision:** Add `scanner.type` field with enum values `cli` (default) or `registry`, with per-registry override capability

**Rationale:**
- Global default makes sense (most users choose one type)
- Per-registry override supports mixed environments (e.g., public repos via CLI, private via Registry Scanner)
- Explicit over implicit (no automatic selection)
- Backward compatible (missing type defaults to `cli`)

**Alternative Considered:** Registry-level only configuration
- Rejected: Would require repeating scanner config for every registry
- Rejected: No sensible default for new registries

**Configuration Structure:**
```yaml
scanner:
  sysdig_token: ${SYSDIG_API_TOKEN}
  type: cli  # Global default: cli or registry
  default_timeout: 300s

  # CLI Scanner specific (existing)
  cli_path: /usr/local/bin/sysdig-cli-scanner

  # Registry Scanner specific (new)
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true

registries:
  - name: harbor-prod
    type: harbor
    scanner:
      type: registry  # Override: use Registry Scanner for this registry
      timeout: 600s
```

### 3. Registry Scanner API Client

**Decision:** Implement Registry Scanner client using standard `net/http` client with structured API calls

**Rationale:**
- No external Sysdig Registry Scanner SDK available (as of implementation)
- Standard library HTTP client is well-tested and reliable
- Allows fine-grained control over timeouts, retries, headers
- Minimal dependencies

**Alternative Considered:** Generate OpenAPI client from Sysdig API spec
- Rejected: Adds code generation complexity
- Rejected: Sysdig may not provide public OpenAPI spec for Registry Scanner

**API Integration Points:**
- `POST /api/scanning/v1/registry/scan` - Initiate scan
- `GET /api/scanning/v1/registry/scan/{scanId}` - Poll for results
- Authentication via Sysdig API token in `Authorization: Bearer` header

### 4. Scanner Factory Pattern

**Decision:** Use factory function to create appropriate scanner backend based on configuration

**Rationale:**
- Centralizes scanner creation logic
- Hides implementation details from callers
- Makes it easy to add new scanner types in the future
- Validates configuration at initialization time

**Implementation:**
```go
func NewScannerBackend(cfg *config.Config, registryName string, logger *logrus.Logger) (ScannerBackend, error) {
    scannerType := determineScannerType(cfg, registryName)

    switch scannerType {
    case "cli":
        return NewCLIScanner(cfg, logger), nil
    case "registry":
        return NewRegistryScanner(cfg, logger), nil
    default:
        return nil, fmt.Errorf("unsupported scanner type: %s", scannerType)
    }
}
```

### 5. Credential Handling

**Decision:** Reuse existing Sysdig API token for both scanner types; Registry Scanner requires project ID

**Rationale:**
- Same Sysdig account for both scanners (no duplicate credentials)
- Project ID provides multi-tenancy support in Sysdig
- Simpler credential management (one token)

**Alternative Considered:** Separate tokens for CLI and Registry Scanner
- Rejected: Unnecessary complexity for users
- Rejected: Same Sysdig backend, same authentication

**Configuration:**
- `scanner.sysdig_token`: Used by both CLI and Registry Scanner
- `scanner.registry_scanner.project_id`: Required for Registry Scanner (optional for CLI)

### 6. Error Handling and Retries

**Decision:** Registry Scanner uses HTTP client retries; CLI Scanner keeps existing exec-based error handling

**Rationale:**
- Network errors are more common with API calls (transient failures)
- CLI Scanner errors are typically deterministic (image not found, scan policy violation)
- HTTP 5xx errors are retriable, 4xx are not

**Retry Strategy:**
- Registry Scanner: Exponential backoff for 5xx, timeout, connection errors (max 3 retries)
- CLI Scanner: No automatic retries (handled by existing queue retry logic)

### 7. Polling for Registry Scanner Results

**Decision:** Registry Scanner client polls for scan completion with configurable poll interval (default 5s)

**Rationale:**
- Registry Scanner API is asynchronous (returns immediately with scan ID)
- Polling is simpler than webhooks (no additional endpoint needed)
- Fits existing synchronous scan workflow

**Alternative Considered:** Webhook callback when scan completes
- Rejected: Requires exposing additional endpoint and handling async callbacks
- Rejected: Complicates worker pool logic (would need callback routing)

**Implementation:**
- POST to initiate scan, receive scan ID
- Poll GET endpoint every 5s (configurable) until status is "completed" or "failed"
- Overall timeout enforced via context (same as CLI Scanner)

### 8. Backward Compatibility Strategy

**Decision:** Default scanner type is `cli` if not specified; existing configs work without changes

**Rationale:**
- Zero-impact migration for existing users
- Opt-in for Registry Scanner (users must explicitly enable)
- Clear upgrade path

**Validation:**
- If `scanner.type=registry` but `registry_scanner.project_id` is missing → error at startup
- If `scanner.type=cli` but CLI binary not found → error at startup

## Risks / Trade-offs

### Risk: Registry Scanner API Changes
Registry Scanner API may change or require different authentication in the future.

**Mitigation:**
- Use API versioning in URL path (`/v1/`)
- Add `api_version` config field for future flexibility
- Document required Sysdig product version for Registry Scanner support

### Risk: Polling Overhead
Polling for scan results adds latency and additional API calls.

**Mitigation:**
- Make poll interval configurable (default 5s, allow 1-30s)
- Use exponential backoff if scan takes longer than expected
- Consider webhook callback in future version if polling becomes problematic

### Risk: Registry Scanner May Not Support All Registries
Registry Scanner may have limitations on which registries it can scan (authentication, network access).

**Mitigation:**
- Document supported registries for each scanner type
- Allow per-registry scanner type override
- Provide fallback mechanism (if Registry Scanner fails, optionally fall back to CLI)

### Risk: Increased Configuration Complexity
Adding scanner type options increases cognitive load for users.

**Mitigation:**
- Provide sensible defaults (CLI Scanner)
- Add configuration validation with helpful error messages
- Update docs with decision guide: "When to use CLI vs Registry Scanner"
- Provide example configs for common scenarios

### Trade-off: Single Container vs Microservices
Keeping both scanners in one container increases binary size and complexity.

**Accepted Trade-off:**
- Benefit: Simpler deployment (no orchestration of multiple services)
- Benefit: Existing deployment process unchanged
- Cost: Larger Docker image (~50MB increase for dependencies)
- Cost: More complex initialization logic

### Trade-off: Polling vs Webhooks
Polling adds latency (5s average) vs webhook callback complexity.

**Accepted Trade-off:**
- Benefit: Simpler implementation, no additional endpoints
- Benefit: Fits existing synchronous worker model
- Cost: 5s average latency increase for Registry Scanner
- Cost: More API calls to Sysdig (not expected to hit rate limits)

## Migration Plan

### Phase 1: Implementation (This Change)
1. Create `ScannerBackend` interface and `CLIScanner` wrapper (refactor existing code)
2. Implement `RegistryScanner` with API client
3. Add configuration schema changes
4. Update scanner initialization in `main.go`
5. Add unit tests and mocks for Registry Scanner
6. Update documentation

### Phase 2: Testing
1. Deploy to test environment with CLI Scanner (verify no regression)
2. Deploy with Registry Scanner for test registry
3. Validate mixed configuration (some CLI, some Registry Scanner)
4. Performance testing: compare scan durations
5. Load testing: ensure API rate limits not hit

### Phase 3: Rollout (User-Driven)
1. Announce feature in release notes
2. Provide migration guide: when to choose CLI vs Registry Scanner
3. Users opt-in by changing `scanner.type=registry` in config
4. No automatic migration (users must explicitly enable)

### Rollback Strategy
If issues arise with Registry Scanner:
1. Change `scanner.type=cli` in ConfigMap
2. Restart deployment: `kubectl rollout restart deployment/webhook-scanner`
3. All registries fall back to CLI Scanner (no data loss, scans may be slower)

### Configuration Migration
Existing configurations require no changes:
```yaml
# Before (works as-is)
scanner:
  sysdig_token: ${TOKEN}

# After (explicit CLI)
scanner:
  type: cli  # Optional, defaults to cli
  sysdig_token: ${TOKEN}

# After (opt-in to Registry Scanner)
scanner:
  type: registry
  sysdig_token: ${TOKEN}
  registry_scanner:
    project_id: ${PROJECT_ID}
```

## Open Questions

1. **Registry Scanner API Endpoint:** Is the API URL the same for all Sysdig regions (US, EU, etc.) or region-specific?
   - **Resolution Needed:** Test with EU Sysdig instance, make `api_url` configurable

2. **Project ID Requirement:** Can Registry Scanner work without project ID (use default project)?
   - **Resolution Needed:** Consult Sysdig docs, make project_id optional if possible

3. **Scan Result Format:** Does Registry Scanner return the same vulnerability schema as CLI?
   - **Resolution Needed:** Verify JSON schema compatibility, add normalization if needed

4. **Rate Limiting:** What are Sysdig Registry Scanner API rate limits?
   - **Resolution Needed:** Document in CONFIGURATION.md, add rate limit handling if needed

5. **Fallback Behavior:** Should we automatically fall back to CLI Scanner if Registry Scanner fails?
   - **Resolution Needed:** Discuss with users, may be too complex for v1 (manual override for now)

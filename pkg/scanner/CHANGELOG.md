# Changelog

All notable changes to the Registry Webhook Scanner project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Registry Scanner Support**: Added Sysdig Registry Scanner API as an alternative scanning backend alongside the existing CLI Scanner
  - New scanner type configuration: `scanner.type` can be set to `"cli"` or `"registry"`
  - Per-registry scanner type overrides via `registries[].scanner.type`
  - Registry Scanner specific configuration options:
    - `scanner.registry_scanner.api_url`: Sysdig API endpoint (default: `https://secure.sysdig.com`)
    - `scanner.registry_scanner.project_id`: Required Sysdig project ID for Registry Scanner
    - `scanner.registry_scanner.verify_tls`: TLS certificate verification (default: `true`)
    - `scanner.registry_scanner.poll_interval`: Polling interval for async scan completion (default: `5s`)
  - Asynchronous scanning workflow with configurable polling
  - HTTP client with exponential backoff retry logic and rate limiting support
  - Structured logging with `scanner_type` field for observability

- **Scanner Backend Architecture**: Introduced pluggable scanner backend system
  - `ScannerBackend` interface for implementing different scanning methods
  - Scanner factory with automatic type selection based on configuration
  - Priority-based scanner type determination: per-registry override → global default → CLI fallback

- **Registry Scanner API Client**: New HTTP client implementation
  - Automatic retry logic with exponential backoff (max 3 retries)
  - 429 rate limit handling with `Retry-After` header support
  - TLS configuration support
  - Request/response logging with sanitized tokens

- **Scanner-Specific Error Types**: Added structured error handling
  - `APIError` type with retry classification
  - Error categorization (retriable vs non-retriable)
  - Improved error messages with context

### Changed

- **Configuration Schema**: Extended configuration to support multiple scanner types
  - `scanner.type` field added (values: `"cli"`, `"registry"`)
  - `scanner.registry_scanner` section for Registry Scanner configuration
  - Per-registry `scanner.type` override capability
  - Renamed internal `RegistryScannerConfig` to `ScannerOverride` for clarity

- **CLI Scanner Refactoring**: Renamed `Scanner` to `CLIScanner` for consistency
  - Implements `ScannerBackend` interface
  - All CLI-specific logic isolated in `cli_scanner.go`
  - Logging updated with `scanner_type: "cli"` field

- **Factory Pattern**: Scanner instantiation now uses factory function
  - Centralized scanner creation logic
  - Configuration validation during backend creation
  - Scanner type determination with fallback to CLI for backward compatibility

### Enhanced

- **Logging and Observability**:
  - All scanner operations include `scanner_type` field in logs
  - Request IDs tracked throughout scan lifecycle
  - API call duration metrics for Registry Scanner
  - Poll attempt tracking for Registry Scanner async operations

- **Documentation**:
  - Comprehensive [CONFIGURATION.md](docs/CONFIGURATION.md) with scanner type comparison
  - When-to-use guide for CLI vs Registry Scanner
  - [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) with Registry Scanner specific sections
  - [DEVELOPER.md](docs/DEVELOPER.md) with scanner backend architecture documentation
  - Step-by-step guide for adding new scanner backends
  - Kubernetes deployment guide with scanner type configuration examples

- **Deployment Configuration**:
  - Updated Kubernetes ConfigMap templates with scanner type examples
  - Secret templates include `SYSDIG_PROJECT_ID` field for Registry Scanner
  - Kubernetes deployment README with scenario-based configuration guides
  - Updated `config.example.yaml` with comprehensive scanner type examples

### Fixed

- Configuration validation now enforces required fields based on scanner type
- Scanner type defaults to "cli" for backward compatibility with existing configurations

### Deprecated

- None

### Removed

- None

### Security

- Registry Scanner API client includes TLS verification (configurable, enabled by default)
- API token sanitization in logs (only shows first 2 and last 2 characters)
- Structured error handling prevents token leakage in error messages

---

## Comparison: CLI Scanner vs Registry Scanner

| Feature | CLI Scanner | Registry Scanner |
|---------|-------------|------------------|
| **Image Download** | Yes (local pull required) | No (scans in Sysdig backend) |
| **Storage Required** | Yes (ephemeral or persistent volume) | No |
| **Network Dependency** | Low (only for scan result upload) | High (continuous API connectivity) |
| **Scan Initiation** | Fast (local execution) | Fast (API call) |
| **Scan Execution** | Synchronous | Asynchronous (polling) |
| **API Dependencies** | Minimal | Requires Sysdig API availability |
| **Resource Usage** | Higher (image download + scan) | Lower (API calls only) |
| **Best For** | Local/air-gapped environments | Large images, bandwidth optimization |

---

## Migration Guide

### From CLI-Only to Registry Scanner

**Step 1:** Update configuration to add Registry Scanner settings:

```yaml
scanner:
  type: registry  # Change from "cli"
  sysdig_token: ${SYSDIG_API_TOKEN}
  
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s
```

**Step 2:** Set the `SYSDIG_PROJECT_ID` environment variable:

```bash
export SYSDIG_PROJECT_ID="your-project-id"
```

**Step 3:** Restart the application to apply changes

**Step 4:** Verify scanner type in logs:

```bash
grep "scanner_type" logs/webhook-scanner.log
# Should show: "scanner_type": "registry"
```

### Using Mixed Scanner Types

Use CLI for some registries and Registry Scanner for others:

```yaml
scanner:
  type: cli  # Global default
  cli_path: /usr/local/bin/sysdig-cli-scanner
  
  # Enable Registry Scanner for overrides
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}

registries:
  - name: dockerhub
    # Uses global default (cli)
    
  - name: large-images-registry
    scanner:
      type: registry  # Override for this registry
```

---

## Configuration Examples

### Example 1: CLI Scanner Only (Default)

```yaml
scanner:
  type: cli
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  max_concurrent: 5
```

### Example 2: Registry Scanner Only

```yaml
scanner:
  type: registry
  sysdig_token: ${SYSDIG_API_TOKEN}
  default_timeout: 300s
  max_concurrent: 10
  
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s
```

### Example 3: Mixed Configuration

```yaml
scanner:
  type: cli
  sysdig_token: ${SYSDIG_API_TOKEN}
  cli_path: /usr/local/bin/sysdig-cli-scanner
  default_timeout: 300s
  max_concurrent: 5
  
  registry_scanner:
    api_url: https://secure.sysdig.com
    project_id: ${SYSDIG_PROJECT_ID}
    verify_tls: true
    poll_interval: 5s

registries:
  - name: public-registry
    # Uses CLI (fast for public images)
    
  - name: private-large-images
    scanner:
      type: registry  # Use Registry Scanner (no download)
```

---

## Breaking Changes

None. The new scanner type system is fully backward compatible:

- Existing configurations without `scanner.type` default to `"cli"`
- CLI Scanner behavior is unchanged
- All existing configuration options remain functional

---

## Contributors

This release includes contributions from the Sysdig team and community. Thank you to all contributors!

---

## Links

- [Documentation](README.md)
- [Configuration Guide](docs/CONFIGURATION.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Developer Guide](docs/DEVELOPER.md)
- [GitHub Repository](https://github.com/sysdig/registry-webhook-scanner)


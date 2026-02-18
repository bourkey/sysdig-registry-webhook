## 1. Configuration Schema Updates

- [x] 1.1 Add `ScannerType` type alias in `pkg/config/types.go` with values "cli" and "registry"
- [x] 1.2 Add `RegistryScannerConfig` struct in `pkg/config/types.go` with fields: `APIURL`, `ProjectID`, `VerifyTLS`, `PollInterval`
- [x] 1.3 Update `ScannerConfig` struct to include `Type` field (default "cli") and `RegistryScanner` nested config
- [x] 1.4 Update `ScannerOverride` struct in `RegistryConfig` to include `Type` field for per-registry override
- [x] 1.5 Add configuration validation in `pkg/config/loader.go` to check Registry Scanner requirements (project_id if type=registry)
- [x] 1.6 Add default values for Registry Scanner config (api_url, poll_interval) in loader
- [x] 1.7 Update `config.example.yaml` with scanner type options and Registry Scanner configuration example

## 2. Scanner Backend Interface

- [x] 2.1 Create `pkg/scanner/backend.go` with `ScannerBackend` interface definition
- [x] 2.2 Add `Scan(ctx, req) (*ScanResult, error)` method to interface
- [x] 2.3 Add `Type() string` method to interface
- [x] 2.4 Add `ValidateConfig() error` method to interface for startup validation

## 3. CLI Scanner Refactoring

- [x] 3.1 Rename `pkg/scanner/scanner.go` to `pkg/scanner/cli_scanner.go`
- [x] 3.2 Rename `Scanner` struct to `CLIScanner` and implement `ScannerBackend` interface
- [x] 3.3 Move existing `Scan` method implementation to `CLIScanner.Scan`
- [x] 3.4 Add `Type() string` method to `CLIScanner` returning "cli"
- [x] 3.5 Add `ValidateConfig() error` method to `CLIScanner` checking binary exists
- [x] 3.6 Update `NewScanner` to `NewCLIScanner` constructor
- [x] 3.7 Update all imports and references from `Scanner` to `CLIScanner` in existing code

## 4. Registry Scanner Implementation

- [x] 4.1 Create `pkg/scanner/registry_scanner.go` with `RegistryScanner` struct
- [x] 4.2 Implement `NewRegistryScanner` constructor with HTTP client initialization
- [x] 4.3 Implement `RegistryScanner.Scan` method with async API flow
- [x] 4.4 Add `initiateScan` helper method to POST scan request and return scan ID
- [x] 4.5 Add `pollScanStatus` helper method with configurable poll interval and exponential backoff
- [x] 4.6 Add `getScanResult` helper method to retrieve final scan results
- [x] 4.7 Implement `Type() string` method returning "registry"
- [x] 4.8 Implement `ValidateConfig() error` method checking API URL, token, project ID
- [x] 4.9 Add `buildScanRequest` helper to construct scan API request payload with registry credentials
- [x] 4.10 Add `parseScanResponse` helper to parse JSON scan results into `models.ScanResult`

## 5. HTTP Client & API Integration

- [x] 5.1 Create `pkg/scanner/registry_api.go` with Registry Scanner API constants (endpoints, headers)
- [x] 5.2 Add `makeAPIRequest` helper method with authentication (Bearer token)
- [x] 5.3 Implement retry logic for 5xx errors and network timeouts (exponential backoff, max 3 retries)
- [x] 5.4 Add rate limit handling for 429 responses (respect Retry-After header)
- [x] 5.5 Add TLS certificate verification toggle based on config
- [x] 5.6 Add request/response logging with sanitized credentials

## 6. Scanner Factory & Routing

- [x] 6.1 Create `pkg/scanner/factory.go` with scanner factory functions
- [x] 6.2 Add `NewScannerBackend(cfg, registryName, logger)` factory function
- [x] 6.3 Implement scanner type determination logic (per-registry override → global default → "cli")
- [x] 6.4 Add switch statement to create appropriate backend (CLIScanner or RegistryScanner)
- [x] 6.5 Add validation to call `ValidateConfig()` on created backend
- [x] 6.6 Update `cmd/webhook-server/main.go` to use factory function instead of direct CLIScanner creation
- [x] 6.7 Update worker pool to use `ScannerBackend` interface instead of concrete type

## 7. Error Handling & Logging

- [x] 7.1 Add Registry Scanner-specific error types in `pkg/scanner/errors.go`
- [x] 7.2 Add structured logging for Registry Scanner API calls (request ID, duration, status)
- [x] 7.3 Add logging for scanner type selection per scan
- [x] 7.4 Add error categorization (retriable vs non-retriable) for Registry Scanner errors
- [x] 7.5 Update existing error messages to clarify scanner type (CLI vs Registry)

## 8. Testing - Unit Tests

- [x] 8.1 Create `pkg/scanner/backend_test.go` with interface compliance tests
- [x] 8.2 Create `pkg/scanner/cli_scanner_test.go` with existing CLI scanner tests (migrate from scanner_test.go)
- [x] 8.3 Create `pkg/scanner/registry_scanner_test.go` with Registry Scanner unit tests
- [x] 8.4 Add test for successful scan initiation (mock HTTP POST response with scan ID)
- [x] 8.5 Add test for scan polling until completion (mock multiple GET requests)
- [x] 8.6 Add test for scan timeout during polling
- [x] 8.7 Add test for API authentication failure (401 response)
- [x] 8.8 Add test for transient error retry (5xx responses)
- [x] 8.9 Add test for non-retriable error (4xx responses)
- [x] 8.10 Add test for rate limit handling (429 response)
- [x] 8.11 Create `pkg/scanner/factory_test.go` with scanner factory tests
- [x] 8.12 Add test for scanner type selection (global default, per-registry override)
- [x] 8.13 Add test for invalid scanner type error
- [x] 8.14 Add configuration validation tests in `pkg/config/loader_test.go`

## 9. Testing - Integration Tests

- [x] 9.1 Create mock Registry Scanner API server in `test/mocks/mock-registry-scanner-api`
- [x] 9.2 Add `test/fixtures/registry-scanner-scan-response.json` with sample API response
- [x] 9.3 Add `test/fixtures/registry-scanner-result-response.json` with sample result response
- [x] 9.4 Create integration test for full Registry Scanner flow (initiate → poll → result)
- [x] 9.5 Add integration test for mixed scanner types (CLI for one registry, Registry for another)
- [x] 9.6 Add integration test for Registry Scanner timeout scenario
- [x] 9.7 Add integration test for Registry Scanner retry logic

## 10. Documentation Updates

- [x] 10.1 Update `docs/CONFIGURATION.md` with scanner type configuration section
- [x] 10.2 Add scanner type selection guide: when to use CLI vs Registry Scanner
- [x] 10.3 Document Registry Scanner-specific configuration options (api_url, project_id, etc.)
- [x] 10.4 Add configuration examples for Registry Scanner (global and per-registry)
- [x] 10.5 Update `README.md` with Registry Scanner feature in key features list
- [x] 10.6 Add Registry Scanner section to README quick start
- [x] 10.7 Update `docs/TROUBLESHOOTING.md` with Registry Scanner-specific troubleshooting
- [x] 10.8 Add troubleshooting for API connection errors, authentication failures, polling timeouts
- [x] 10.9 Update `docs/DEVELOPER.md` with scanner backend architecture section
- [x] 10.10 Document how to add new scanner backends in developer guide

## 11. Deployment Updates

- [x] 11.1 Update `deployments/kubernetes/02-configmap.yaml` with scanner type example
- [x] 11.2 Add Registry Scanner configuration example to ConfigMap
- [x] 11.3 Update `deployments/kubernetes/03-secret.yaml` template with project_id field
- [x] 11.4 Update `deployments/kubernetes/README.md` with Registry Scanner configuration steps
- [x] 11.5 Update `config.example.yaml` in repository root with Registry Scanner example

## 12. Backward Compatibility Validation

- [x] 12.1 Test that existing configurations without scanner type work (default to CLI)
- [x] 12.2 Verify CLI scanner behavior unchanged for existing users
- [x] 12.3 Test configuration migration (adding scanner.type=cli explicitly)
- [x] 12.4 Validate that existing environment variables still work
- [x] 12.5 Test mixed configuration (some registries CLI, some Registry Scanner)

## 13. Performance & Metrics

- [x] 13.1 Add metrics for Registry Scanner API call duration
- [x] 13.2 Add metrics for Registry Scanner poll attempts per scan
- [x] 13.3 Add metrics for Registry Scanner API error rates by type
- [x] 13.4 Add metrics for scanner type distribution (CLI vs Registry)
- [x] 13.5 Update logging to include scanner type in scan lifecycle logs

## 14. Final Integration

- [x] 14.1 Run all unit tests: `go test ./pkg/scanner/...`
- [x] 14.2 Run integration tests: `go test -tags=integration ./test/integration/...`
- [ ] 14.3 Build Docker image with Registry Scanner support
- [ ] 14.4 Test deployment with CLI scanner (verify no regression)
- [ ] 14.5 Test deployment with Registry Scanner configuration
- [ ] 14.6 Test deployment with mixed scanner types
- [ ] 14.7 Verify configuration validation catches missing project_id
- [x] 14.8 Update CHANGELOG.md with new feature description

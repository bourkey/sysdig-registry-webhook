## Why

Currently, the webhook scanner only supports the Sysdig CLI Scanner, which requires downloading container images locally before scanning. The Sysdig Registry Scanner offers a cloud-native alternative that scans images directly in the registry without requiring image pulls, providing better performance for large images and reducing bandwidth usage. Adding Registry Scanner as an option gives users the flexibility to choose the scanning method that best fits their infrastructure and compliance requirements.

## What Changes

- Add Sysdig Registry Scanner API client integration
- Make scanner type configurable per registry (CLI or Registry Scanner)
- Extend configuration schema to support scanner type selection
- Update scanner invocation logic to route to appropriate scanner backend
- Add Registry Scanner-specific configuration options (API endpoint, project ID, etc.)
- Update documentation to explain scanner type trade-offs and configuration

## Capabilities

### New Capabilities

- `registry-scanner-client`: Integration with Sysdig Registry Scanner API for agentless scanning directly in container registries

### Modified Capabilities

- `scanner-integration`: Extend to support multiple scanner backends (CLI and Registry Scanner) with configurable selection per registry

## Impact

**Affected Code:**
- `pkg/scanner/scanner.go`: Add scanner type abstraction and routing logic
- `pkg/scanner/`: New files for Registry Scanner API client
- `pkg/config/types.go`: Extend RegistryConfig with scanner type and Registry Scanner options
- `cmd/webhook-server/main.go`: Scanner initialization logic

**Configuration:**
- New YAML fields: `scanner.type` (cli|registry), `scanner.registry_scanner.*` options
- Backward compatible: defaults to CLI scanner if type not specified

**Dependencies:**
- Add Sysdig Registry Scanner API client library (if available) or implement HTTP client
- May require new environment variables for Registry Scanner configuration

**Testing:**
- Mock Registry Scanner API responses
- Integration tests for both scanner types
- Configuration validation tests

**Documentation:**
- Update CONFIGURATION.md with scanner type options
- Add comparison guide: when to use CLI vs Registry Scanner
- Update example configurations

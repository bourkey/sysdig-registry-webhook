## ADDED Requirements

### Requirement: Select scanner backend based on configuration

The system SHALL select the appropriate scanner backend (CLI or Registry Scanner) based on global and per-registry configuration.

#### Scenario: Global scanner type applied
- **WHEN** a registry does not specify a scanner type
- **THEN** system uses the global default scanner type from configuration

#### Scenario: Per-registry scanner type override
- **WHEN** a registry specifies a scanner type in its configuration
- **THEN** system uses that scanner type instead of the global default

#### Scenario: Default to CLI scanner
- **WHEN** no scanner type is specified in configuration
- **THEN** system defaults to CLI scanner for backward compatibility

#### Scenario: Invalid scanner type
- **WHEN** configuration specifies an unsupported scanner type
- **THEN** system logs an error and fails to initialize

### Requirement: Route scan requests to appropriate backend

The system SHALL route scan requests to the correct scanner backend implementation based on the selected scanner type.

#### Scenario: Route to CLI scanner
- **WHEN** scanner type is configured as "cli"
- **THEN** system invokes the Sysdig CLI Scanner binary

#### Scenario: Route to Registry Scanner
- **WHEN** scanner type is configured as "registry"
- **THEN** system invokes the Registry Scanner API client

#### Scenario: Mixed scanner types
- **WHEN** multiple registries use different scanner types
- **THEN** system routes each scan to the appropriate backend based on the registry configuration

### Requirement: Validate scanner backend availability

The system SHALL validate that the selected scanner backend is properly configured and available at service startup.

#### Scenario: CLI scanner binary exists
- **WHEN** CLI scanner is selected and service starts
- **THEN** system verifies the CLI binary exists and is executable

#### Scenario: Registry Scanner credentials configured
- **WHEN** Registry Scanner is selected and service starts
- **THEN** system verifies API token and project ID are configured

#### Scenario: Scanner backend not available
- **WHEN** selected scanner backend is not properly configured
- **THEN** system logs an error and fails to start

## MODIFIED Requirements

### Requirement: Invoke Sysdig CLI Scanner

The system SHALL execute the appropriate scanner backend (CLI Scanner or Registry Scanner) with appropriate parameters for each image to be scanned.

#### Scenario: Successful CLI scan invocation
- **WHEN** an image reference is ready to be scanned using CLI Scanner
- **THEN** system invokes the Sysdig CLI with the image reference and waits for completion

#### Scenario: Successful Registry Scanner invocation
- **WHEN** an image reference is ready to be scanned using Registry Scanner
- **THEN** system initiates a scan via the Registry Scanner API and polls for completion

#### Scenario: Scanner binary not found (CLI)
- **WHEN** the system attempts to invoke the CLI scanner but the binary is not installed or not in PATH
- **THEN** system logs an error and marks the scan as failed

#### Scenario: Registry Scanner API unavailable
- **WHEN** the system attempts to invoke the Registry Scanner but the API is unreachable
- **THEN** system logs an error and marks the scan as failed

#### Scenario: Multiple concurrent scans with mixed backends
- **WHEN** multiple images are queued for scanning using different scanner types
- **THEN** system invokes the appropriate scanner for each image up to the configured concurrency limit

### Requirement: Manage scanner credentials

The system SHALL provide necessary credentials to the selected scanner backend for authentication with Sysdig backend and registries.

#### Scenario: Sysdig API token provided to CLI scanner
- **WHEN** invoking the CLI scanner
- **THEN** system provides the Sysdig API token via environment variable or CLI flag

#### Scenario: Sysdig API token provided to Registry Scanner
- **WHEN** invoking the Registry Scanner
- **THEN** system includes the Sysdig API token in the API request Authorization header

#### Scenario: Registry credentials provided to CLI scanner
- **WHEN** scanning an image from a private registry using CLI scanner
- **THEN** system provides registry credentials to the scanner CLI

#### Scenario: Registry credentials provided to Registry Scanner
- **WHEN** scanning an image from a private registry using Registry Scanner
- **THEN** system includes registry credentials in the scan request payload

#### Scenario: Missing credentials
- **WHEN** required credentials are not configured for the selected scanner type
- **THEN** system logs an error and marks the scan as failed without attempting invocation

#### Scenario: Project ID required for Registry Scanner
- **WHEN** using Registry Scanner and project ID is not configured
- **THEN** system logs an error and marks the scan as failed

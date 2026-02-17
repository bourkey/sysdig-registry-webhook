## ADDED Requirements

### Requirement: Invoke Sysdig CLI Scanner

The system SHALL execute the Sysdig CLI Scanner binary with appropriate arguments for each image to be scanned.

#### Scenario: Successful scan invocation
- **WHEN** an image reference is ready to be scanned
- **THEN** system invokes the Sysdig CLI with the image reference and waits for completion

#### Scenario: Scanner binary not found
- **WHEN** the system attempts to invoke the scanner but the binary is not installed or not in PATH
- **THEN** system logs an error and marks the scan as failed

#### Scenario: Multiple concurrent scans
- **WHEN** multiple images are queued for scanning simultaneously
- **THEN** system invokes the scanner for each image up to the configured concurrency limit

### Requirement: Pass image references correctly

The system SHALL format and pass image references to the Sysdig CLI Scanner in the expected format.

#### Scenario: Public image scanned
- **WHEN** scanning a public Docker Hub image like "nginx:latest"
- **THEN** system passes the image reference as-is to the scanner CLI

#### Scenario: Private registry image scanned
- **WHEN** scanning an image from a private registry like "registry.company.com/app:v1.0"
- **THEN** system includes the full registry URL and credentials if required

#### Scenario: Image with digest scanned
- **WHEN** scanning an image specified by digest like "nginx@sha256:abc123..."
- **THEN** system passes the digest reference to the scanner CLI

### Requirement: Capture and process scanner output

The system SHALL capture the Sysdig CLI Scanner's standard output and standard error for logging and result processing.

#### Scenario: Scan completes successfully
- **WHEN** the Sysdig CLI Scanner exits with code 0
- **THEN** system captures the output, logs it, and marks the scan as successful

#### Scenario: Scan finds vulnerabilities
- **WHEN** the Sysdig CLI Scanner exits with a non-zero code indicating vulnerabilities were found
- **THEN** system captures the output, logs the findings, and marks the scan as complete (not failed)

#### Scenario: Scan fails with error
- **WHEN** the Sysdig CLI Scanner exits with an error code indicating a scan failure (not vulnerabilities)
- **THEN** system logs the error output and marks the scan as failed for retry

### Requirement: Handle scanner timeouts

The system SHALL enforce timeout limits on scanner invocations to prevent indefinite hangs.

#### Scenario: Scan completes within timeout
- **WHEN** a scan completes before the configured timeout period
- **THEN** system processes the result normally

#### Scenario: Scan exceeds timeout
- **WHEN** a scan runs longer than the configured timeout period
- **THEN** system terminates the scanner process and marks the scan as failed

#### Scenario: Configurable timeout per registry
- **WHEN** different registries have different typical scan durations
- **THEN** system allows timeout configuration per registry or globally

### Requirement: Manage scanner credentials

The system SHALL provide necessary credentials to the Sysdig CLI Scanner for authentication with Sysdig backend and registries.

#### Scenario: Sysdig API token provided
- **WHEN** invoking the scanner
- **THEN** system provides the Sysdig API token via environment variable or CLI flag

#### Scenario: Registry credentials provided
- **WHEN** scanning an image from a private registry requiring authentication
- **THEN** system provides registry credentials to the scanner CLI

#### Scenario: Missing credentials
- **WHEN** required credentials are not configured
- **THEN** system logs an error and marks the scan as failed without attempting invocation

### Requirement: Handle scanner process lifecycle

The system SHALL properly manage the lifecycle of scanner processes including cleanup and resource management.

#### Scenario: Scanner process cleanup on success
- **WHEN** a scanner process completes successfully
- **THEN** system ensures the process is terminated and resources are released

#### Scenario: Scanner process cleanup on timeout
- **WHEN** a scanner process is terminated due to timeout
- **THEN** system kills the process forcefully if graceful shutdown fails

#### Scenario: Scanner process cleanup on service shutdown
- **WHEN** the webhook service is shutting down and scanner processes are still running
- **THEN** system attempts graceful shutdown of running scans before exiting

### Requirement: Report scan results

The system SHALL make scan results available through logging and optional result storage.

#### Scenario: Scan result logged
- **WHEN** a scan completes
- **THEN** system logs the scan result with image reference, status, and summary

#### Scenario: Scan metrics exposed
- **WHEN** a scan completes
- **THEN** system updates metrics including scan count, duration, and success/failure rates

#### Scenario: Scan result cached
- **WHEN** a scan completes successfully
- **THEN** system optionally caches the result to avoid duplicate scans of the same image:tag within a time window

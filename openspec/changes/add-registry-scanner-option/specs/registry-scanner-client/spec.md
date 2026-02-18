## ADDED Requirements

### Requirement: Initialize Registry Scanner API client

The system SHALL initialize a Registry Scanner API client with the configured API endpoint, project ID, and authentication credentials.

#### Scenario: Client initialized successfully
- **WHEN** the service starts with Registry Scanner configured
- **THEN** system creates an API client with the Sysdig API URL, project ID, and API token

#### Scenario: Missing project ID
- **WHEN** Registry Scanner is configured but project ID is not provided
- **THEN** system logs an error and fails to initialize the client

#### Scenario: Invalid API endpoint
- **WHEN** Registry Scanner API URL is unreachable or invalid
- **THEN** system logs a warning and scan requests will fail with connection errors

### Requirement: Initiate image scan via API

The system SHALL send a scan request to the Registry Scanner API with the image reference and registry credentials.

#### Scenario: Scan initiated successfully
- **WHEN** an image is ready to be scanned using Registry Scanner
- **THEN** system POSTs to the scan endpoint with image reference, registry credentials, and receives a scan ID

#### Scenario: Public image scan initiated
- **WHEN** scanning a public image that requires no authentication
- **THEN** system sends scan request without registry credentials

#### Scenario: Private image scan initiated
- **WHEN** scanning a private registry image requiring authentication
- **THEN** system includes registry credentials (username/password or token) in the scan request

#### Scenario: API returns error
- **WHEN** the Registry Scanner API returns a 4xx or 5xx error
- **THEN** system logs the error response and marks the scan as failed

### Requirement: Poll for scan completion

The system SHALL poll the Registry Scanner API for scan status until the scan completes or times out.

#### Scenario: Scan completes successfully
- **WHEN** polling the scan status endpoint
- **THEN** system receives "completed" status and retrieves the scan results

#### Scenario: Scan fails during processing
- **WHEN** polling the scan status endpoint
- **THEN** system receives "failed" status with error details and marks the scan as failed

#### Scenario: Scan still in progress
- **WHEN** polling the scan status endpoint and scan is not yet complete
- **THEN** system waits for the configured poll interval (default 5s) before polling again

#### Scenario: Scan exceeds timeout
- **WHEN** polling for scan status and the overall timeout is exceeded
- **THEN** system stops polling and marks the scan as failed with timeout error

#### Scenario: Configurable poll interval
- **WHEN** scans typically take longer than expected
- **THEN** system allows configuration of the poll interval between 1-30 seconds

### Requirement: Parse Registry Scanner API responses

The system SHALL parse JSON responses from the Registry Scanner API and extract scan results.

#### Scenario: Successful scan result parsed
- **WHEN** Registry Scanner returns a completed scan with vulnerability data
- **THEN** system extracts vulnerability counts (critical, high, medium, low) and summary information

#### Scenario: Scan result with no vulnerabilities
- **WHEN** Registry Scanner returns a completed scan with zero vulnerabilities
- **THEN** system marks the scan as successful with no findings

#### Scenario: Scan result with policy violations
- **WHEN** Registry Scanner returns a completed scan with policy violations
- **THEN** system logs the policy violation details along with vulnerabilities

#### Scenario: Malformed API response
- **WHEN** Registry Scanner returns invalid or incomplete JSON
- **THEN** system logs the raw response and marks the scan as failed

### Requirement: Handle Registry Scanner API authentication

The system SHALL authenticate all Registry Scanner API requests using the Sysdig API token.

#### Scenario: API token included in requests
- **WHEN** making any request to the Registry Scanner API
- **THEN** system includes the Sysdig API token in the Authorization header as a Bearer token

#### Scenario: API token expired or invalid
- **WHEN** Registry Scanner API returns 401 Unauthorized
- **THEN** system logs authentication failure and marks the scan as failed

#### Scenario: API token missing
- **WHEN** scanner type is Registry Scanner but API token is not configured
- **THEN** system logs an error at initialization and prevents scan requests

### Requirement: Retry transient API errors

The system SHALL retry Registry Scanner API requests that fail due to transient network or server errors.

#### Scenario: Retry on 5xx server error
- **WHEN** Registry Scanner API returns a 5xx server error
- **THEN** system retries the request up to 3 times with exponential backoff

#### Scenario: Retry on network timeout
- **WHEN** a request to Registry Scanner API times out
- **THEN** system retries the request up to 3 times with exponential backoff

#### Scenario: No retry on 4xx client error
- **WHEN** Registry Scanner API returns a 4xx client error (except 429)
- **THEN** system does not retry and marks the scan as failed

#### Scenario: Retry on rate limit
- **WHEN** Registry Scanner API returns 429 Too Many Requests
- **THEN** system waits for the Retry-After duration (if provided) or uses exponential backoff

### Requirement: Configure Registry Scanner options

The system SHALL allow configuration of Registry Scanner-specific options including API URL, project ID, and TLS verification.

#### Scenario: API URL configured
- **WHEN** user specifies a custom Registry Scanner API URL in configuration
- **THEN** system uses the custom URL instead of the default Sysdig API URL

#### Scenario: TLS verification enabled
- **WHEN** TLS verification is enabled (default)
- **THEN** system validates the Registry Scanner API server certificate

#### Scenario: TLS verification disabled
- **WHEN** TLS verification is disabled in configuration (for testing/development)
- **THEN** system skips certificate validation but logs a security warning

#### Scenario: Per-registry project ID
- **WHEN** different registries should use different Sysdig projects
- **THEN** system allows specifying project ID per registry in configuration

### Requirement: Report Registry Scanner API metrics

The system SHALL track and report metrics specific to Registry Scanner API operations.

#### Scenario: API call duration tracked
- **WHEN** making requests to the Registry Scanner API
- **THEN** system measures and logs the duration of each API call

#### Scenario: Polling attempts counted
- **WHEN** polling for scan results
- **THEN** system tracks the number of poll attempts per scan

#### Scenario: API error rates tracked
- **WHEN** API calls fail
- **THEN** system increments error counters categorized by error type (auth, timeout, server error)

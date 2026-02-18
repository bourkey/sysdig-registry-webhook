# Webhook Receiver Capability

## Purpose

This capability handles incoming webhook requests from container registries, authenticates them, parses the payloads, and extracts image metadata for security scanning.

## Requirements

### Requirement: Accept webhook HTTP requests

The system SHALL expose an HTTP endpoint that accepts POST requests containing container registry webhook payloads.

#### Scenario: Valid webhook received
- **WHEN** a container registry sends a POST request to the webhook endpoint with a valid payload
- **THEN** system responds with HTTP 200 status code

#### Scenario: Invalid HTTP method
- **WHEN** a request is sent using GET, PUT, DELETE, or other non-POST method
- **THEN** system responds with HTTP 405 Method Not Allowed

#### Scenario: Missing or empty payload
- **WHEN** a POST request is received with no body or empty body
- **THEN** system responds with HTTP 400 Bad Request

### Requirement: Authenticate webhook requests

The system SHALL validate webhook authenticity using registry-provided authentication mechanisms before processing.

#### Scenario: Valid HMAC signature
- **WHEN** a webhook includes a valid HMAC signature in the header that matches the configured secret
- **THEN** system accepts the webhook for processing

#### Scenario: Invalid HMAC signature
- **WHEN** a webhook includes an HMAC signature that does not match the configured secret
- **THEN** system responds with HTTP 401 Unauthorized and logs the authentication failure

#### Scenario: Valid bearer token
- **WHEN** a webhook includes a valid bearer token in the Authorization header
- **THEN** system accepts the webhook for processing

#### Scenario: Missing authentication
- **WHEN** a webhook arrives without required authentication headers
- **THEN** system responds with HTTP 401 Unauthorized

### Requirement: Parse multiple registry formats

The system SHALL parse webhook payloads from different container registry providers and extract standardized image information.

#### Scenario: Docker Hub webhook parsed
- **WHEN** a Docker Hub webhook is received with repository and tag information
- **THEN** system extracts the image name, registry URL, and tag into a normalized format

#### Scenario: Harbor webhook parsed
- **WHEN** a Harbor webhook is received with push event data
- **THEN** system extracts the image name, registry URL, and tag into a normalized format

#### Scenario: GitLab Registry webhook parsed
- **WHEN** a GitLab Container Registry webhook is received
- **THEN** system extracts the image name, registry URL, and tag into a normalized format

#### Scenario: Unsupported registry format
- **WHEN** a webhook is received from an unsupported or unrecognized registry type
- **THEN** system responds with HTTP 400 Bad Request and logs the unsupported format

### Requirement: Extract image metadata

The system SHALL extract all relevant image metadata from webhook payloads needed for security scanning.

#### Scenario: Image reference extracted
- **WHEN** a webhook payload contains image information
- **THEN** system extracts the full image reference in the format registry/repository:tag

#### Scenario: Digest extracted when available
- **WHEN** a webhook payload includes an image digest (SHA256)
- **THEN** system extracts and includes the digest for scanning

#### Scenario: Multiple tags in single webhook
- **WHEN** a webhook indicates multiple tags were pushed for the same image
- **THEN** system extracts each tag as a separate scan request

### Requirement: Handle malformed requests gracefully

The system SHALL handle malformed webhook requests without crashing and provide useful error responses.

#### Scenario: Invalid JSON payload
- **WHEN** a webhook is received with malformed JSON in the body
- **THEN** system responds with HTTP 400 Bad Request and logs the parse error

#### Scenario: Missing required fields
- **WHEN** a webhook payload is missing critical fields like repository name or tag
- **THEN** system responds with HTTP 400 Bad Request indicating which fields are missing

#### Scenario: Oversized payload
- **WHEN** a webhook payload exceeds the configured maximum size limit
- **THEN** system responds with HTTP 413 Payload Too Large

### Requirement: Provide health and readiness endpoints

The system SHALL expose health check endpoints for monitoring and orchestration systems.

#### Scenario: Health check when healthy
- **WHEN** a GET request is sent to the /health endpoint and the service is running normally
- **THEN** system responds with HTTP 200 and status "healthy"

#### Scenario: Readiness check when ready
- **WHEN** a GET request is sent to the /ready endpoint and the service can accept webhooks
- **THEN** system responds with HTTP 200 and status "ready"

#### Scenario: Readiness check when not ready
- **WHEN** a GET request is sent to the /ready endpoint and the service is initializing or overloaded
- **THEN** system responds with HTTP 503 Service Unavailable

# Integration Tests

This directory contains integration tests for the Registry Webhook Scanner.

## Test Coverage

### End-to-End Webhook Flow (Task 20.2)
Tests the complete flow from webhook receipt to scan completion:
1. Webhook received and authenticated
2. Payload parsed for image information
3. Scan request queued
4. Worker processes scan
5. Sysdig CLI invoked
6. Results logged

### Authentication Failures (Task 20.4)
Tests various authentication failure scenarios:
- Missing HMAC signature
- Invalid HMAC signature
- Missing bearer token
- Invalid bearer token
- Wrong authentication type

### Queue Overflow (Task 20.5)
Tests queue capacity handling:
- Fill queue to capacity
- Attempt to enqueue additional request
- Verify HTTP 503 response
- Verify queue depth tracking

### Graceful Shutdown (Task 20.6)
Tests graceful shutdown with in-flight scans:
- Start scan processing
- Send SIGTERM signal
- Verify no new webhooks accepted
- Verify in-flight scans complete
- Verify clean shutdown

## Running Tests

### Prerequisites

```bash
# Install dependencies
go mod download

# Make mock CLI executable
chmod +x ../mocks/mock-sysdig-cli
```

### Run All Integration Tests

```bash
go test -tags=integration ./test/integration/... -v
```

### Run Specific Test

```bash
go test -tags=integration -run TestEndToEndWebhookFlow ./test/integration/... -v
```

## Mock Sysdig CLI

The `test/mocks/mock-sysdig-cli` script simulates the Sysdig CLI Scanner:

**Features:**
- Accepts image references
- Requires SYSDIG_API_TOKEN environment variable
- Returns mock scan results with vulnerabilities
- Simulates scan duration (0.5s)
- Exits with code 0 on success

**Usage:**
```bash
export SYSDIG_API_TOKEN=test-token
./test/mocks/mock-sysdig-cli nginx:latest --apiurl https://secure.sysdig.com
```

## Test Fixtures

Mock webhook payloads are in `test/fixtures/`:
- `dockerhub-webhook.json` - Docker Hub push event
- `harbor-webhook.json` - Harbor push artifact event
- `gitlab-webhook.json` - GitLab Registry push event

## Integration Test Checklist

- [x] Mock webhook payloads created
- [x] Mock Sysdig CLI implemented
- [ ] End-to-end webhook flow test
- [ ] Authentication failure tests
- [ ] Queue overflow handling test
- [ ] Graceful shutdown test

## Writing New Integration Tests

Example test structure:

```go
// +build integration

package integration

import (
	"testing"
	"net/http/httptest"
	// ...
)

func TestEndToEndWebhookFlow(t *testing.T) {
	// Setup: Create test config, server, queue, workers
	// Act: Send webhook request
	// Assert: Verify scan completed, results logged
}
```

## CI/CD Integration

Run integration tests in CI with:

```bash
# Fast unit tests (always run)
go test ./... -short

# Full test suite including integration (optional)
go test ./... -tags=integration
```

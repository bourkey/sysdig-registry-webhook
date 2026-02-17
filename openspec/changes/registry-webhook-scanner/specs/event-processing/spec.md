## ADDED Requirements

### Requirement: Queue scan requests

The system SHALL queue incoming scan requests from webhooks for asynchronous processing.

#### Scenario: Scan request enqueued
- **WHEN** a valid webhook is received and parsed successfully
- **THEN** system adds the scan request to the processing queue and returns HTTP 200 immediately

#### Scenario: Queue at capacity
- **WHEN** the processing queue is full and a new scan request arrives
- **THEN** system responds with HTTP 503 Service Unavailable with retry-after header

#### Scenario: Queue operations are thread-safe
- **WHEN** multiple webhooks arrive concurrently
- **THEN** system safely enqueues all requests without race conditions or data loss

### Requirement: Process events in order

The system SHALL process scan requests from the queue in a fair and predictable order.

#### Scenario: FIFO processing
- **WHEN** multiple scan requests are in the queue
- **THEN** system processes them in first-in-first-out order by default

#### Scenario: Priority processing optional
- **WHEN** configured with priority settings
- **THEN** system processes higher priority registries or images before lower priority ones

### Requirement: Implement retry logic for failures

The system SHALL automatically retry failed scan attempts with exponential backoff.

#### Scenario: Transient failure retried
- **WHEN** a scan fails due to a transient error (network timeout, temporary Sysdig backend issue)
- **THEN** system retries the scan after a delay with exponential backoff

#### Scenario: Maximum retry attempts reached
- **WHEN** a scan fails repeatedly and reaches the maximum retry count
- **THEN** system marks the scan as permanently failed and logs the error

#### Scenario: Non-retriable failures
- **WHEN** a scan fails due to a permanent error (invalid image reference, authentication failure)
- **THEN** system does not retry and immediately marks the scan as failed

#### Scenario: Retry delay increases exponentially
- **WHEN** a scan is retried multiple times
- **THEN** system increases the delay between retries exponentially (e.g., 1s, 2s, 4s, 8s)

### Requirement: Deduplicate scan requests

The system SHALL detect and handle duplicate scan requests for the same image within a time window.

#### Scenario: Duplicate within time window
- **WHEN** the same image:tag is scanned and a webhook for the same image arrives within N seconds
- **THEN** system skips the duplicate scan and logs the deduplication

#### Scenario: Duplicate outside time window
- **WHEN** the same image:tag is scanned again after the deduplication window expires
- **THEN** system processes the new scan request normally

#### Scenario: Digest-based deduplication
- **WHEN** webhooks include image digests
- **THEN** system uses the digest for more accurate deduplication (same digest = truly identical image)

### Requirement: Control concurrency

The system SHALL limit the number of concurrent scanner invocations to prevent resource exhaustion.

#### Scenario: Concurrency limit enforced
- **WHEN** the number of running scans reaches the configured limit
- **THEN** system queues additional scan requests until a running scan completes

#### Scenario: Concurrency configurable
- **WHEN** the service is deployed in different environments (local dev vs production cluster)
- **THEN** system allows concurrency limit to be configured via environment variable or config file

#### Scenario: Worker pool manages concurrency
- **WHEN** processing queued scans
- **THEN** system uses a worker pool pattern with a fixed number of workers

### Requirement: Handle service shutdown gracefully

The system SHALL gracefully shutdown when receiving termination signals, allowing in-flight scans to complete.

#### Scenario: Graceful shutdown on SIGTERM
- **WHEN** the service receives SIGTERM signal
- **THEN** system stops accepting new webhooks, completes in-flight scans, and exits cleanly

#### Scenario: Forced shutdown on SIGKILL
- **WHEN** the service receives SIGKILL or shutdown grace period expires
- **THEN** system terminates immediately, leaving queued items for next startup

#### Scenario: Drain queue before shutdown
- **WHEN** shutting down gracefully
- **THEN** system attempts to process as many queued items as possible within the grace period

### Requirement: Persist queue across restarts

The system SHALL optionally persist the queue to avoid losing scan requests during restarts.

#### Scenario: In-memory queue mode
- **WHEN** configured for in-memory queue (default)
- **THEN** system loses queued items on restart but starts quickly

#### Scenario: Persistent queue mode
- **WHEN** configured for persistent queue (Redis or database)
- **THEN** system restores queued items after restart

### Requirement: Monitor queue health

The system SHALL expose queue metrics and health information for monitoring.

#### Scenario: Queue depth metric
- **WHEN** monitoring systems query queue metrics
- **THEN** system reports the current number of items in the queue

#### Scenario: Processing rate metric
- **WHEN** monitoring systems query processing metrics
- **THEN** system reports scans processed per minute

#### Scenario: Queue age metric
- **WHEN** monitoring systems query queue metrics
- **THEN** system reports the age of the oldest item in the queue

#### Scenario: Alert on queue buildup
- **WHEN** the queue depth exceeds a threshold or oldest item age exceeds a threshold
- **THEN** system logs a warning for alerting systems to detect

### Requirement: Log event processing lifecycle

The system SHALL log key events throughout the scan request lifecycle for debugging and audit trails.

#### Scenario: Webhook received logged
- **WHEN** a webhook is received
- **THEN** system logs the webhook source, timestamp, and image reference

#### Scenario: Scan started logged
- **WHEN** a scan begins processing
- **THEN** system logs the image reference and scan start time

#### Scenario: Scan completed logged
- **WHEN** a scan completes (success or failure)
- **THEN** system logs the result, duration, and any error details

#### Scenario: Retry logged
- **WHEN** a scan is retried after failure
- **THEN** system logs the retry attempt number and reason for retry

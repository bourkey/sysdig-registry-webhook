package queue

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// RetryConfig holds retry logic configuration
type RetryConfig struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  time.Second,
		MaxBackoff:      time.Minute,
		BackoffMultiplier: 2.0,
	}
}

// RetryManager manages retry logic for failed scans
type RetryManager struct {
	config RetryConfig
	queue  *ScanQueue
	logger *logrus.Logger
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config RetryConfig, queue *ScanQueue, logger *logrus.Logger) *RetryManager {
	return &RetryManager{
		config: config,
		queue:  queue,
		logger: logger,
	}
}

// ShouldRetry determines if a scan should be retried based on the error and retry count
func (rm *RetryManager) ShouldRetry(req *models.ScanRequest, err error) bool {
	// Check if max retries exceeded
	if req.RetryCount >= rm.config.MaxRetries {
		rm.logger.WithFields(logrus.Fields{
			"image_ref":   req.ImageRef,
			"request_id":  req.RequestID,
			"retry_count": req.RetryCount,
		}).Warn("Max retries exceeded")
		return false
	}

	// Determine if error is retriable
	if !rm.isRetriableError(err) {
		rm.logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
			"error":      err.Error(),
		}).Debug("Non-retriable error, not retrying")
		return false
	}

	return true
}

// isRetriableError determines if an error should trigger a retry
func (rm *RetryManager) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()

	// List of retriable error patterns
	retriablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"network",
		"503",
		"502",
		"504",
	}

	for _, pattern := range retriablePatterns {
		if contains(errorMsg, pattern) {
			return true
		}
	}

	// List of non-retriable error patterns
	nonRetriablePatterns := []string{
		"not found",
		"404",
		"401",
		"403",
		"invalid",
		"authentication",
		"authorization",
	}

	for _, pattern := range nonRetriablePatterns {
		if contains(errorMsg, pattern) {
			return false
		}
	}

	// Default to retriable for unknown errors
	return true
}

// ScheduleRetry schedules a scan request for retry with exponential backoff
func (rm *RetryManager) ScheduleRetry(req *models.ScanRequest, err error) error {
	// Increment retry count
	req.RetryCount++

	// Calculate backoff duration
	backoff := rm.calculateBackoff(req.RetryCount)

	rm.logger.WithFields(logrus.Fields{
		"image_ref":   req.ImageRef,
		"request_id":  req.RequestID,
		"retry_count": req.RetryCount,
		"backoff":     backoff,
		"error":       err.Error(),
	}).Info("Scheduling scan retry")

	// Wait for backoff duration
	time.Sleep(backoff)

	// Re-enqueue the request
	ctx := time.Now().Add(30 * time.Second) // 30s timeout for enqueue
	if err := rm.queue.Enqueue(contextWithDeadline(ctx), req); err != nil {
		return fmt.Errorf("failed to re-enqueue scan: %w", err)
	}

	return nil
}

// calculateBackoff calculates the backoff duration using exponential backoff
func (rm *RetryManager) calculateBackoff(retryCount int) time.Duration {
	// Calculate exponential backoff: initialBackoff * (multiplier ^ retryCount)
	backoff := float64(rm.config.InitialBackoff)

	for i := 1; i < retryCount; i++ {
		backoff *= rm.config.BackoffMultiplier
	}

	duration := time.Duration(backoff)

	// Cap at max backoff
	if duration > rm.config.MaxBackoff {
		duration = rm.config.MaxBackoff
	}

	return duration
}

// GetBackoffDurations returns the backoff durations for each retry attempt
func (rm *RetryManager) GetBackoffDurations() []time.Duration {
	durations := make([]time.Duration, rm.config.MaxRetries)
	for i := 0; i < rm.config.MaxRetries; i++ {
		durations[i] = rm.calculateBackoff(i + 1)
	}
	return durations
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// contextWithDeadline creates a simple context with deadline
func contextWithDeadline(deadline time.Time) contextDeadline {
	return contextDeadline{deadline: deadline}
}

type contextDeadline struct {
	deadline time.Time
}

func (c contextDeadline) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (c contextDeadline) Done() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Until(c.deadline))
		close(ch)
	}()
	return ch
}

func (c contextDeadline) Err() error {
	if time.Now().After(c.deadline) {
		return fmt.Errorf("deadline exceeded")
	}
	return nil
}

func (c contextDeadline) Value(key interface{}) interface{} {
	return nil
}

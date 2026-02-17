package models

import (
	"time"
)

// ScanRequest represents a request to scan a container image
type ScanRequest struct {
	// Image reference (registry/repository:tag or registry/repository@digest)
	ImageRef string

	// Registry name from configuration
	RegistryName string

	// Image metadata
	Registry   string // Registry URL
	Repository string // Repository name
	Tag        string // Image tag
	Digest     string // Image digest (SHA256), if available

	// Retry tracking
	RetryCount int
	MaxRetries int

	// Timestamps
	ReceivedAt time.Time
	QueuedAt   time.Time

	// Request ID for tracing
	RequestID string
}

// ScanResult represents the result of a container image scan
type ScanResult struct {
	// Request information
	ImageRef   string
	RequestID  string

	// Scan outcome
	Status     ScanStatus
	ExitCode   int
	Output     string // Scanner stdout
	ErrorOutput string // Scanner stderr

	// Timing
	StartedAt  time.Time
	CompletedAt time.Time
	Duration   time.Duration

	// Error information
	Error string
}

// ScanStatus represents the status of a scan
type ScanStatus string

const (
	ScanStatusPending    ScanStatus = "pending"
	ScanStatusRunning    ScanStatus = "running"
	ScanStatusSuccess    ScanStatus = "success"
	ScanStatusFailed     ScanStatus = "failed"
	ScanStatusTimeout    ScanStatus = "timeout"
	ScanStatusRetrying   ScanStatus = "retrying"
)

// IsComplete returns true if the scan has reached a terminal state
func (s ScanStatus) IsComplete() bool {
	return s == ScanStatusSuccess || s == ScanStatusFailed || s == ScanStatusTimeout
}

// ShouldRetry determines if a scan failure should be retried
func (s ScanStatus) ShouldRetry() bool {
	return s == ScanStatusFailed || s == ScanStatusTimeout
}

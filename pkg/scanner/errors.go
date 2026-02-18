package scanner

import "fmt"

// Registry Scanner specific error types

// APIError represents an error from the Registry Scanner API
type APIError struct {
	StatusCode int
	Message    string
	Retriable  bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// IsRetriable returns true if this error should be retried
func (e *APIError) IsRetriable() bool {
	return e.Retriable
}

// ScanTimeoutError represents a scan timeout
type ScanTimeoutError struct {
	ScanID       string
	PollAttempts int
	Duration     string
}

func (e *ScanTimeoutError) Error() string {
	return fmt.Sprintf("scan timeout after %d poll attempts (scan_id: %s)", e.PollAttempts, e.ScanID)
}

// AuthenticationError represents an authentication failure
type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.Message)
}

// ConfigurationError represents a configuration validation error
type ConfigurationError struct {
	Field   string
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error for %s: %s", e.Field, e.Message)
}

// NetworkError represents a network connectivity error
type NetworkError struct {
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error with retriability determination
func NewAPIError(statusCode int, message string) *APIError {
	retriable := isRetriableStatusCode(statusCode)
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Retriable:  retriable,
	}
}

// IsRetriableError checks if an error should be retried
func IsRetriableError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsRetriable()
	}

	// Network errors are generally retriable
	if _, ok := err.(*NetworkError); ok {
		return true
	}

	// Timeout errors are not retriable (already timed out)
	if _, ok := err.(*ScanTimeoutError); ok {
		return false
	}

	// Authentication errors are not retriable
	if _, ok := err.(*AuthenticationError); ok {
		return false
	}

	// Configuration errors are not retriable
	if _, ok := err.(*ConfigurationError); ok {
		return false
	}

	// Default to not retriable
	return false
}

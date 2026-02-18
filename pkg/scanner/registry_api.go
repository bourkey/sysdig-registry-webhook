package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/pkg/metrics"
)

// API constants
const (
	DefaultAPIURL      = "https://secure.sysdig.com"
	ScanEndpoint       = "/api/scanning/v1/registry/scan"
	ScanStatusEndpoint = "/api/scanning/v1/registry/scan/%s"

	HeaderAuthorization = "Authorization"
	HeaderContentType   = "Content-Type"
	HeaderRetryAfter    = "Retry-After"

	ContentTypeJSON = "application/json"
)

// APIClient wraps HTTP client with retry logic and authentication
type APIClient struct {
	httpClient *http.Client
	logger     *logrus.Logger
	token      string
	maxRetries int
}

// NewAPIClient creates a new API client with retry support
func NewAPIClient(token string, verifyTLS bool, logger *logrus.Logger) *APIClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verifyTLS,
		},
	}

	return &APIClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		logger:     logger,
		token:      token,
		maxRetries: 3,
	}
}

// makeAPIRequest sends an HTTP request with authentication and retry logic
func (c *APIClient) makeAPIRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			c.logger.WithFields(logrus.Fields{
				"attempt": attempt,
				"backoff": backoff,
			}).Debug("Retrying API request")

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authentication header
		req.Header.Set(HeaderAuthorization, fmt.Sprintf("Bearer %s", sanitizeToken(c.token)))
		if body != nil {
			req.Header.Set(HeaderContentType, ContentTypeJSON)
		}

		// Log request (sanitized)
		c.logger.WithFields(logrus.Fields{
			"method":  method,
			"url":     url,
			"attempt": attempt + 1,
		}).Debug("Sending API request")

		// Time the API request
		startTime := time.Now()
		resp, err := c.httpClient.Do(req)
		duration := time.Since(startTime).Seconds()

		if err != nil {
			// Record error metrics
			metrics.RecordScannerAPIError("network_error", 0)
			lastErr = fmt.Errorf("request failed: %w", err)
			if isRetriable(err) && attempt < c.maxRetries {
				continue
			}
			return nil, lastErr
		}

		// Record API call duration
		endpoint := getEndpointName(url)
		metrics.RecordScannerAPIDuration(endpoint, resp.StatusCode, duration)

		// Check if response is retriable
		if isRetriableStatusCode(resp.StatusCode) && attempt < c.maxRetries {
			// Record retriable error
			metrics.RecordScannerAPIError("retriable_status", resp.StatusCode)

			// Handle rate limiting
			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfter := getRetryAfter(resp)
				c.logger.WithFields(logrus.Fields{
					"retry_after": retryAfter,
				}).Warn("Rate limited by API")

				select {
				case <-time.After(retryAfter):
				case <-ctx.Done():
					resp.Body.Close()
					return nil, ctx.Err()
				}
			}

			// Read and close body for retry
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("retriable status code: %d", resp.StatusCode)
			continue
		}

		// Record non-retriable errors (4xx except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			metrics.RecordScannerAPIError("client_error", resp.StatusCode)
		}

		// Success or non-retriable error
		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetriableStatusCode returns true for HTTP status codes that should be retried
func isRetriableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests,      // 429
		http.StatusInternalServerError,   // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		http.StatusGatewayTimeout:        // 504
		return true
	default:
		return false
	}
}

// isRetriable returns true for errors that should be retried
func isRetriable(err error) bool {
	// Network errors, timeouts, etc. are retriable
	// This is a simplified check - in production, you'd check specific error types
	return true
}

// getRetryAfter extracts the Retry-After header value
func getRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get(HeaderRetryAfter)
	if retryAfter == "" {
		return 5 * time.Second // Default
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date (not implemented for simplicity)
	return 5 * time.Second
}

// sanitizeToken returns a sanitized version of the token for logging
func sanitizeToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:2] + "***" + token[len(token)-2:]
}

// getEndpointName extracts a simplified endpoint name from URL for metrics
func getEndpointName(url string) string {
	// Simple extraction - just check if it's a scan or status endpoint
	if strings.Contains(url, "/scan/") {
		return "scan_status"
	}
	if strings.Contains(url, "/scan") {
		return "scan_initiate"
	}
	return "unknown"
}

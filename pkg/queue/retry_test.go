package queue

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

func TestRetryManager_ShouldRetry(t *testing.T) {
	config := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  time.Second,
		MaxBackoff:      time.Minute,
		BackoffMultiplier: 2.0,
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	queue := NewScanQueue(100, logger)
	rm := NewRetryManager(config, queue, logger)

	tests := []struct {
		name        string
		retryCount  int
		err         error
		wantRetry   bool
	}{
		{
			name:       "first failure, retriable error",
			retryCount: 0,
			err:        fmt.Errorf("connection timeout"),
			wantRetry:  true,
		},
		{
			name:       "max retries not exceeded",
			retryCount: 2,
			err:        fmt.Errorf("network error"),
			wantRetry:  true,
		},
		{
			name:       "max retries exceeded",
			retryCount: 3,
			err:        fmt.Errorf("timeout"),
			wantRetry:  false,
		},
		{
			name:       "non-retriable error - not found",
			retryCount: 0,
			err:        fmt.Errorf("image not found"),
			wantRetry:  false,
		},
		{
			name:       "non-retriable error - authentication",
			retryCount: 0,
			err:        fmt.Errorf("authentication failed"),
			wantRetry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.ScanRequest{
				ImageRef:   "test:latest",
				RetryCount: tt.retryCount,
			}

			got := rm.ShouldRetry(req, tt.err)

			if got != tt.wantRetry {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestRetryManager_calculateBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    time.Second,
		MaxBackoff:        time.Minute,
		BackoffMultiplier: 2.0,
	}

	logger := logrus.New()
	queue := NewScanQueue(100, logger)
	rm := NewRetryManager(config, queue, logger)

	tests := []struct {
		name       string
		retryCount int
		want       time.Duration
	}{
		{
			name:       "first retry",
			retryCount: 1,
			want:       time.Second,
		},
		{
			name:       "second retry",
			retryCount: 2,
			want:       2 * time.Second,
		},
		{
			name:       "third retry",
			retryCount: 3,
			want:       4 * time.Second,
		},
		{
			name:       "fourth retry",
			retryCount: 4,
			want:       8 * time.Second,
		},
		{
			name:       "exceeds max backoff",
			retryCount: 10,
			want:       time.Minute, // Capped at max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.calculateBackoff(tt.retryCount)

			if got != tt.want {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.retryCount, got, tt.want)
			}
		})
	}
}

func TestRetryManager_isRetriableError(t *testing.T) {
	config := DefaultRetryConfig()
	logger := logrus.New()
	queue := NewScanQueue(100, logger)
	rm := NewRetryManager(config, queue, logger)

	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{
			name: "timeout error",
			err:  fmt.Errorf("request timeout"),
			want: true,
		},
		{
			name: "connection refused",
			err:  fmt.Errorf("connection refused"),
			want: true,
		},
		{
			name: "503 error",
			err:  fmt.Errorf("HTTP 503 service unavailable"),
			want: true,
		},
		{
			name: "404 not found",
			err:  fmt.Errorf("404 not found"),
			want: false,
		},
		{
			name: "401 unauthorized",
			err:  fmt.Errorf("401 unauthorized"),
			want: false,
		},
		{
			name: "invalid image",
			err:  fmt.Errorf("invalid image reference"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.isRetriableError(tt.err)

			if got != tt.want {
				t.Errorf("isRetriableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

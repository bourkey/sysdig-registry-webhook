package scanner

import (
	"context"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// ScannerBackend defines the interface for different scanner implementations
type ScannerBackend interface {
	// Scan executes a scan for the given image and returns the result
	Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error)

	// Type returns the scanner type identifier ("cli" or "registry")
	Type() string

	// ValidateConfig validates that the scanner backend is properly configured
	ValidateConfig() error
}

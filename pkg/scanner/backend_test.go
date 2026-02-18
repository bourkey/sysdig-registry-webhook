package scanner

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// TestScannerBackendInterface verifies that all scanner implementations
// satisfy the ScannerBackend interface
func TestScannerBackendInterface(t *testing.T) {
	tests := []struct {
		name    string
		backend ScannerBackend
	}{
		{
			name: "CLIScanner implements ScannerBackend",
			backend: &CLIScanner{
				config: &config.Config{},
				logger: logrus.New(),
			},
		},
		{
			name: "RegistryScanner implements ScannerBackend",
			backend: &RegistryScanner{
				config: &config.Config{},
				logger: logrus.New(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify interface methods exist and have correct signatures
			var _ ScannerBackend = tt.backend

			// Test Type() returns non-empty string
			scannerType := tt.backend.Type()
			if scannerType == "" {
				t.Error("Type() returned empty string")
			}

			// Test ValidateConfig() is callable (result depends on config)
			_ = tt.backend.ValidateConfig()

			// Test Scan() is callable with proper signature
			_, _ = tt.backend.Scan(context.Background(), &models.ScanRequest{
				ImageRef:     "test/image:latest",
				RequestID:    "test-123",
				RegistryName: "test-registry",
			})
		})
	}
}

// TestScannerTypeIdentifiers verifies scanner type identifiers are correct
func TestScannerTypeIdentifiers(t *testing.T) {
	tests := []struct {
		name         string
		backend      ScannerBackend
		expectedType string
	}{
		{
			name: "CLIScanner type is 'cli'",
			backend: &CLIScanner{
				config: &config.Config{},
				logger: logrus.New(),
			},
			expectedType: "cli",
		},
		{
			name: "RegistryScanner type is 'registry'",
			backend: &RegistryScanner{
				config: &config.Config{},
				logger: logrus.New(),
			},
			expectedType: "registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.backend.Type()
			if got != tt.expectedType {
				t.Errorf("Type() = %v, want %v", got, tt.expectedType)
			}
		})
	}
}

// TestScannerValidateConfig tests configuration validation for each backend
func TestScannerValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		backend ScannerBackend
		wantErr bool
	}{
		{
			name: "CLIScanner with valid config",
			backend: &CLIScanner{
				config: &config.Config{
					Scanner: config.ScannerConfig{
						Type:    config.ScannerTypeCLI,
						CLIPath: "/usr/local/bin/sysdig-cli-scanner",
					},
				},
				logger: logrus.New(),
			},
			wantErr: false, // Will fail in actual test since binary doesn't exist, but validates interface
		},
		{
			name: "RegistryScanner with missing config",
			backend: &RegistryScanner{
				config: &config.Config{
					Scanner: config.ScannerConfig{
						Type: config.ScannerTypeRegistry,
						// RegistryScanner config missing
					},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "RegistryScanner with missing API URL",
			backend: &RegistryScanner{
				config: &config.Config{
					Scanner: config.ScannerConfig{
						Type: config.ScannerTypeRegistry,
						RegistryScanner: &config.RegistryScannerConfig{
							// APIURL missing
							ProjectID: "test-project",
						},
					},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "RegistryScanner with missing project ID",
			backend: &RegistryScanner{
				config: &config.Config{
					Scanner: config.ScannerConfig{
						Type: config.ScannerTypeRegistry,
						RegistryScanner: &config.RegistryScannerConfig{
							APIURL: "https://secure.sysdig.com",
							// ProjectID missing
						},
					},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "RegistryScanner with valid config",
			backend: &RegistryScanner{
				config: &config.Config{
					Scanner: config.ScannerConfig{
						Type:        config.ScannerTypeRegistry,
						SysdigToken: "test-token",
						RegistryScanner: &config.RegistryScannerConfig{
							APIURL:    "https://secure.sysdig.com",
							ProjectID: "test-project",
							VerifyTLS: true,
						},
					},
				},
				logger: logrus.New(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.backend.ValidateConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

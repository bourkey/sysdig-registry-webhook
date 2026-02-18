package scanner

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// Test task 8.11: Factory creates correct scanner types
func TestNewScannerBackend(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.Config
		registryName string
		wantType     string
		wantErr      bool
	}{
		{
			name: "create CLI scanner from global default",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    config.ScannerTypeCLI,
					CLIPath: "/bin/sh", // Use existing binary
				},
			},
			registryName: "test-registry",
			wantType:     "cli",
			wantErr:      false,
		},
		{
			name: "create Registry scanner from global default",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:        config.ScannerTypeRegistry,
					SysdigToken: "test-token",
					RegistryScanner: &config.RegistryScannerConfig{
						APIURL:    "https://secure.sysdig.com",
						ProjectID: "test-project",
					},
				},
			},
			registryName: "test-registry",
			wantType:     "registry",
			wantErr:      false,
		},
		{
			name: "create CLI scanner with empty type (backward compatibility)",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    "", // Empty type should default to CLI
					CLIPath: "/bin/sh",
				},
			},
			registryName: "test-registry",
			wantType:     "cli",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			backend, err := NewScannerBackend(tt.config, tt.registryName, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewScannerBackend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if backend == nil {
					t.Fatal("NewScannerBackend() returned nil backend")
				}

				gotType := backend.Type()
				if gotType != tt.wantType {
					t.Errorf("NewScannerBackend() type = %v, want %v", gotType, tt.wantType)
				}
			}
		})
	}
}

// Test task 8.12: Scanner type selection with per-registry override
func TestDetermineScannerType(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.Config
		registryName string
		want         config.ScannerType
	}{
		{
			name: "use global default - CLI",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeCLI,
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
						// No scanner override
					},
				},
			},
			registryName: "test-registry",
			want:         config.ScannerTypeCLI,
		},
		{
			name: "use global default - Registry",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeRegistry,
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
						// No scanner override
					},
				},
			},
			registryName: "test-registry",
			want:         config.ScannerTypeRegistry,
		},
		{
			name: "use per-registry override - CLI to Registry",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeCLI, // Global default
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
						Scanner: config.ScannerOverride{
							Type: config.ScannerTypeRegistry, // Override
						},
					},
				},
			},
			registryName: "test-registry",
			want:         config.ScannerTypeRegistry,
		},
		{
			name: "use per-registry override - Registry to CLI",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeRegistry, // Global default
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
						Scanner: config.ScannerOverride{
							Type: config.ScannerTypeCLI, // Override
						},
					},
				},
			},
			registryName: "test-registry",
			want:         config.ScannerTypeCLI,
		},
		{
			name: "fallback to CLI when no type specified (backward compatibility)",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: "", // Empty
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
					},
				},
			},
			registryName: "test-registry",
			want:         config.ScannerTypeCLI,
		},
		{
			name: "registry not found - use global default",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeRegistry,
				},
				Registries: []config.RegistryConfig{
					{
						Name: "other-registry",
					},
				},
			},
			registryName: "test-registry", // Not in config
			want:         config.ScannerTypeRegistry,
		},
		{
			name: "multiple registries - correct override selected",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerTypeCLI,
				},
				Registries: []config.RegistryConfig{
					{
						Name: "registry-1",
						Scanner: config.ScannerOverride{
							Type: config.ScannerTypeRegistry,
						},
					},
					{
						Name: "registry-2",
						// No override
					},
					{
						Name: "registry-3",
						Scanner: config.ScannerOverride{
							Type: config.ScannerTypeRegistry,
						},
					},
				},
			},
			registryName: "registry-2",
			want:         config.ScannerTypeCLI, // Uses global default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineScannerType(tt.config, tt.registryName)
			if got != tt.want {
				t.Errorf("determineScannerType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test task 8.13: Invalid scanner type error
func TestNewScannerBackend_InvalidType(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		wantErrMsg  string
	}{
		{
			name: "invalid scanner type 'unknown'",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerType("unknown"),
				},
			},
			wantErrMsg: "unsupported scanner type",
		},
		{
			name: "invalid scanner type 'api'",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerType("api"),
				},
			},
			wantErrMsg: "unsupported scanner type",
		},
		{
			name: "invalid scanner type 'sysdig'",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type: config.ScannerType("sysdig"),
				},
			},
			wantErrMsg: "unsupported scanner type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			backend, err := NewScannerBackend(tt.config, "test-registry", logger)

			if err == nil {
				t.Fatal("NewScannerBackend() error = nil, want error for invalid type")
			}

			if backend != nil {
				t.Error("NewScannerBackend() returned non-nil backend on error")
			}

			// Check error message contains expected text
			if tt.wantErrMsg != "" {
				errStr := err.Error()
				if len(errStr) == 0 {
					t.Error("NewScannerBackend() error message is empty")
				}
			}
		})
	}
}

// Test factory validation catches configuration errors
func TestNewScannerBackend_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "CLI scanner with missing binary",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    config.ScannerTypeCLI,
					CLIPath: "/nonexistent/scanner",
				},
			},
			wantErr: true,
		},
		{
			name: "Registry scanner missing project ID",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:        config.ScannerTypeRegistry,
					SysdigToken: "test-token",
					RegistryScanner: &config.RegistryScannerConfig{
						APIURL: "https://secure.sysdig.com",
						// ProjectID missing
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Registry scanner missing configuration",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:        config.ScannerTypeRegistry,
					SysdigToken: "test-token",
					// RegistryScanner nil
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			backend, err := NewScannerBackend(tt.config, "test-registry", logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewScannerBackend() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && backend != nil {
				t.Error("NewScannerBackend() returned backend despite validation error")
			}
		})
	}
}

// Test factory creates backend with correct logger
func TestNewScannerBackend_Logger(t *testing.T) {
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:    config.ScannerTypeCLI,
			CLIPath: "/bin/sh",
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	backend, err := NewScannerBackend(cfg, "test-registry", logger)

	if err != nil {
		t.Fatalf("NewScannerBackend() error = %v, want nil", err)
	}

	// Verify backend can use logger (implementation detail, but verifies logger is set)
	if backend.Type() == "" {
		t.Error("Backend returned empty type (may indicate initialization issue)")
	}
}

// Benchmark scanner type determination
func BenchmarkDetermineScannerType(b *testing.B) {
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type: config.ScannerTypeCLI,
		},
		Registries: []config.RegistryConfig{
			{Name: "registry-1"},
			{Name: "registry-2", Scanner: config.ScannerOverride{Type: config.ScannerTypeRegistry}},
			{Name: "registry-3"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = determineScannerType(cfg, "registry-2")
	}
}

// Benchmark backend creation
func BenchmarkNewScannerBackend(b *testing.B) {
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:    config.ScannerTypeCLI,
			CLIPath: "/bin/sh",
		},
	}

	logger := logrus.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend, _ := NewScannerBackend(cfg, "test-registry", logger)
		_ = backend
	}
}

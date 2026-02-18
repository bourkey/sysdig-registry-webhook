package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

func TestCLIScanner_Type(t *testing.T) {
	cfg := &config.Config{}
	scanner := NewCLIScanner(cfg, logrus.New())

	got := scanner.Type()
	want := "cli"

	if got != want {
		t.Errorf("Type() = %v, want %v", got, want)
	}
}

func TestCLIScanner_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config with existing CLI path",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    config.ScannerTypeCLI,
					CLIPath: "/bin/sh", // Use existing binary for test
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config with non-existent CLI path",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    config.ScannerTypeCLI,
					CLIPath: "/nonexistent/sysdig-cli-scanner",
				},
			},
			wantErr: true,
		},
		{
			name: "empty CLI path",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					Type:    config.ScannerTypeCLI,
					CLIPath: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewCLIScanner(tt.config, logrus.New())
			err := scanner.ValidateConfig()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCLIScanner_buildScanArgs(t *testing.T) {
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:        config.ScannerTypeCLI,
			SysdigToken: "test-token-12345",
		},
		Registries: []config.RegistryConfig{
			{
				Name: "test-registry",
				Scanner: config.ScannerOverride{
					Credentials: config.RegistryCredentials{
						Username: "testuser",
						Password: "testpass",
					},
				},
			},
		},
	}

	scanner := NewCLIScanner(cfg, logrus.New())

	tests := []struct {
		name        string
		req         *models.ScanRequest
		wantContain []string
	}{
		{
			name: "basic scan args",
			req: &models.ScanRequest{
				ImageRef:     "registry.example.com/myimage:v1.0.0",
				RequestID:    "req-123",
				RegistryName: "other-registry",
			},
			wantContain: []string{
				"registry.example.com/myimage:v1.0.0",
				"--apiurl",
			},
		},
		{
			name: "scan with registry credentials",
			req: &models.ScanRequest{
				ImageRef:     "registry.example.com/myimage:v1.0.0",
				RequestID:    "req-123",
				RegistryName: "test-registry",
			},
			wantContain: []string{
				"registry.example.com/myimage:v1.0.0",
				"--apiurl",
				"--registry-user",
				"testuser",
				"--registry-password",
				"testpass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := scanner.buildScanArgs(tt.req)

			for _, want := range tt.wantContain {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildScanArgs() missing expected arg %q, got args: %v", want, args)
				}
			}
		})
	}
}

func TestCLIScanner_getTimeout(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		req     *models.ScanRequest
		want    time.Duration
		wantErr bool
	}{
		{
			name: "use default timeout",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					DefaultTimeout: "300s",
				},
			},
			req: &models.ScanRequest{
				RegistryName: "test-registry",
			},
			want:    300 * time.Second,
			wantErr: false,
		},
		{
			name: "use registry-specific timeout",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					DefaultTimeout: "300s",
				},
				Registries: []config.RegistryConfig{
					{
						Name: "test-registry",
						Scanner: config.ScannerOverride{
							Timeout: "600s",
						},
					},
				},
			},
			req: &models.ScanRequest{
				RegistryName: "test-registry",
			},
			want:    600 * time.Second,
			wantErr: false,
		},
		{
			name: "invalid timeout format",
			config: &config.Config{
				Scanner: config.ScannerConfig{
					DefaultTimeout: "invalid",
				},
			},
			req: &models.ScanRequest{
				RegistryName: "test-registry",
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewCLIScanner(tt.config, logrus.New())
			got, err := scanner.getTimeout(tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("getTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("getTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCLIScanner_Scan_Timeout tests that scan respects timeout
func TestCLIScanner_Scan_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:           config.ScannerTypeCLI,
			CLIPath:        "/bin/sleep", // Use sleep to simulate long-running process
			DefaultTimeout: "1s",         // Short timeout
		},
	}

	scanner := NewCLIScanner(cfg, logrus.New())

	req := &models.ScanRequest{
		ImageRef:     "10", // Sleep for 10 seconds (will timeout)
		RequestID:    "timeout-test",
		RegistryName: "test",
	}

	ctx := context.Background()
	result, err := scanner.Scan(ctx, req)

	// Should timeout
	if err == nil {
		t.Error("Scan() expected timeout error, got nil")
	}

	if result == nil {
		t.Error("Scan() should return result even on error")
	}

	if result != nil && result.Status != models.ScanStatusFailed {
		t.Errorf("Scan() status = %v, want %v", result.Status, models.ScanStatusFailed)
	}
}

// TestCLIScanner_Scan_ContextCancellation tests that scan respects context cancellation
func TestCLIScanner_Scan_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping context cancellation test in short mode")
	}

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:           config.ScannerTypeCLI,
			CLIPath:        "/bin/sleep",
			DefaultTimeout: "60s",
		},
	}

	scanner := NewCLIScanner(cfg, logrus.New())

	req := &models.ScanRequest{
		ImageRef:     "30", // Sleep for 30 seconds
		RequestID:    "cancel-test",
		RegistryName: "test",
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	result, err := scanner.Scan(ctx, req)

	// Should be cancelled
	if err == nil {
		t.Error("Scan() expected context cancellation error, got nil")
	}

	if result != nil && result.Status != models.ScanStatusFailed {
		t.Errorf("Scan() status = %v, want %v on cancellation", result.Status, models.ScanStatusFailed)
	}
}

// TestNewCLIScanner tests scanner initialization
func TestNewCLIScanner(t *testing.T) {
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Type:    config.ScannerTypeCLI,
			CLIPath: "/usr/local/bin/sysdig-cli-scanner",
		},
	}

	logger := logrus.New()
	scanner := NewCLIScanner(cfg, logger)

	if scanner == nil {
		t.Fatal("NewCLIScanner() returned nil")
	}

	if scanner.config != cfg {
		t.Error("NewCLIScanner() did not set config correctly")
	}

	if scanner.logger != logger {
		t.Error("NewCLIScanner() did not set logger correctly")
	}

	if scanner.Type() != "cli" {
		t.Errorf("NewCLIScanner() Type() = %v, want 'cli'", scanner.Type())
	}
}

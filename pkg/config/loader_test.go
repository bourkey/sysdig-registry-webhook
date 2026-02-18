package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := `
server:
  port: 8080

registries:
  - name: test-registry
    type: harbor
    url: https://harbor.example.com
    auth:
      type: bearer
      secret: test-secret

scanner:
  sysdig_token: test-token
  default_timeout: 300s
`

	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify defaults were applied
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}

	// Verify registry was loaded
	if len(cfg.Registries) != 1 {
		t.Errorf("len(Registries) = %d, want 1", len(cfg.Registries))
	}

	if cfg.Registries[0].Name != "test-registry" {
		t.Errorf("Registry name = %s, want test-registry", cfg.Registries[0].Name)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	// Set environment variable
	os.Setenv("TEST_TOKEN", "my-secret-token")
	defer os.Unsetenv("TEST_TOKEN")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configWithEnv := `
registries:
  - name: test
    type: dockerhub
    auth:
      type: bearer
      secret: ${TEST_TOKEN}

scanner:
  sysdig_token: ${TEST_TOKEN}
`

	err := os.WriteFile(configPath, []byte(configWithEnv), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Registries[0].Auth.Secret != "my-secret-token" {
		t.Errorf("Auth.Secret = %s, want my-secret-token", cfg.Registries[0].Auth.Secret)
	}

	if cfg.Scanner.SysdigToken != "my-secret-token" {
		t.Errorf("Scanner.SysdigToken = %s, want my-secret-token", cfg.Scanner.SysdigToken)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: &Config{
				Registries: []RegistryConfig{
					{
						Name: "test",
						Type: "dockerhub",
						Auth: AuthConfig{Type: "hmac", Secret: "secret"},
					},
				},
				Scanner: ScannerConfig{
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr: false,
		},
		{
			name: "no registries",
			config: &Config{
				Scanner: ScannerConfig{
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
			},
			wantErr:     true,
			errContains: "at least one registry",
		},
		{
			name: "missing sysdig token",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					DefaultTimeout: "300s",
				},
			},
			wantErr:     true,
			errContains: "sysdig_token is required",
		},
		{
			name: "invalid registry type",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "invalid", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
			},
			wantErr:     true,
			errContains: "invalid registry type",
		},
		{
			name: "duplicate registry names",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
					{Name: "test", Type: "harbor", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
			},
			wantErr:     true,
			errContains: "duplicate registry name",
		},
		// Registry Scanner validation tests (task 8.14)
		{
			name: "valid Registry Scanner config",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           ScannerTypeRegistry,
					SysdigToken:    "token",
					DefaultTimeout: "300s",
					RegistryScanner: &RegistryScannerConfig{
						APIURL:       "https://secure.sysdig.com",
						ProjectID:    "test-project",
						VerifyTLS:    true,
						PollInterval: "5s",
					},
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr: false,
		},
		{
			name: "Registry Scanner missing config",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           ScannerTypeRegistry,
					SysdigToken:    "token",
					DefaultTimeout: "300s",
					// RegistryScanner nil
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr:     true,
			errContains: "registry_scanner configuration is required",
		},
		{
			name: "Registry Scanner missing project ID",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           ScannerTypeRegistry,
					SysdigToken:    "token",
					DefaultTimeout: "300s",
					RegistryScanner: &RegistryScannerConfig{
						APIURL:    "https://secure.sysdig.com",
						VerifyTLS: true,
						// ProjectID missing
					},
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr:     true,
			errContains: "project_id is required",
		},
		{
			name: "invalid scanner type",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           ScannerType("invalid"),
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr:     true,
			errContains: "scanner.type must be",
		},
		{
			name: "Registry Scanner with invalid poll interval",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           ScannerTypeRegistry,
					SysdigToken:    "token",
					DefaultTimeout: "300s",
					RegistryScanner: &RegistryScannerConfig{
						APIURL:       "https://secure.sysdig.com",
						ProjectID:    "test-project",
						PollInterval: "invalid",
					},
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr:     true,
			errContains: "invalid scanner.registry_scanner.poll_interval",
		},
		{
			name: "CLI Scanner defaults to CLI type when not specified",
			config: &Config{
				Registries: []RegistryConfig{
					{Name: "test", Type: "dockerhub", Auth: AuthConfig{Type: "none"}},
				},
				Scanner: ScannerConfig{
					Type:           "",  // Empty - should default to CLI
					SysdigToken:    "token",
					DefaultTimeout: "300s",
				},
				Server: ServerConfig{
					ReadTimeout:     "30s",
					WriteTimeout:    "30s",
					ShutdownTimeout: "30s",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults
			tt.config.applyDefaults()

			err := tt.config.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

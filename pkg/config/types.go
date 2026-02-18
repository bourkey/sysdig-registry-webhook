package config

import "time"

// ScannerType defines the type of scanner to use
type ScannerType string

const (
	ScannerTypeCLI      ScannerType = "cli"
	ScannerTypeRegistry ScannerType = "registry"
)

// Config represents the complete application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Registries []RegistryConfig `yaml:"registries"`
	Scanner    ScannerConfig    `yaml:"scanner"`
	Queue      QueueConfig      `yaml:"queue"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port            int    `yaml:"port"`
	ReadTimeout     string `yaml:"read_timeout"`
	WriteTimeout    string `yaml:"write_timeout"`
	MaxRequestSize  int64  `yaml:"max_request_size"`
	ShutdownTimeout string `yaml:"shutdown_timeout"`
}

// RegistryConfig defines settings for a single container registry
type RegistryConfig struct {
	Name    string          `yaml:"name"`
	Type    string          `yaml:"type"` // dockerhub, harbor, gitlab
	URL     string          `yaml:"url"`
	Auth    AuthConfig      `yaml:"auth"`
	Scanner ScannerOverride `yaml:"scanner,omitempty"`
}

// AuthConfig defines authentication settings for webhooks
type AuthConfig struct {
	Type   string `yaml:"type"`   // hmac or bearer
	Secret string `yaml:"secret"` // HMAC secret or bearer token
}

// ScannerOverride holds registry-specific scanner settings
type ScannerOverride struct {
	Type        ScannerType         `yaml:"type,omitempty"`
	Timeout     string              `yaml:"timeout,omitempty"`
	Credentials RegistryCredentials `yaml:"credentials,omitempty"`
}

// RegistryCredentials stores registry authentication for pulling images
type RegistryCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ScannerConfig holds Sysdig scanner settings
type ScannerConfig struct {
	Type            ScannerType            `yaml:"type"`
	SysdigToken     string                 `yaml:"sysdig_token"`
	CLIPath         string                 `yaml:"cli_path"`
	DefaultTimeout  string                 `yaml:"default_timeout"`
	MaxConcurrent   int                    `yaml:"max_concurrent"`
	RegistryScanner *RegistryScannerConfig `yaml:"registry_scanner,omitempty"`
}

// RegistryScannerConfig holds Sysdig Registry Scanner API settings
type RegistryScannerConfig struct {
	APIURL       string `yaml:"api_url"`
	ProjectID    string `yaml:"project_id"`
	VerifyTLS    bool   `yaml:"verify_tls"`
	PollInterval string `yaml:"poll_interval"`
}

// QueueConfig holds event queue settings
type QueueConfig struct {
	BufferSize int `yaml:"buffer_size"`
	Workers    int `yaml:"workers"`
}

// ParseDuration converts string duration to time.Duration
func (c *Config) ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

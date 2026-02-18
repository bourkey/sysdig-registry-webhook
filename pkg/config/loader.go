package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses the YAML configuration file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for unspecified configuration options
func (c *Config) applyDefaults() {
	// Server defaults
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == "" {
		c.Server.ReadTimeout = "30s"
	}
	if c.Server.WriteTimeout == "" {
		c.Server.WriteTimeout = "30s"
	}
	if c.Server.MaxRequestSize == 0 {
		c.Server.MaxRequestSize = 10 * 1024 * 1024 // 10MB
	}
	if c.Server.ShutdownTimeout == "" {
		c.Server.ShutdownTimeout = "30s"
	}

	// Scanner defaults
	if c.Scanner.Type == "" {
		c.Scanner.Type = ScannerTypeCLI
	}
	if c.Scanner.CLIPath == "" {
		c.Scanner.CLIPath = "/usr/local/bin/sysdig-cli-scanner"
	}
	if c.Scanner.DefaultTimeout == "" {
		c.Scanner.DefaultTimeout = "300s"
	}
	if c.Scanner.MaxConcurrent == 0 {
		c.Scanner.MaxConcurrent = 5
	}

	// Registry Scanner defaults
	if c.Scanner.Type == ScannerTypeRegistry && c.Scanner.RegistryScanner != nil {
		if c.Scanner.RegistryScanner.APIURL == "" {
			c.Scanner.RegistryScanner.APIURL = "https://secure.sysdig.com"
		}
		if c.Scanner.RegistryScanner.PollInterval == "" {
			c.Scanner.RegistryScanner.PollInterval = "5s"
		}
		// VerifyTLS defaults to true (zero value for bool is false, so we check if it was set)
		// Note: This is implicitly true unless explicitly set to false
	}

	// Queue defaults
	if c.Queue.BufferSize == 0 {
		c.Queue.BufferSize = 100
	}
	if c.Queue.Workers == 0 {
		c.Queue.Workers = 3
	}
}

// Validate checks the configuration for required fields and valid values
func (c *Config) Validate() error {
	// Validate registries
	if len(c.Registries) == 0 {
		return fmt.Errorf("at least one registry must be configured")
	}

	registryNames := make(map[string]bool)
	for i, reg := range c.Registries {
		if reg.Name == "" {
			return fmt.Errorf("registry[%d]: name is required", i)
		}
		if registryNames[reg.Name] {
			return fmt.Errorf("duplicate registry name: %s", reg.Name)
		}
		registryNames[reg.Name] = true

		if err := validateRegistryType(reg.Type); err != nil {
			return fmt.Errorf("registry[%s]: %w", reg.Name, err)
		}

		if err := validateAuthConfig(reg.Auth); err != nil {
			return fmt.Errorf("registry[%s]: %w", reg.Name, err)
		}
	}

	// Validate scanner config
	if c.Scanner.SysdigToken == "" {
		return fmt.Errorf("scanner.sysdig_token is required")
	}

	// Validate scanner type
	if c.Scanner.Type != ScannerTypeCLI && c.Scanner.Type != ScannerTypeRegistry {
		return fmt.Errorf("scanner.type must be 'cli' or 'registry', got: %s", c.Scanner.Type)
	}

	// Validate Registry Scanner config if type is registry
	if c.Scanner.Type == ScannerTypeRegistry {
		if c.Scanner.RegistryScanner == nil {
			return fmt.Errorf("scanner.registry_scanner configuration is required when scanner.type is 'registry'")
		}
		if c.Scanner.RegistryScanner.ProjectID == "" {
			return fmt.Errorf("scanner.registry_scanner.project_id is required when scanner.type is 'registry'")
		}
		if c.Scanner.RegistryScanner.PollInterval != "" {
			if _, err := c.ParseDuration(c.Scanner.RegistryScanner.PollInterval); err != nil {
				return fmt.Errorf("invalid scanner.registry_scanner.poll_interval: %w", err)
			}
		}
	}

	// Validate duration strings
	durations := map[string]string{
		"server.read_timeout":      c.Server.ReadTimeout,
		"server.write_timeout":     c.Server.WriteTimeout,
		"server.shutdown_timeout":  c.Server.ShutdownTimeout,
		"scanner.default_timeout":  c.Scanner.DefaultTimeout,
	}

	for name, value := range durations {
		if _, err := c.ParseDuration(value); err != nil {
			return fmt.Errorf("invalid %s: %w", name, err)
		}
	}

	return nil
}

func validateRegistryType(regType string) error {
	validTypes := []string{"dockerhub", "harbor", "gitlab"}
	for _, valid := range validTypes {
		if regType == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid registry type '%s', must be one of: %s",
		regType, strings.Join(validTypes, ", "))
}

func validateAuthConfig(auth AuthConfig) error {
	if auth.Type != "hmac" && auth.Type != "bearer" && auth.Type != "none" {
		return fmt.Errorf("invalid auth type '%s', must be 'hmac', 'bearer', or 'none'", auth.Type)
	}

	if (auth.Type == "hmac" || auth.Type == "bearer") && auth.Secret == "" {
		return fmt.Errorf("auth.secret is required when auth type is '%s'", auth.Type)
	}

	return nil
}

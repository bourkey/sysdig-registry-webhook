package scanner

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// CredentialProvider manages credentials for scanner and registry access
type CredentialProvider struct {
	config *config.Config
}

// NewCredentialProvider creates a new credential provider
func NewCredentialProvider(cfg *config.Config) *CredentialProvider {
	return &CredentialProvider{
		config: cfg,
	}
}

// InjectSysdigToken adds Sysdig API token to the command environment
func (cp *CredentialProvider) InjectSysdigToken(cmd *exec.Cmd) error {
	if cp.config.Scanner.SysdigToken == "" {
		return fmt.Errorf("Sysdig API token not configured")
	}

	// Add token as environment variable
	cmd.Env = append(cmd.Env, fmt.Sprintf("SYSDIG_API_TOKEN=%s", cp.config.Scanner.SysdigToken))

	return nil
}

// GetRegistryCredentials returns registry credentials for the given scan request
func (cp *CredentialProvider) GetRegistryCredentials(req *models.ScanRequest) (*RegistryCredentials, error) {
	// Find registry configuration
	for _, reg := range cp.config.Registries {
		if reg.Name == req.RegistryName {
			if reg.Scanner.Credentials.Username != "" && reg.Scanner.Credentials.Password != "" {
				return &RegistryCredentials{
					Username: reg.Scanner.Credentials.Username,
					Password: reg.Scanner.Credentials.Password,
					Registry: req.Registry,
				}, nil
			}
			// No credentials configured for this registry
			return nil, nil
		}
	}

	return nil, fmt.Errorf("registry not found: %s", req.RegistryName)
}

// InjectRegistryCredentials adds registry credentials to the command
func (cp *CredentialProvider) InjectRegistryCredentials(cmd *exec.Cmd, req *models.ScanRequest) error {
	creds, err := cp.GetRegistryCredentials(req)
	if err != nil {
		return err
	}

	// No credentials needed for public images
	if creds == nil {
		return nil
	}

	// Add registry credentials as environment variables
	// Format depends on the Sysdig CLI requirements
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("REGISTRY_USERNAME=%s", creds.Username),
		fmt.Sprintf("REGISTRY_PASSWORD=%s", creds.Password),
	)

	return nil
}

// ValidateCredentials checks if all required credentials are configured
func (cp *CredentialProvider) ValidateCredentials() error {
	// Validate Sysdig token
	if cp.config.Scanner.SysdigToken == "" {
		return fmt.Errorf("Sysdig API token is required (scanner.sysdig_token)")
	}

	// Validate it's not a placeholder
	if strings.HasPrefix(cp.config.Scanner.SysdigToken, "${") {
		return fmt.Errorf("Sysdig API token is not set (still contains placeholder)")
	}

	// Warn about registries without credentials
	for _, reg := range cp.config.Registries {
		if reg.Scanner.Credentials.Username == "" && reg.Scanner.Credentials.Password == "" {
			// This is okay for public registries, just a note
			continue
		}

		if reg.Scanner.Credentials.Username == "" || reg.Scanner.Credentials.Password == "" {
			return fmt.Errorf("registry %s has incomplete credentials (both username and password required)", reg.Name)
		}
	}

	return nil
}

// SanitizeForLogging returns a sanitized version of credentials for logging
// Replaces actual values with placeholders to prevent secret exposure
func (cp *CredentialProvider) SanitizeForLogging(value string) string {
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "****"
	}
	// Show first 2 and last 2 characters
	return fmt.Sprintf("%s...%s", value[:2], value[len(value)-2:])
}

// RegistryCredentials holds credentials for accessing a container registry
type RegistryCredentials struct {
	Username string
	Password string
	Registry string
}

// IsEmpty returns true if credentials are not set
func (rc *RegistryCredentials) IsEmpty() bool {
	return rc.Username == "" && rc.Password == ""
}

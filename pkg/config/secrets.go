package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSecretsFromFiles loads secrets from mounted Kubernetes Secret volumes
// Looks for files in the format: /secrets/<secret-name>
// Returns a map of secret names to their values
func LoadSecretsFromFiles(secretsDir string) (map[string]string, error) {
	secrets := make(map[string]string)

	// Check if secrets directory exists
	if _, err := os.Stat(secretsDir); os.IsNotExist(err) {
		// No secrets directory, return empty map
		return secrets, nil
	}

	// Read all files in the secrets directory
	files, err := os.ReadDir(secretsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Read secret file
		secretPath := filepath.Join(secretsDir, file.Name())
		content, err := os.ReadFile(secretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read secret file %s: %w", file.Name(), err)
		}

		// Store secret with trimmed content
		secrets[file.Name()] = strings.TrimSpace(string(content))
	}

	return secrets, nil
}

// InjectSecretsIntoConfig replaces ${FILE:<secret-name>} placeholders with secret values
func InjectSecretsIntoConfig(cfg *Config, secrets map[string]string) {
	// Inject secrets into registry auth
	for i := range cfg.Registries {
		cfg.Registries[i].Auth.Secret = resolveSecret(cfg.Registries[i].Auth.Secret, secrets)

		// Inject registry credentials
		if cfg.Registries[i].Scanner.Credentials.Username != "" {
			cfg.Registries[i].Scanner.Credentials.Username =
				resolveSecret(cfg.Registries[i].Scanner.Credentials.Username, secrets)
		}
		if cfg.Registries[i].Scanner.Credentials.Password != "" {
			cfg.Registries[i].Scanner.Credentials.Password =
				resolveSecret(cfg.Registries[i].Scanner.Credentials.Password, secrets)
		}
	}

	// Inject Sysdig token
	cfg.Scanner.SysdigToken = resolveSecret(cfg.Scanner.SysdigToken, secrets)
}

// resolveSecret replaces ${FILE:<secret-name>} with the secret value
// If not a file reference, returns the original value
func resolveSecret(value string, secrets map[string]string) string {
	prefix := "${FILE:"
	suffix := "}"

	if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix) {
		secretName := strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
		if secretValue, ok := secrets[secretName]; ok {
			return secretValue
		}
	}

	return value
}

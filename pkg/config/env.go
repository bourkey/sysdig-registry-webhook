package config

import (
	"os"
	"strconv"
)

// EnvConfig holds environment variable-based configuration
type EnvConfig struct {
	Port           int
	LogLevel       string
	ConfigFile     string
	SysdigCLIPath  string
}

// LoadFromEnv reads configuration from environment variables
func LoadFromEnv() *EnvConfig {
	env := &EnvConfig{
		Port:          getEnvAsInt("PORT", 8080),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		ConfigFile:    getEnv("CONFIG_FILE", "config.yaml"),
		SysdigCLIPath: getEnv("SYSDIG_CLI_PATH", "/usr/local/bin/sysdig-cli-scanner"),
	}

	return env
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

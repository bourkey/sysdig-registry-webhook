package scanner

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// NewScannerBackend creates the appropriate scanner backend based on configuration
func NewScannerBackend(cfg *config.Config, registryName string, logger *logrus.Logger) (ScannerBackend, error) {
	// Determine scanner type
	scannerType := determineScannerType(cfg, registryName)

	logger.WithFields(logrus.Fields{
		"registry":     registryName,
		"scanner_type": scannerType,
	}).Debug("Creating scanner backend")

	// Create appropriate backend
	var backend ScannerBackend

	switch scannerType {
	case config.ScannerTypeCLI:
		backend = NewCLIScanner(cfg, logger)

	case config.ScannerTypeRegistry:
		backend = NewRegistryScanner(cfg, logger)

	default:
		return nil, fmt.Errorf("unsupported scanner type: %s", scannerType)
	}

	// Validate configuration
	if err := backend.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("scanner validation failed for type %s: %w", scannerType, err)
	}

	logger.WithFields(logrus.Fields{
		"registry":     registryName,
		"scanner_type": backend.Type(),
	}).Info("Scanner backend created and validated")

	return backend, nil
}

// determineScannerType determines which scanner type to use based on configuration
// Priority: per-registry override → global default → "cli" (backward compatibility)
func determineScannerType(cfg *config.Config, registryName string) config.ScannerType {
	// Check for per-registry override
	for _, reg := range cfg.Registries {
		if reg.Name == registryName {
			if reg.Scanner.Type != "" {
				return reg.Scanner.Type
			}
			break
		}
	}

	// Use global default
	if cfg.Scanner.Type != "" {
		return cfg.Scanner.Type
	}

	// Default to CLI for backward compatibility
	return config.ScannerTypeCLI
}

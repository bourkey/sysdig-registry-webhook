package parsers

import (
	"fmt"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// ParserRegistry manages webhook parsers for different registry types
type ParserRegistry struct {
	parsers map[string]models.WebhookParser
}

// NewParserRegistry creates a new parser registry
func NewParserRegistry(cfg *config.Config) *ParserRegistry {
	registry := &ParserRegistry{
		parsers: make(map[string]models.WebhookParser),
	}

	// Register parsers based on configured registries
	for _, regConfig := range cfg.Registries {
		var parser models.WebhookParser

		switch regConfig.Type {
		case "dockerhub":
			parser = NewDockerHubParser()
		case "harbor":
			parser = NewHarborParser(regConfig.URL)
		case "gitlab":
			parser = NewGitLabParser(regConfig.URL)
		default:
			// Skip unknown registry types
			continue
		}

		registry.parsers[regConfig.Name] = parser
	}

	return registry
}

// GetParser returns the parser for the given registry name
func (r *ParserRegistry) GetParser(registryName string) (models.WebhookParser, error) {
	parser, ok := r.parsers[registryName]
	if !ok {
		return nil, fmt.Errorf("no parser found for registry: %s", registryName)
	}
	return parser, nil
}

// GetParserByType returns a parser for the given registry type
func (r *ParserRegistry) GetParserByType(registryType string) (models.WebhookParser, error) {
	for _, parser := range r.parsers {
		if parser.RegistryType() == registryType {
			return parser, nil
		}
	}
	return nil, fmt.Errorf("no parser found for registry type: %s", registryType)
}

// ListRegistries returns all registered registry names
func (r *ParserRegistry) ListRegistries() []string {
	names := make([]string, 0, len(r.parsers))
	for name := range r.parsers {
		names = append(names, name)
	}
	return names
}

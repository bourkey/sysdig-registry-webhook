package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// HarborParser parses Harbor webhook payloads
type HarborParser struct {
	registryURL string
}

// NewHarborParser creates a new Harbor parser
func NewHarborParser(registryURL string) *HarborParser {
	return &HarborParser{
		registryURL: registryURL,
	}
}

// RegistryType returns the registry type this parser handles
func (p *HarborParser) RegistryType() string {
	return "harbor"
}

// Validate checks if the webhook payload is valid
func (p *HarborParser) Validate(r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("invalid HTTP method: %s", r.Method)
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	return nil
}

// Parse extracts scan requests from Harbor webhook
func (p *HarborParser) Parse(r *http.Request) ([]*models.ScanRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var payload HarborWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Only process PUSH_ARTIFACT events
	if payload.Type != "PUSH_ARTIFACT" && payload.Type != "pushImage" {
		return nil, fmt.Errorf("unsupported event type: %s", payload.Type)
	}

	// Validate required fields
	if payload.EventData.Repository.Name == "" {
		return nil, fmt.Errorf("missing repository name")
	}

	// Extract registry host from URL
	registryHost := p.registryURL
	if registryHost == "" {
		registryHost = "harbor.local"
	}
	registryHost = strings.TrimPrefix(registryHost, "https://")
	registryHost = strings.TrimPrefix(registryHost, "http://")

	var scanRequests []*models.ScanRequest

	// Harbor can have multiple tags for a single push
	for _, tag := range payload.EventData.Resources {
		if tag.Tag == "" {
			continue
		}

		imageRef := fmt.Sprintf("%s/%s:%s",
			registryHost,
			payload.EventData.Repository.Name,
			tag.Tag)

		scanRequest := &models.ScanRequest{
			ImageRef:     imageRef,
			RegistryName: "harbor",
			Registry:     registryHost,
			Repository:   payload.EventData.Repository.Name,
			Tag:          tag.Tag,
			Digest:       tag.Digest,
			ReceivedAt:   time.Now(),
			RequestID:    generateRequestID(),
		}

		scanRequests = append(scanRequests, scanRequest)
	}

	if len(scanRequests) == 0 {
		return nil, fmt.Errorf("no valid tags found in webhook")
	}

	return scanRequests, nil
}

// HarborWebhook represents the Harbor webhook payload structure
type HarborWebhook struct {
	Type      string `json:"type"`
	OccurAt   int64  `json:"occur_at"`
	Operator  string `json:"operator"`
	EventData struct {
		Resources []struct {
			Digest      string `json:"digest"`
			Tag         string `json:"tag"`
			ResourceURL string `json:"resource_url"`
		} `json:"resources"`
		Repository struct {
			DateCreated  int64  `json:"date_created"`
			Name         string `json:"name"`
			Namespace    string `json:"namespace"`
			RepoFullName string `json:"repo_full_name"`
			RepoType     string `json:"repo_type"`
		} `json:"repository"`
	} `json:"event_data"`
}

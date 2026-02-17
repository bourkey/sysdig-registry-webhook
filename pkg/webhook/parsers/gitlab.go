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

// GitLabParser parses GitLab Container Registry webhook payloads
type GitLabParser struct {
	registryURL string
}

// NewGitLabParser creates a new GitLab parser
func NewGitLabParser(registryURL string) *GitLabParser {
	return &GitLabParser{
		registryURL: registryURL,
	}
}

// RegistryType returns the registry type this parser handles
func (p *GitLabParser) RegistryType() string {
	return "gitlab"
}

// Validate checks if the webhook payload is valid
func (p *GitLabParser) Validate(r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("invalid HTTP method: %s", r.Method)
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	return nil
}

// Parse extracts scan requests from GitLab webhook
func (p *GitLabParser) Parse(r *http.Request) ([]*models.ScanRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var payload GitLabWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Only process push events
	if payload.ObjectKind != "push" && payload.EventName != "push" {
		return nil, fmt.Errorf("unsupported event type: %s", payload.ObjectKind)
	}

	// Extract registry host from URL
	registryHost := p.registryURL
	if registryHost == "" {
		registryHost = "registry.gitlab.com"
	}
	registryHost = strings.TrimPrefix(registryHost, "https://")
	registryHost = strings.TrimPrefix(registryHost, "http://")

	var scanRequests []*models.ScanRequest

	// GitLab sends container registry events with project path
	// Image format: registry.gitlab.com/group/project/image:tag
	if payload.Project.PathWithNamespace != "" {
		// Extract tag from ref (refs/tags/v1.0 -> v1.0)
		tag := strings.TrimPrefix(payload.Ref, "refs/tags/")
		if tag == "" || strings.HasPrefix(tag, "refs/") {
			// Not a tag push, might be a branch push
			// Use commit SHA as tag
			tag = payload.After[:12] // Use short SHA
		}

		// Construct image reference
		imageRef := fmt.Sprintf("%s/%s:%s",
			registryHost,
			payload.Project.PathWithNamespace,
			tag)

		scanRequest := &models.ScanRequest{
			ImageRef:     imageRef,
			RegistryName: "gitlab",
			Registry:     registryHost,
			Repository:   payload.Project.PathWithNamespace,
			Tag:          tag,
			ReceivedAt:   time.Now(),
			RequestID:    generateRequestID(),
		}

		scanRequests = append(scanRequests, scanRequest)
	}

	if len(scanRequests) == 0 {
		return nil, fmt.Errorf("no valid image references found in webhook")
	}

	return scanRequests, nil
}

// GitLabWebhook represents the GitLab webhook payload structure
type GitLabWebhook struct {
	ObjectKind string `json:"object_kind"`
	EventName  string `json:"event_name"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Ref        string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	UserID     int    `json:"user_id"`
	UserName   string `json:"user_name"`
	UserEmail  string `json:"user_email"`
	Project    struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		WebURL            string `json:"web_url"`
		GitSSHURL         string `json:"git_ssh_url"`
		GitHTTPURL        string `json:"git_http_url"`
		Namespace         string `json:"namespace"`
		PathWithNamespace string `json:"path_with_namespace"`
		DefaultBranch     string `json:"default_branch"`
		Homepage          string `json:"homepage"`
		URL               string `json:"url"`
		SSHURL            string `json:"ssh_url"`
		HTTPURL           string `json:"http_url"`
	} `json:"project"`
	Repository struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Homepage    string `json:"homepage"`
	} `json:"repository"`
	Commits []struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		URL       string    `json:"url"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

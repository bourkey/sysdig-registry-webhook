package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// DockerHubParser parses Docker Hub webhook payloads
type DockerHubParser struct{}

// NewDockerHubParser creates a new Docker Hub parser
func NewDockerHubParser() *DockerHubParser {
	return &DockerHubParser{}
}

// RegistryType returns the registry type this parser handles
func (p *DockerHubParser) RegistryType() string {
	return "dockerhub"
}

// Validate checks if the webhook payload is valid
func (p *DockerHubParser) Validate(r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("invalid HTTP method: %s", r.Method)
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	return nil
}

// Parse extracts scan requests from Docker Hub webhook
func (p *DockerHubParser) Parse(r *http.Request) ([]*models.ScanRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var payload DockerHubWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if payload.Repository.RepoName == "" {
		return nil, fmt.Errorf("missing repository name")
	}

	if payload.PushData.Tag == "" {
		return nil, fmt.Errorf("missing tag")
	}

	// Create scan request
	scanRequest := &models.ScanRequest{
		ImageRef:     fmt.Sprintf("%s:%s", payload.Repository.RepoName, payload.PushData.Tag),
		RegistryName: "dockerhub",
		Registry:     "docker.io",
		Repository:   payload.Repository.RepoName,
		Tag:          payload.PushData.Tag,
		ReceivedAt:   time.Now(),
		RequestID:    generateRequestID(),
	}

	return []*models.ScanRequest{scanRequest}, nil
}

// DockerHubWebhook represents the Docker Hub webhook payload structure
type DockerHubWebhook struct {
	PushData struct {
		PushedAt int64    `json:"pushed_at"`
		Images   []string `json:"images"`
		Tag      string   `json:"tag"`
		Pusher   string   `json:"pusher"`
	} `json:"push_data"`
	CallbackURL string `json:"callback_url"`
	Repository  struct {
		Status          string `json:"status"`
		Description     string `json:"description"`
		IsTrusted       bool   `json:"is_trusted"`
		FullDescription string `json:"full_description"`
		RepoName        string `json:"repo_name"`
		RepoURL         string `json:"repo_url"`
		Owner           string `json:"owner"`
		IsOfficial      bool   `json:"is_official"`
		IsPrivate       bool   `json:"is_private"`
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		StarCount       int    `json:"star_count"`
		CommentCount    int    `json:"comment_count"`
		DateCreated     int64  `json:"date_created"`
		Dockerfile      string `json:"dockerfile"`
	} `json:"repository"`
}

package models

import (
	"net/http"
)

// WebhookParser defines the interface for parsing registry-specific webhooks
type WebhookParser interface {
	// Parse extracts scan requests from the webhook payload
	Parse(req *http.Request) ([]*ScanRequest, error)

	// Validate checks if the webhook payload is valid for this parser
	Validate(req *http.Request) error

	// RegistryType returns the registry type this parser handles
	RegistryType() string
}

// WebhookEvent represents a parsed webhook event
type WebhookEvent struct {
	// Event type (push, delete, etc.)
	EventType string

	// Images affected by this event
	Images []ImageInfo

	// Raw payload for logging
	RawPayload []byte

	// Registry information
	RegistryName string
	RegistryURL  string
}

// ImageInfo contains information about a container image from a webhook
type ImageInfo struct {
	Repository string
	Tag        string
	Digest     string
	PushedAt   string
}

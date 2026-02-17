package parsers

import (
	"fmt"
	"strings"
)

// NormalizeImageReference converts image references to a standard format
// Format: [registry/]repository:tag[@digest]
func NormalizeImageReference(registry, repository, tag, digest string) string {
	var ref string

	// Build base reference
	if registry != "" && registry != "docker.io" {
		ref = fmt.Sprintf("%s/%s", registry, repository)
	} else {
		ref = repository
	}

	// Add tag
	if tag != "" {
		ref = fmt.Sprintf("%s:%s", ref, tag)
	} else {
		ref = fmt.Sprintf("%s:latest", ref)
	}

	// Add digest if available
	if digest != "" {
		if !strings.HasPrefix(digest, "sha256:") {
			digest = "sha256:" + digest
		}
		ref = fmt.Sprintf("%s@%s", ref, digest)
	}

	return ref
}

// ParseImageReference parses an image reference string into components
func ParseImageReference(imageRef string) (registry, repository, tag, digest string) {
	// Handle digest
	if strings.Contains(imageRef, "@") {
		parts := strings.SplitN(imageRef, "@", 2)
		imageRef = parts[0]
		digest = parts[1]
	}

	// Handle tag
	if strings.Contains(imageRef, ":") {
		parts := strings.SplitN(imageRef, ":", 2)
		imageRef = parts[0]
		tag = parts[1]
	} else {
		tag = "latest"
	}

	// Handle registry and repository
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		// First part looks like a registry (has . or :)
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	} else {
		// No explicit registry, assume Docker Hub
		registry = "docker.io"
		repository = imageRef
	}

	return
}

// ValidateImageReference checks if an image reference is valid
func ValidateImageReference(imageRef string) error {
	if imageRef == "" {
		return fmt.Errorf("image reference is empty")
	}

	registry, repository, tag, _ := ParseImageReference(imageRef)

	if repository == "" {
		return fmt.Errorf("repository is empty")
	}

	if tag == "" {
		return fmt.Errorf("tag is empty")
	}

	if registry == "" {
		return fmt.Errorf("registry is empty")
	}

	return nil
}

package auth

import (
	"fmt"
	"net/http"
	"strings"
)

// VerifyBearerToken verifies the bearer token in the webhook request
func VerifyBearerToken(r *http.Request, expectedToken string) error {
	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	// Parse bearer token (format: "Bearer <token>")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Authorization header format")
	}

	scheme := parts[0]
	token := parts[1]

	// Verify scheme
	if !strings.EqualFold(scheme, "Bearer") {
		return fmt.Errorf("invalid authorization scheme: %s", scheme)
	}

	// Compare tokens (constant-time comparison to prevent timing attacks)
	if !constantTimeCompare(token, expectedToken) {
		return fmt.Errorf("invalid bearer token")
	}

	return nil
}

// constantTimeCompare performs a constant-time string comparison
func constantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	result := 0
	for i := 0; i < len(a); i++ {
		result |= int(a[i]) ^ int(b[i])
	}

	return result == 0
}

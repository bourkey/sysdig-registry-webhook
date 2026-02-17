package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// VerifyHMAC verifies the HMAC signature in the webhook request
func VerifyHMAC(r *http.Request, secret string) error {
	// Read the signature from the header
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		// Try alternative header names
		signature = r.Header.Get("X-Signature")
		if signature == "" {
			return fmt.Errorf("missing HMAC signature header")
		}
	}

	// Parse signature (format: "sha256=<hex>")
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid signature format")
	}

	algorithm := parts[0]
	providedSignature := parts[1]

	// Only support SHA256
	if algorithm != "sha256" {
		return fmt.Errorf("unsupported signature algorithm: %s", algorithm)
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore the body for later reading
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Compute HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(expectedSignature), []byte(providedSignature)) {
		return fmt.Errorf("HMAC signature mismatch")
	}

	return nil
}

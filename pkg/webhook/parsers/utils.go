package parsers

import (
	"crypto/rand"
	"encoding/hex"
)

// generateRequestID generates a unique request ID for tracing
func generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

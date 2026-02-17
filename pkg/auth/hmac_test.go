package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyHMAC(t *testing.T) {
	secret := "test-secret-key"
	payload := []byte(`{"test":"data"}`)

	tests := []struct {
		name        string
		payload     []byte
		secret      string
		setupHeader func(*http.Request, []byte, string)
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid HMAC signature",
			payload: payload,
			secret:  secret,
			setupHeader: func(r *http.Request, p []byte, s string) {
				mac := hmac.New(sha256.New, []byte(s))
				mac.Write(p)
				sig := hex.EncodeToString(mac.Sum(nil))
				r.Header.Set("X-Hub-Signature-256", "sha256="+sig)
			},
			wantErr: false,
		},
		{
			name:    "missing signature header",
			payload: payload,
			secret:  secret,
			setupHeader: func(r *http.Request, p []byte, s string) {
				// Don't set header
			},
			wantErr:     true,
			errContains: "missing HMAC signature",
		},
		{
			name:    "invalid signature format",
			payload: payload,
			secret:  secret,
			setupHeader: func(r *http.Request, p []byte, s string) {
				r.Header.Set("X-Hub-Signature-256", "invalid-format")
			},
			wantErr:     true,
			errContains: "invalid signature format",
		},
		{
			name:    "wrong signature",
			payload: payload,
			secret:  secret,
			setupHeader: func(r *http.Request, p []byte, s string) {
				r.Header.Set("X-Hub-Signature-256", "sha256=wrongsignature")
			},
			wantErr:     true,
			errContains: "signature mismatch",
		},
		{
			name:    "unsupported algorithm",
			payload: payload,
			secret:  secret,
			setupHeader: func(r *http.Request, p []byte, s string) {
				r.Header.Set("X-Hub-Signature-256", "md5=somehash")
			},
			wantErr:     true,
			errContains: "unsupported signature algorithm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(tt.payload))
			tt.setupHeader(req, tt.payload, tt.secret)

			err := VerifyHMAC(req, tt.secret)

			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyHMAC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("VerifyHMAC() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestVerifyHMAC_AlternativeHeader(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Signature", "sha256="+sig)

	err := VerifyHMAC(req, secret)
	if err != nil {
		t.Errorf("VerifyHMAC() with X-Signature header failed: %v", err)
	}
}

func TestVerifyHMAC_BodyReusable(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+sig)

	// Verify HMAC
	err := VerifyHMAC(req, secret)
	if err != nil {
		t.Fatalf("VerifyHMAC() failed: %v", err)
	}

	// Body should still be readable
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("Failed to read body after verification: %v", err)
	}

	if !bytes.Equal(body, payload) {
		t.Errorf("Body after verification = %s, want %s", body, payload)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

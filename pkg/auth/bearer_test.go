package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyBearerToken(t *testing.T) {
	validToken := "valid-test-token-12345"

	tests := []struct {
		name        string
		authHeader  string
		token       string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer " + validToken,
			token:      validToken,
			wantErr:    false,
		},
		{
			name:        "missing authorization header",
			authHeader:  "",
			token:       validToken,
			wantErr:     true,
			errContains: "missing Authorization header",
		},
		{
			name:        "invalid format - no space",
			authHeader:  "Bearer" + validToken,
			token:       validToken,
			wantErr:     true,
			errContains: "invalid Authorization header format",
		},
		{
			name:        "invalid scheme",
			authHeader:  "Basic " + validToken,
			token:       validToken,
			wantErr:     true,
			errContains: "invalid authorization scheme",
		},
		{
			name:        "wrong token",
			authHeader:  "Bearer wrong-token",
			token:       validToken,
			wantErr:     true,
			errContains: "invalid bearer token",
		},
		{
			name:        "case insensitive scheme",
			authHeader:  "bearer " + validToken,
			token:       validToken,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			err := VerifyBearerToken(req, tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("VerifyBearerToken() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "equal strings",
			a:    "test-token-123",
			b:    "test-token-123",
			want: true,
		},
		{
			name: "different strings same length",
			a:    "test-token-123",
			b:    "test-token-456",
			want: false,
		},
		{
			name: "different lengths",
			a:    "short",
			b:    "much-longer-string",
			want: false,
		},
		{
			name: "empty strings",
			a:    "",
			b:    "",
			want: true,
		},
		{
			name: "one empty",
			a:    "token",
			b:    "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constantTimeCompare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("constantTimeCompare(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

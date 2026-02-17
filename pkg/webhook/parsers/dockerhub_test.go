package parsers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockerHubParser_Parse(t *testing.T) {
	parser := NewDockerHubParser()

	tests := []struct {
		name        string
		payload     string
		wantCount   int
		wantRepo    string
		wantTag     string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid docker hub webhook",
			payload: `{
				"push_data": {
					"pushed_at": 1620000000,
					"tag": "v1.0.0",
					"pusher": "testuser"
				},
				"repository": {
					"repo_name": "myuser/myapp",
					"namespace": "myuser",
					"name": "myapp"
				}
			}`,
			wantCount: 1,
			wantRepo:  "myuser/myapp",
			wantTag:   "v1.0.0",
			wantErr:   false,
		},
		{
			name: "missing repository name",
			payload: `{
				"push_data": {
					"tag": "v1.0.0"
				},
				"repository": {
					"repo_name": ""
				}
			}`,
			wantErr:     true,
			errContains: "missing repository name",
		},
		{
			name: "missing tag",
			payload: `{
				"push_data": {
					"tag": ""
				},
				"repository": {
					"repo_name": "myuser/myapp"
				}
			}`,
			wantErr:     true,
			errContains: "missing tag",
		},
		{
			name:        "invalid JSON",
			payload:     `{invalid json}`,
			wantErr:     true,
			errContains: "failed to parse JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")

			requests, err := parser.Parse(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Parse() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if len(requests) != tt.wantCount {
				t.Errorf("Parse() returned %d requests, want %d", len(requests), tt.wantCount)
				return
			}

			if tt.wantCount > 0 {
				req := requests[0]
				if req.Repository != tt.wantRepo {
					t.Errorf("Parse() repository = %v, want %v", req.Repository, tt.wantRepo)
				}
				if req.Tag != tt.wantTag {
					t.Errorf("Parse() tag = %v, want %v", req.Tag, tt.wantTag)
				}
				if req.Registry != "docker.io" {
					t.Errorf("Parse() registry = %v, want docker.io", req.Registry)
				}
			}
		})
	}
}

func TestDockerHubParser_Validate(t *testing.T) {
	parser := NewDockerHubParser()

	tests := []struct {
		name        string
		method      string
		contentType string
		wantErr     bool
	}{
		{
			name:        "valid POST with JSON",
			method:      http.MethodPost,
			contentType: "application/json",
			wantErr:     false,
		},
		{
			name:        "invalid method GET",
			method:      http.MethodGet,
			contentType: "application/json",
			wantErr:     true,
		},
		{
			name:        "invalid content type",
			method:      http.MethodPost,
			contentType: "text/plain",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/webhook", nil)
			req.Header.Set("Content-Type", tt.contentType)

			err := parser.Validate(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDockerHubParser_RegistryType(t *testing.T) {
	parser := NewDockerHubParser()
	if got := parser.RegistryType(); got != "dockerhub" {
		t.Errorf("RegistryType() = %v, want dockerhub", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

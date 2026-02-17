package parsers

import (
	"testing"
)

func TestNormalizeImageReference(t *testing.T) {
	tests := []struct {
		name       string
		registry   string
		repository string
		tag        string
		digest     string
		want       string
	}{
		{
			name:       "full reference with tag",
			registry:   "harbor.example.com",
			repository: "myproject/myapp",
			tag:        "v1.0.0",
			digest:     "",
			want:       "harbor.example.com/myproject/myapp:v1.0.0",
		},
		{
			name:       "docker hub image",
			registry:   "docker.io",
			repository: "nginx",
			tag:        "latest",
			digest:     "",
			want:       "nginx:latest",
		},
		{
			name:       "with digest",
			registry:   "harbor.example.com",
			repository: "myapp",
			tag:        "v1.0.0",
			digest:     "sha256:abc123",
			want:       "harbor.example.com/myapp:v1.0.0@sha256:abc123",
		},
		{
			name:       "digest without sha256 prefix",
			registry:   "gcr.io",
			repository: "project/app",
			tag:        "latest",
			digest:     "abc123",
			want:       "gcr.io/project/app:latest@sha256:abc123",
		},
		{
			name:       "no tag defaults to latest",
			registry:   "harbor.example.com",
			repository: "myapp",
			tag:        "",
			digest:     "",
			want:       "harbor.example.com/myapp:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeImageReference(tt.registry, tt.repository, tt.tag, tt.digest)

			if got != tt.want {
				t.Errorf("NormalizeImageReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name           string
		imageRef       string
		wantRegistry   string
		wantRepository string
		wantTag        string
		wantDigest     string
	}{
		{
			name:           "full reference",
			imageRef:       "harbor.example.com/project/app:v1.0.0",
			wantRegistry:   "harbor.example.com",
			wantRepository: "project/app",
			wantTag:        "v1.0.0",
			wantDigest:     "",
		},
		{
			name:           "with digest",
			imageRef:       "harbor.example.com/app:latest@sha256:abc123",
			wantRegistry:   "harbor.example.com",
			wantRepository: "app",
			wantTag:        "latest",
			wantDigest:     "sha256:abc123",
		},
		{
			name:           "docker hub image",
			imageRef:       "nginx:latest",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "latest",
			wantDigest:     "",
		},
		{
			name:           "no tag defaults to latest",
			imageRef:       "nginx",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "latest",
			wantDigest:     "",
		},
		{
			name:           "registry with port",
			imageRef:       "localhost:5000/myapp:v1",
			wantRegistry:   "localhost:5000",
			wantRepository: "myapp",
			wantTag:        "v1",
			wantDigest:     "",
		},
		{
			name:           "multi-level repository",
			imageRef:       "gcr.io/project/subproject/app:latest",
			wantRegistry:   "gcr.io",
			wantRepository: "project/subproject/app",
			wantTag:        "latest",
			wantDigest:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRegistry, gotRepository, gotTag, gotDigest := ParseImageReference(tt.imageRef)

			if gotRegistry != tt.wantRegistry {
				t.Errorf("ParseImageReference() registry = %v, want %v", gotRegistry, tt.wantRegistry)
			}
			if gotRepository != tt.wantRepository {
				t.Errorf("ParseImageReference() repository = %v, want %v", gotRepository, tt.wantRepository)
			}
			if gotTag != tt.wantTag {
				t.Errorf("ParseImageReference() tag = %v, want %v", gotTag, tt.wantTag)
			}
			if gotDigest != tt.wantDigest {
				t.Errorf("ParseImageReference() digest = %v, want %v", gotDigest, tt.wantDigest)
			}
		})
	}
}

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name    string
		imageRef string
		wantErr bool
	}{
		{
			name:     "valid full reference",
			imageRef: "harbor.example.com/project/app:v1.0.0",
			wantErr:  false,
		},
		{
			name:     "valid docker hub",
			imageRef: "nginx:latest",
			wantErr:  false,
		},
		{
			name:     "empty reference",
			imageRef: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageReference(tt.imageRef)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageReference() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

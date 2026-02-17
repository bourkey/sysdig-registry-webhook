package queue

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

func TestDeduplicationCache_IsDuplicate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&discardWriter{})

	ttl := 100 * time.Millisecond
	cache := NewDeduplicationCache(ttl, logger)
	defer cache.Stop()

	req1 := &models.ScanRequest{
		ImageRef:  "nginx:latest",
		RequestID: "req-1",
	}

	req2 := &models.ScanRequest{
		ImageRef:  "nginx:latest",
		RequestID: "req-2",
	}

	req3 := &models.ScanRequest{
		ImageRef:  "redis:latest",
		RequestID: "req-3",
	}

	// First request - not a duplicate
	if cache.IsDuplicate(req1) {
		t.Error("First request should not be duplicate")
	}

	// Same image - should be duplicate
	if !cache.IsDuplicate(req2) {
		t.Error("Second request for same image should be duplicate")
	}

	// Different image - not a duplicate
	if cache.IsDuplicate(req3) {
		t.Error("Request for different image should not be duplicate")
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 50*time.Millisecond)

	// After TTL, should not be duplicate
	if cache.IsDuplicate(req1) {
		t.Error("After TTL expiry, request should not be duplicate")
	}
}

func TestDeduplicationCache_DigestBased(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&discardWriter{})

	cache := NewDeduplicationCache(time.Minute, logger)
	defer cache.Stop()

	req1 := &models.ScanRequest{
		ImageRef: "nginx:latest",
		Digest:   "sha256:abc123",
	}

	req2 := &models.ScanRequest{
		ImageRef: "nginx:v1.0",
		Digest:   "sha256:abc123", // Same digest, different tag
	}

	// First request
	if cache.IsDuplicate(req1) {
		t.Error("First request should not be duplicate")
	}

	// Same digest - should be duplicate even with different tag
	if !cache.IsDuplicate(req2) {
		t.Error("Request with same digest should be duplicate")
	}
}

func TestDeduplicationCache_Stats(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&discardWriter{})

	cache := NewDeduplicationCache(time.Minute, logger)
	defer cache.Stop()

	req1 := &models.ScanRequest{ImageRef: "nginx:latest"}
	req2 := &models.ScanRequest{ImageRef: "nginx:latest"}
	req3 := &models.ScanRequest{ImageRef: "redis:latest"}

	cache.IsDuplicate(req1) // Miss
	cache.IsDuplicate(req2) // Hit
	cache.IsDuplicate(req3) // Miss

	stats := cache.Stats()

	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}

	if stats.Misses != 2 {
		t.Errorf("Stats.Misses = %d, want 2", stats.Misses)
	}

	expectedHitRate := (1.0 / 3.0) * 100
	if stats.HitRate < expectedHitRate-1 || stats.HitRate > expectedHitRate+1 {
		t.Errorf("Stats.HitRate = %f, want ~%f", stats.HitRate, expectedHitRate)
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

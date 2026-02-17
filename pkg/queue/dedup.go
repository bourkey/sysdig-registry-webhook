package queue

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// DeduplicationCache prevents duplicate scan requests within a time window
type DeduplicationCache struct {
	cache      map[string]time.Time
	ttl        time.Duration
	mu         sync.RWMutex
	logger     *logrus.Logger
	stopChan   chan struct{}
	stopped    bool
	hitCount   int64
	missCount  int64
}

// NewDeduplicationCache creates a new deduplication cache
func NewDeduplicationCache(ttl time.Duration, logger *logrus.Logger) *DeduplicationCache {
	cache := &DeduplicationCache{
		cache:    make(map[string]time.Time),
		ttl:      ttl,
		logger:   logger,
		stopChan: make(chan struct{}),
		stopped:  false,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// IsDuplicate checks if a scan request is a duplicate
// Returns true if the request was seen within the TTL window
func (dc *DeduplicationCache) IsDuplicate(req *models.ScanRequest) bool {
	key := dc.generateKey(req)

	dc.mu.RLock()
	lastSeen, exists := dc.cache[key]
	dc.mu.RUnlock()

	if exists && time.Since(lastSeen) < dc.ttl {
		dc.hitCount++
		dc.logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
			"key":        key,
			"age":        time.Since(lastSeen),
		}).Debug("Duplicate scan request detected")
		return true
	}

	// Not a duplicate, mark as seen
	dc.mu.Lock()
	dc.cache[key] = time.Now()
	dc.mu.Unlock()

	dc.missCount++
	return false
}

// generateKey creates a deduplication key for a scan request
// Uses digest if available, otherwise image:tag
func (dc *DeduplicationCache) generateKey(req *models.ScanRequest) string {
	// Prefer digest-based deduplication (more accurate)
	if req.Digest != "" {
		return fmt.Sprintf("digest:%s", req.Digest)
	}

	// Fall back to image:tag
	// Hash the image ref to keep keys consistent length
	hash := sha256.Sum256([]byte(req.ImageRef))
	return fmt.Sprintf("ref:%x", hash[:16])
}

// cleanupLoop periodically removes expired entries from the cache
func (dc *DeduplicationCache) cleanupLoop() {
	ticker := time.NewTicker(dc.ttl / 2) // Cleanup at half the TTL interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dc.cleanup()
		case <-dc.stopChan:
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (dc *DeduplicationCache) cleanup() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	now := time.Now()
	expired := 0

	for key, lastSeen := range dc.cache {
		if now.Sub(lastSeen) > dc.ttl {
			delete(dc.cache, key)
			expired++
		}
	}

	if expired > 0 {
		dc.logger.WithFields(logrus.Fields{
			"expired":   expired,
			"remaining": len(dc.cache),
		}).Debug("Deduplication cache cleanup")
	}
}

// Stop stops the cleanup goroutine
func (dc *DeduplicationCache) Stop() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if !dc.stopped {
		dc.stopped = true
		close(dc.stopChan)
		dc.logger.Info("Deduplication cache stopped")
	}
}

// Stats returns cache statistics
func (dc *DeduplicationCache) Stats() DedupStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	total := dc.hitCount + dc.missCount
	var hitRate float64
	if total > 0 {
		hitRate = float64(dc.hitCount) / float64(total) * 100
	}

	return DedupStats{
		Size:      len(dc.cache),
		Hits:      dc.hitCount,
		Misses:    dc.missCount,
		HitRate:   hitRate,
		TTL:       dc.ttl,
	}
}

// DedupStats represents deduplication cache statistics
type DedupStats struct {
	Size      int
	Hits      int64
	Misses    int64
	HitRate   float64 // Percentage
	TTL       time.Duration
}

package scanner

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// ResultProcessor handles scan result processing and caching
type ResultProcessor struct {
	logger      *logrus.Logger
	cache       map[string]*CachedResult
	cacheTTL    time.Duration
	mu          sync.RWMutex
	metrics     *ResultMetrics
}

// NewResultProcessor creates a new result processor
func NewResultProcessor(cacheTTL time.Duration, logger *logrus.Logger) *ResultProcessor {
	return &ResultProcessor{
		logger:   logger,
		cache:    make(map[string]*CachedResult),
		cacheTTL: cacheTTL,
		metrics:  NewResultMetrics(),
	}
}

// ProcessResult processes and logs a scan result
func (rp *ResultProcessor) ProcessResult(result *models.ScanResult) error {
	// Parse scan output if JSON
	summary, err := rp.parseScanOutput(result.Output)
	if err != nil {
		rp.logger.WithFields(logrus.Fields{
			"image_ref":  result.ImageRef,
			"request_id": result.RequestID,
			"error":      err.Error(),
		}).Warn("Failed to parse scan output")
	}

	// Log result with structured fields
	rp.logResult(result, summary)

	// Update metrics
	rp.updateMetrics(result)

	// Cache result if successful
	if result.Status == models.ScanStatusSuccess {
		rp.cacheResult(result)
	}

	return nil
}

// parseScanOutput attempts to parse JSON scan output
func (rp *ResultProcessor) parseScanOutput(output string) (*ScanSummary, error) {
	if output == "" {
		return nil, fmt.Errorf("empty output")
	}

	// Try to parse as JSON
	var summary ScanSummary
	if err := json.Unmarshal([]byte(output), &summary); err != nil {
		// Not JSON, try to extract key information from text
		return rp.parseTextOutput(output), nil
	}

	return &summary, nil
}

// parseTextOutput extracts information from text-based scan output
func (rp *ResultProcessor) parseTextOutput(output string) *ScanSummary {
	summary := &ScanSummary{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "critical") {
			summary.Critical++
		} else if strings.Contains(lower, "high") {
			summary.High++
		} else if strings.Contains(lower, "medium") {
			summary.Medium++
		} else if strings.Contains(lower, "low") {
			summary.Low++
		}
	}

	return summary
}

// logResult logs the scan result with structured fields
func (rp *ResultProcessor) logResult(result *models.ScanResult, summary *ScanSummary) {
	fields := logrus.Fields{
		"image_ref":  result.ImageRef,
		"request_id": result.RequestID,
		"status":     result.Status,
		"duration":   result.Duration.Seconds(),
		"exit_code":  result.ExitCode,
	}

	if summary != nil {
		fields["vulnerabilities"] = map[string]int{
			"critical": summary.Critical,
			"high":     summary.High,
			"medium":   summary.Medium,
			"low":      summary.Low,
		}
		fields["total_vulnerabilities"] = summary.Total()
	}

	if result.Error != "" {
		fields["error"] = result.Error
	}

	// Log at appropriate level based on status
	switch result.Status {
	case models.ScanStatusSuccess:
		rp.logger.WithFields(fields).Info("Scan result processed")
	case models.ScanStatusFailed:
		rp.logger.WithFields(fields).Error("Scan failed")
	case models.ScanStatusTimeout:
		rp.logger.WithFields(fields).Warn("Scan timeout")
	default:
		rp.logger.WithFields(fields).Info("Scan result")
	}
}

// updateMetrics updates scan metrics
func (rp *ResultProcessor) updateMetrics(result *models.ScanResult) {
	rp.metrics.mu.Lock()
	defer rp.metrics.mu.Unlock()

	rp.metrics.TotalScans++

	switch result.Status {
	case models.ScanStatusSuccess:
		rp.metrics.SuccessfulScans++
	case models.ScanStatusFailed:
		rp.metrics.FailedScans++
	case models.ScanStatusTimeout:
		rp.metrics.TimeoutScans++
	}

	// Update duration stats
	duration := result.Duration.Seconds()
	if duration > rp.metrics.MaxDuration {
		rp.metrics.MaxDuration = duration
	}
	if rp.metrics.MinDuration == 0 || duration < rp.metrics.MinDuration {
		rp.metrics.MinDuration = duration
	}

	// Update average duration
	rp.metrics.AvgDuration = (rp.metrics.AvgDuration*float64(rp.metrics.TotalScans-1) + duration) / float64(rp.metrics.TotalScans)
}

// cacheResult stores the scan result in cache
func (rp *ResultProcessor) cacheResult(result *models.ScanResult) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.cache[result.ImageRef] = &CachedResult{
		Result:    result,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(rp.cacheTTL),
	}
}

// GetCachedResult retrieves a cached result if available and not expired
func (rp *ResultProcessor) GetCachedResult(imageRef string) (*models.ScanResult, bool) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	cached, ok := rp.cache[imageRef]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil, false
	}

	return cached.Result, true
}

// GetMetrics returns current scan metrics
func (rp *ResultProcessor) GetMetrics() ResultMetrics {
	rp.metrics.mu.RLock()
	defer rp.metrics.mu.RUnlock()

	return ResultMetrics{
		TotalScans:      rp.metrics.TotalScans,
		SuccessfulScans: rp.metrics.SuccessfulScans,
		FailedScans:     rp.metrics.FailedScans,
		TimeoutScans:    rp.metrics.TimeoutScans,
		AvgDuration:     rp.metrics.AvgDuration,
		MinDuration:     rp.metrics.MinDuration,
		MaxDuration:     rp.metrics.MaxDuration,
	}
}

// ScanSummary represents a summary of scan results
type ScanSummary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Total returns the total number of vulnerabilities
func (ss *ScanSummary) Total() int {
	return ss.Critical + ss.High + ss.Medium + ss.Low
}

// CachedResult represents a cached scan result
type CachedResult struct {
	Result    *models.ScanResult
	CachedAt  time.Time
	ExpiresAt time.Time
}

// ResultMetrics tracks scan result metrics
type ResultMetrics struct {
	TotalScans      int64
	SuccessfulScans int64
	FailedScans     int64
	TimeoutScans    int64
	AvgDuration     float64
	MinDuration     float64
	MaxDuration     float64
	mu              sync.RWMutex
}

// NewResultMetrics creates a new result metrics tracker
func NewResultMetrics() *ResultMetrics {
	return &ResultMetrics{}
}

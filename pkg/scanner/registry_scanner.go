package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
	"github.com/sysdig/registry-webhook-scanner/pkg/metrics"
)

// RegistryScanner implements the ScannerBackend interface using Sysdig Registry Scanner API
type RegistryScanner struct {
	config     *config.Config
	logger     *logrus.Logger
	httpClient *http.Client
}

// NewRegistryScanner creates a new RegistryScanner instance
func NewRegistryScanner(cfg *config.Config, logger *logrus.Logger) *RegistryScanner {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure TLS verification
	if cfg.Scanner.RegistryScanner != nil && !cfg.Scanner.RegistryScanner.VerifyTLS {
		// TODO: Add TLS skip verification if needed
		logger.Warn("TLS verification disabled for Registry Scanner - this is insecure!")
	}

	return &RegistryScanner{
		config:     cfg,
		logger:     logger,
		httpClient: httpClient,
	}
}

// Scan initiates a scan via Registry Scanner API and polls for results
func (s *RegistryScanner) Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error) {
	startTime := time.Now()

	result := &models.ScanResult{
		ImageRef:  req.ImageRef,
		RequestID: req.RequestID,
		Status:    models.ScanStatusRunning,
		StartedAt: startTime,
	}

	// Validate config before starting scan
	if s.config.Scanner.RegistryScanner == nil {
		result.Status = models.ScanStatusFailed
		result.Error = "registry scanner configuration is missing"
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(startTime)
		return result, fmt.Errorf("registry scanner configuration is missing")
	}

	s.logger.WithFields(logrus.Fields{
		"image_ref":    req.ImageRef,
		"request_id":   req.RequestID,
		"scanner_type": "registry",
	}).Info("Starting Registry Scanner API scan")

	// Step 1: Initiate scan
	scanID, err := s.initiateScan(ctx, req)
	if err != nil {
		result.Status = models.ScanStatusFailed
		result.Error = fmt.Sprintf("failed to initiate scan: %v", err)
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(startTime)

		// Record failure metrics
		metrics.RecordScannerType("registry", "failed")
		metrics.RecordScanDuration("registry", "failed", result.Duration.Seconds())
		metrics.RecordScan("registry", req.RegistryName, "failed")

		return result, err
	}

	s.logger.WithFields(logrus.Fields{
		"image_ref":  req.ImageRef,
		"request_id": req.RequestID,
		"scan_id":    scanID,
	}).Info("Scan initiated successfully")

	// Step 2: Poll for scan completion
	scanResult, err := s.pollScanStatus(ctx, scanID, req)
	if err != nil {
		result.Status = models.ScanStatusFailed
		result.Error = fmt.Sprintf("scan polling failed: %v", err)
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(startTime)

		// Record failure metrics
		metrics.RecordScannerType("registry", "failed")
		metrics.RecordScanDuration("registry", "failed", result.Duration.Seconds())
		metrics.RecordScan("registry", req.RegistryName, "failed")

		return result, err
	}

	// Step 3: Parse and return results
	scanResult.ImageRef = req.ImageRef
	scanResult.RequestID = req.RequestID
	scanResult.StartedAt = startTime
	scanResult.CompletedAt = time.Now()
	scanResult.Duration = scanResult.CompletedAt.Sub(startTime)
	scanResult.Status = models.ScanStatusSuccess // Map "completed" to our status type

	s.logger.WithFields(logrus.Fields{
		"image_ref":  req.ImageRef,
		"request_id": req.RequestID,
		"scan_id":    scanID,
		"duration":   scanResult.Duration,
	}).Info("Registry Scanner scan completed successfully")

	// Record metrics
	metrics.RecordScannerType("registry", "success")
	metrics.RecordScanDuration("registry", "success", scanResult.Duration.Seconds())
	metrics.RecordScan("registry", req.RegistryName, "success")

	return scanResult, nil
}

// Type returns the scanner type identifier
func (s *RegistryScanner) Type() string {
	return "registry"
}

// ValidateConfig validates that Registry Scanner is properly configured
func (s *RegistryScanner) ValidateConfig() error {
	if s.config.Scanner.RegistryScanner == nil {
		return fmt.Errorf("registry scanner configuration is missing")
	}

	if s.config.Scanner.RegistryScanner.APIURL == "" {
		return fmt.Errorf("registry scanner API URL is required")
	}

	if s.config.Scanner.RegistryScanner.ProjectID == "" {
		return fmt.Errorf("registry scanner project ID is required")
	}

	if s.config.Scanner.SysdigToken == "" {
		return fmt.Errorf("sysdig API token is required")
	}

	return nil
}

// initiateScan sends a POST request to initiate a scan and returns the scan ID
func (s *RegistryScanner) initiateScan(ctx context.Context, req *models.ScanRequest) (string, error) {
	apiURL := s.config.Scanner.RegistryScanner.APIURL
	endpoint := fmt.Sprintf("%s/api/scanning/v1/registry/scan", apiURL)

	// Build scan request payload
	payload, err := s.buildScanRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to build scan request: %w", err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.Scanner.SysdigToken))

	// Log API request (sanitized)
	s.logger.WithFields(logrus.Fields{
		"endpoint":     endpoint,
		"method":       "POST",
		"scanner_type": "registry",
	}).Debug("Sending Registry Scanner API request")

	startTime := time.Now()

	// Send request
	resp, err := s.httpClient.Do(httpReq)
	duration := time.Since(startTime)

	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"endpoint":     endpoint,
			"duration_ms":  duration.Milliseconds(),
			"error":        err.Error(),
			"scanner_type": "registry",
		}).Error("Registry Scanner API request failed")
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log API response
	s.logger.WithFields(logrus.Fields{
		"endpoint":     endpoint,
		"status_code":  resp.StatusCode,
		"duration_ms":  duration.Milliseconds(),
		"scanner_type": "registry",
	}).Debug("Received Registry Scanner API response")

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get scan ID
	var scanResp struct {
		ScanID string `json:"scan_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&scanResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if scanResp.ScanID == "" {
		return "", fmt.Errorf("scan ID not found in response")
	}

	return scanResp.ScanID, nil
}

// pollScanStatus polls the Registry Scanner API until the scan completes or times out
func (s *RegistryScanner) pollScanStatus(ctx context.Context, scanID string, req *models.ScanRequest) (*models.ScanResult, error) {
	pollInterval, err := time.ParseDuration(s.config.Scanner.RegistryScanner.PollInterval)
	if err != nil {
		pollInterval = 5 * time.Second
	}

	// Get timeout
	timeout, err := s.getTimeout(req)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	pollAttempts := 0

	for {
		select {
		case <-timeoutCtx.Done():
			s.logger.WithFields(logrus.Fields{
				"scan_id":       scanID,
				"poll_attempts": pollAttempts,
			}).Warn("Scan timeout during polling")
			return nil, fmt.Errorf("scan timeout after %d poll attempts", pollAttempts)

		case <-ticker.C:
			pollAttempts++

			result, err := s.getScanResult(timeoutCtx, scanID)
			if err != nil {
				return nil, fmt.Errorf("failed to get scan result: %w", err)
			}

			// Check scan status
			switch result.Status {
			case "completed":
				s.logger.WithFields(logrus.Fields{
					"scan_id":       scanID,
					"poll_attempts": pollAttempts,
				}).Info("Scan completed")

				// Record poll attempts metric
				metrics.RecordScannerPollAttempts(pollAttempts)

				return result, nil

			case "failed":
				return nil, fmt.Errorf("scan failed: %s", result.Error)

			case "running", "pending":
				// Continue polling
				s.logger.WithFields(logrus.Fields{
					"scan_id":       scanID,
					"poll_attempts": pollAttempts,
					"status":        result.Status,
				}).Debug("Scan still in progress")

			default:
				return nil, fmt.Errorf("unknown scan status: %s", result.Status)
			}
		}
	}
}

// getScanResult retrieves the current scan result from the API
func (s *RegistryScanner) getScanResult(ctx context.Context, scanID string) (*models.ScanResult, error) {
	apiURL := s.config.Scanner.RegistryScanner.APIURL
	endpoint := fmt.Sprintf("%s/api/scanning/v1/registry/scan/%s", apiURL, scanID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.Scanner.SysdigToken))

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	result, err := s.parseScanResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse scan response: %w", err)
	}

	return result, nil
}

// buildScanRequest constructs the scan request payload
func (s *RegistryScanner) buildScanRequest(req *models.ScanRequest) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"image": req.ImageRef,
	}

	// Add project ID
	if s.config.Scanner.RegistryScanner.ProjectID != "" {
		payload["project_id"] = s.config.Scanner.RegistryScanner.ProjectID
	}

	// Add registry credentials if available
	for _, reg := range s.config.Registries {
		if reg.Name == req.RegistryName && reg.Scanner.Credentials.Username != "" {
			payload["registry_credentials"] = map[string]string{
				"username": reg.Scanner.Credentials.Username,
				"password": reg.Scanner.Credentials.Password,
			}
			break
		}
	}

	return payload, nil
}

// parseScanResponse parses the scan result JSON into a ScanResult model
func (s *RegistryScanner) parseScanResponse(body io.Reader) (*models.ScanResult, error) {
	var apiResp struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
		Result struct {
			Vulnerabilities struct {
				Critical int `json:"critical"`
				High     int `json:"high"`
				Medium   int `json:"medium"`
				Low      int `json:"low"`
			} `json:"vulnerabilities"`
		} `json:"result,omitempty"`
	}

	if err := json.NewDecoder(body).Decode(&apiResp); err != nil {
		return nil, err
	}

	result := &models.ScanResult{
		Status: models.ScanStatus(apiResp.Status),
		Error:  apiResp.Error,
	}

	return result, nil
}

// getTimeout returns the timeout duration for a scan request
func (s *RegistryScanner) getTimeout(req *models.ScanRequest) (time.Duration, error) {
	// Check for registry-specific timeout
	for _, reg := range s.config.Registries {
		if reg.Name == req.RegistryName && reg.Scanner.Timeout != "" {
			return time.ParseDuration(reg.Scanner.Timeout)
		}
	}

	// Use default timeout
	return time.ParseDuration(s.config.Scanner.DefaultTimeout)
}

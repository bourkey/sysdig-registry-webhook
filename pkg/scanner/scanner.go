package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// Scanner wraps the Sysdig CLI Scanner
type Scanner struct {
	config *config.Config
	logger *logrus.Logger
}

// NewScanner creates a new Scanner instance
func NewScanner(cfg *config.Config, logger *logrus.Logger) *Scanner {
	return &Scanner{
		config: cfg,
		logger: logger,
	}
}

// Scan executes the Sysdig CLI Scanner for the given image
func (s *Scanner) Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error) {
	startTime := time.Now()

	result := &models.ScanResult{
		ImageRef:  req.ImageRef,
		RequestID: req.RequestID,
		Status:    models.ScanStatusRunning,
		StartedAt: startTime,
	}

	s.logger.WithFields(logrus.Fields{
		"image_ref":  req.ImageRef,
		"request_id": req.RequestID,
	}).Info("Starting image scan")

	// Build scanner command
	cmd, err := s.buildCommand(ctx, req)
	if err != nil {
		result.Status = models.ScanStatusFailed
		result.Error = fmt.Sprintf("failed to build command: %v", err)
		return result, err
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute scanner with timeout
	err = s.executeWithTimeout(ctx, cmd, req)

	// Capture output
	result.Output = stdout.String()
	result.ErrorOutput = stderr.String()
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startTime)

	// Handle execution result
	if err != nil {
		result.ExitCode = s.getExitCode(err)

		// Check if timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = models.ScanStatusTimeout
			result.Error = "scan timeout exceeded"
			s.logger.WithFields(logrus.Fields{
				"image_ref":  req.ImageRef,
				"request_id": req.RequestID,
				"duration":   result.Duration,
			}).Warn("Scan timeout")
			return result, fmt.Errorf("scan timeout")
		}

		// Check if exit code indicates vulnerabilities found (not an error)
		if result.ExitCode > 0 && result.ExitCode < 10 {
			// Sysdig CLI returns non-zero for vulnerabilities, but scan succeeded
			result.Status = models.ScanStatusSuccess
			s.logger.WithFields(logrus.Fields{
				"image_ref":  req.ImageRef,
				"request_id": req.RequestID,
				"exit_code":  result.ExitCode,
				"duration":   result.Duration,
			}).Info("Scan completed with vulnerabilities found")
			return result, nil
		}

		// Actual error
		result.Status = models.ScanStatusFailed
		result.Error = err.Error()
		s.logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
			"error":      err.Error(),
			"exit_code":  result.ExitCode,
		}).Error("Scan failed")
		return result, err
	}

	// Success
	result.Status = models.ScanStatusSuccess
	result.ExitCode = 0
	s.logger.WithFields(logrus.Fields{
		"image_ref":  req.ImageRef,
		"request_id": req.RequestID,
		"duration":   result.Duration,
	}).Info("Scan completed successfully")

	return result, nil
}

// buildCommand constructs the Sysdig CLI scanner command
func (s *Scanner) buildCommand(ctx context.Context, req *models.ScanRequest) (*exec.Cmd, error) {
	args := []string{
		req.ImageRef,
		"--apiurl", "https://secure.sysdig.com",
	}

	// Add JSON output for easier parsing
	args = append(args, "--json-scan-result", "/dev/stdout")

	// Create command
	cmd := exec.CommandContext(ctx, s.config.Scanner.CLIPath, args...)

	// Set environment variables for authentication
	cmd.Env = append(cmd.Env, fmt.Sprintf("SYSDIG_API_TOKEN=%s", s.config.Scanner.SysdigToken))

	return cmd, nil
}

// executeWithTimeout executes the command with a timeout
func (s *Scanner) executeWithTimeout(ctx context.Context, cmd *exec.Cmd, req *models.ScanRequest) error {
	// Get timeout from config
	timeout, err := s.getTimeout(req)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Replace command context with timeout context
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scanner: %w", err)
	}

	// Wait for completion or timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		// Timeout - kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return timeoutCtx.Err()
	}
}

// getTimeout returns the timeout duration for a scan request
func (s *Scanner) getTimeout(req *models.ScanRequest) (time.Duration, error) {
	// Check for registry-specific timeout
	for _, reg := range s.config.Registries {
		if reg.Name == req.RegistryName && reg.Scanner.Timeout != "" {
			return time.ParseDuration(reg.Scanner.Timeout)
		}
	}

	// Use default timeout
	return time.ParseDuration(s.config.Scanner.DefaultTimeout)
}

// getExitCode extracts the exit code from an exec.ExitError
func (s *Scanner) getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// ValidateCLI checks if the Sysdig CLI scanner is available and executable
func (s *Scanner) ValidateCLI() error {
	_, err := exec.LookPath(s.config.Scanner.CLIPath)
	if err != nil {
		return fmt.Errorf("Sysdig CLI scanner not found at %s: %w", s.config.Scanner.CLIPath, err)
	}
	return nil
}

// FormatImageRef ensures the image reference is in the correct format for the scanner
func (s *Scanner) FormatImageRef(req *models.ScanRequest) string {
	// If we have a digest, prefer digest-based reference
	if req.Digest != "" {
		if strings.Contains(req.ImageRef, "@") {
			return req.ImageRef
		}
		// Strip tag and add digest
		parts := strings.Split(req.ImageRef, ":")
		if len(parts) > 0 {
			return fmt.Sprintf("%s@%s", parts[0], req.Digest)
		}
	}

	return req.ImageRef
}

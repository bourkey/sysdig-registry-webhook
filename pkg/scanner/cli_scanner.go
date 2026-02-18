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
	"github.com/sysdig/registry-webhook-scanner/pkg/metrics"
)

// CLIScanner wraps the Sysdig CLI Scanner
type CLIScanner struct {
	config *config.Config
	logger *logrus.Logger
}

// NewCLIScanner creates a new CLIScanner instance
func NewCLIScanner(cfg *config.Config, logger *logrus.Logger) *CLIScanner {
	return &CLIScanner{
		config: cfg,
		logger: logger,
	}
}

// Type returns the scanner type identifier
func (s *CLIScanner) Type() string {
	return "cli"
}

// Scan executes the Sysdig CLI Scanner for the given image
func (s *CLIScanner) Scan(ctx context.Context, req *models.ScanRequest) (*models.ScanResult, error) {
	startTime := time.Now()

	result := &models.ScanResult{
		ImageRef:  req.ImageRef,
		RequestID: req.RequestID,
		Status:    models.ScanStatusRunning,
		StartedAt: startTime,
	}

	s.logger.WithFields(logrus.Fields{
		"image_ref":    req.ImageRef,
		"request_id":   req.RequestID,
		"scanner_type": "cli",
	}).Info("Starting CLI Scanner image scan")

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
			result.Error = "CLI Scanner timeout exceeded"
			s.logger.WithFields(logrus.Fields{
				"image_ref":    req.ImageRef,
				"request_id":   req.RequestID,
				"duration":     result.Duration,
				"scanner_type": "cli",
			}).Warn("CLI Scanner timeout")
			return result, fmt.Errorf("CLI Scanner timeout")
		}

		// Check if exit code indicates vulnerabilities found (not an error)
		if result.ExitCode > 0 && result.ExitCode < 10 {
			// Sysdig CLI returns non-zero for vulnerabilities, but scan succeeded
			result.Status = models.ScanStatusSuccess
			s.logger.WithFields(logrus.Fields{
				"image_ref":    req.ImageRef,
				"request_id":   req.RequestID,
				"exit_code":    result.ExitCode,
				"duration":     result.Duration,
				"scanner_type": "cli",
			}).Info("CLI Scanner completed with vulnerabilities found")

			// Record metrics
			metrics.RecordScannerType("cli", "success")
			metrics.RecordScanDuration("cli", "success", result.Duration.Seconds())
			metrics.RecordScan("cli", req.RegistryName, "success")

			return result, nil
		}

		// Actual error
		result.Status = models.ScanStatusFailed
		result.Error = err.Error()
		s.logger.WithFields(logrus.Fields{
			"image_ref":    req.ImageRef,
			"request_id":   req.RequestID,
			"error":        err.Error(),
			"exit_code":    result.ExitCode,
			"scanner_type": "cli",
		}).Error("CLI Scanner failed")

		// Record failure metrics
		metrics.RecordScannerType("cli", "failed")
		metrics.RecordScanDuration("cli", "failed", result.Duration.Seconds())
		metrics.RecordScan("cli", req.RegistryName, "failed")

		return result, err
	}

	// Success
	result.Status = models.ScanStatusSuccess
	result.ExitCode = 0
	s.logger.WithFields(logrus.Fields{
		"image_ref":    req.ImageRef,
		"request_id":   req.RequestID,
		"duration":     result.Duration,
		"scanner_type": "cli",
	}).Info("CLI Scanner completed successfully")

	// Record metrics
	metrics.RecordScannerType("cli", "success")
	metrics.RecordScanDuration("cli", "success", result.Duration.Seconds())
	metrics.RecordScan("cli", req.RegistryName, "success")

	return result, nil
}

// buildScanArgs constructs the arguments for the Sysdig CLI scanner
func (s *CLIScanner) buildScanArgs(req *models.ScanRequest) []string {
	args := []string{
		req.ImageRef,
		"--apiurl", "https://secure.sysdig.com",
	}

	// Add registry credentials if configured
	if req.RegistryName != "" {
		for _, reg := range s.config.Registries {
			if reg.Name == req.RegistryName {
				if reg.Scanner.Credentials.Username != "" {
					args = append(args, "--registry-user", reg.Scanner.Credentials.Username)
				}
				if reg.Scanner.Credentials.Password != "" {
					args = append(args, "--registry-password", reg.Scanner.Credentials.Password)
				}
				break
			}
		}
	}

	// Add JSON output for easier parsing
	args = append(args, "--json-scan-result", "/dev/stdout")

	return args
}

// buildCommand constructs the Sysdig CLI scanner command
func (s *CLIScanner) buildCommand(ctx context.Context, req *models.ScanRequest) (*exec.Cmd, error) {
	args := s.buildScanArgs(req)

	// Create command
	cmd := exec.CommandContext(ctx, s.config.Scanner.CLIPath, args...)

	// Set environment variables for authentication
	cmd.Env = append(cmd.Env, fmt.Sprintf("SYSDIG_API_TOKEN=%s", s.config.Scanner.SysdigToken))

	return cmd, nil
}

// executeWithTimeout executes the command with a timeout
func (s *CLIScanner) executeWithTimeout(ctx context.Context, cmd *exec.Cmd, req *models.ScanRequest) error {
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
func (s *CLIScanner) getTimeout(req *models.ScanRequest) (time.Duration, error) {
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
func (s *CLIScanner) getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// ValidateConfig checks if the Sysdig CLI scanner is available and executable
func (s *CLIScanner) ValidateConfig() error {
	_, err := exec.LookPath(s.config.Scanner.CLIPath)
	if err != nil {
		return fmt.Errorf("Sysdig CLI scanner not found at %s: %w", s.config.Scanner.CLIPath, err)
	}
	return nil
}

// FormatImageRef ensures the image reference is in the correct format for the scanner
func (s *CLIScanner) FormatImageRef(req *models.ScanRequest) string {
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

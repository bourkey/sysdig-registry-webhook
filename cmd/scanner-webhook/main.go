package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
	"github.com/sysdig/registry-webhook-scanner/pkg/queue"
	"github.com/sysdig/registry-webhook-scanner/pkg/scanner"
	"github.com/sysdig/registry-webhook-scanner/pkg/shutdown"
	"github.com/sysdig/registry-webhook-scanner/pkg/webhook"
)

const (
	gracefulShutdownTimeout = 30 * time.Second
	workerShutdownTimeout   = 2 * time.Minute
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	logger.Info("Registry Webhook Scanner starting...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Set log level from config
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	logger.WithFields(logrus.Fields{
		"log_level":     cfg.LogLevel,
		"port":          cfg.Server.Port,
		"scanner_type":  cfg.Scanner.Type,
		"max_workers":   cfg.Queue.Workers,
		"queue_size":    cfg.Queue.BufferSize,
		"registries":    len(cfg.Registries),
	}).Info("Configuration loaded")

	// Validate scanner configuration at startup
	if err := validateScannerConfig(cfg, logger); err != nil {
		logger.WithError(err).Fatal("Scanner configuration validation failed")
	}

	// Create scan queue
	scanQueue := queue.NewScanQueue(cfg.Queue.BufferSize, logger)

	// Create scan handler that uses scanner factory
	scanHandler := createScanHandler(cfg, logger)

	// Create worker pool
	workerPool := queue.NewWorkerPool(scanQueue, cfg.Queue.Workers, scanHandler, logger)

	// Create webhook server
	webhookServer := webhook.NewServer(cfg, logger)

	// Setup graceful shutdown
	shutdownManager := shutdown.NewManager(logger)
	shutdownManager.RegisterCleanup("webhook-server", func(ctx context.Context) error {
		return webhookServer.Shutdown(ctx)
	})
	shutdownManager.RegisterCleanup("worker-pool", func(ctx context.Context) error {
		return workerPool.Stop(workerShutdownTimeout)
	})
	shutdownManager.RegisterCleanup("scan-queue", func(ctx context.Context) error {
		scanQueue.Close()
		return nil
	})

	// Start worker pool
	workerPool.Start()
	logger.Info("Worker pool started")

	// Mark server as ready
	webhookServer.SetReady(true)

	// Start HTTP server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := webhookServer.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.WithField("signal", sig).Info("Received shutdown signal")
	case err := <-serverErr:
		logger.WithError(err).Error("Server error occurred")
	}

	// Graceful shutdown
	logger.Info("Initiating graceful shutdown...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer shutdownCancel()

	if err := shutdownManager.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Graceful shutdown failed")
		os.Exit(1)
	}

	logger.Info("Registry Webhook Scanner stopped")
	os.Exit(0)
}

// validateScannerConfig validates scanner configuration for all registries at startup
func validateScannerConfig(cfg *config.Config, logger *logrus.Logger) error {
	logger.Info("Validating scanner configuration...")

	// Validate global scanner config
	globalBackend, err := scanner.NewScannerBackend(cfg, "", logger)
	if err != nil {
		return fmt.Errorf("global scanner validation failed: %w", err)
	}

	logger.WithField("scanner_type", globalBackend.Type()).Info("Global scanner validated")

	// Validate per-registry scanner overrides
	for _, reg := range cfg.Registries {
		if reg.Scanner.Type != "" {
			backend, err := scanner.NewScannerBackend(cfg, reg.Name, logger)
			if err != nil {
				return fmt.Errorf("scanner validation failed for registry %s: %w", reg.Name, err)
			}
			logger.WithFields(logrus.Fields{
				"registry":     reg.Name,
				"scanner_type": backend.Type(),
			}).Info("Registry scanner override validated")
		}
	}

	return nil
}

// createScanHandler creates a scan handler function that uses the scanner factory
func createScanHandler(cfg *config.Config, logger *logrus.Logger) queue.ScanHandler {
	return func(ctx context.Context, req *models.ScanRequest) error {
		scanLogger := logger.WithFields(logrus.Fields{
			"request_id": req.RequestID,
			"image_ref":  req.ImageRef,
			"registry":   req.Registry,
		})

		// Create scanner backend using factory
		// This automatically selects CLI or Registry scanner based on config
		backend, err := scanner.NewScannerBackend(cfg, req.Registry, scanLogger)
		if err != nil {
			scanLogger.WithError(err).Error("Failed to create scanner backend")
			return fmt.Errorf("scanner backend creation failed: %w", err)
		}

		scanLogger.WithField("scanner_type", backend.Type()).Info("Initiating scan")

		// Execute scan
		startTime := time.Now()
		result, err := backend.Scan(ctx, req)
		duration := time.Since(startTime)

		if err != nil {
			scanLogger.WithFields(logrus.Fields{
				"error":       err.Error(),
				"duration_ms": duration.Milliseconds(),
				"scanner_type": backend.Type(),
			}).Error("Scan failed")
			return fmt.Errorf("scan execution failed: %w", err)
		}

		scanLogger.WithFields(logrus.Fields{
			"duration_ms":  duration.Milliseconds(),
			"scanner_type": backend.Type(),
			"result_status": result.Status,
		}).Info("Scan completed successfully")

		return nil
	}
}

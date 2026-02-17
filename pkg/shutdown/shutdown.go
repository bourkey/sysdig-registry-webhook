package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager handles graceful shutdown coordination
type Manager struct {
	logger       *logrus.Logger
	shutdownChan chan os.Signal
	handlers     []ShutdownHandler
	timeout      time.Duration
	mu           sync.Mutex
	isShuttingDown bool
}

// ShutdownHandler is a function that performs cleanup during shutdown
type ShutdownHandler func(ctx context.Context) error

// NewManager creates a new shutdown manager
func NewManager(timeout time.Duration, logger *logrus.Logger) *Manager {
	return &Manager{
		logger:       logger,
		shutdownChan: make(chan os.Signal, 1),
		handlers:     make([]ShutdownHandler, 0),
		timeout:      timeout,
		isShuttingDown: false,
	}
}

// RegisterHandler adds a shutdown handler to be called during shutdown
func (m *Manager) RegisterHandler(name string, handler ShutdownHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrappedHandler := func(ctx context.Context) error {
		m.logger.WithField("handler", name).Info("Executing shutdown handler")
		start := time.Now()

		err := handler(ctx)

		duration := time.Since(start)
		if err != nil {
			m.logger.WithFields(logrus.Fields{
				"handler":  name,
				"duration": duration.Seconds(),
				"error":    err.Error(),
			}).Error("Shutdown handler failed")
			return err
		}

		m.logger.WithFields(logrus.Fields{
			"handler":  name,
			"duration": duration.Seconds(),
		}).Info("Shutdown handler completed")
		return nil
	}

	m.handlers = append(m.handlers, wrappedHandler)
}

// WaitForShutdown blocks until a shutdown signal is received
func (m *Manager) WaitForShutdown() {
	// Register for shutdown signals
	signal.Notify(m.shutdownChan, syscall.SIGTERM, syscall.SIGINT)

	// Wait for signal
	sig := <-m.shutdownChan

	m.logger.WithFields(logrus.Fields{
		"signal": sig.String(),
	}).Warn("Shutdown signal received")

	m.Shutdown()
}

// Shutdown executes all registered shutdown handlers
func (m *Manager) Shutdown() {
	m.mu.Lock()
	if m.isShuttingDown {
		m.mu.Unlock()
		return
	}
	m.isShuttingDown = true
	m.mu.Unlock()

	m.logger.Info("Starting graceful shutdown")
	start := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	// Execute all handlers
	var wg sync.WaitGroup
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	for _, handler := range m.handlers {
		wg.Add(1)
		go func(h ShutdownHandler) {
			defer wg.Done()

			if err := h(ctx); err != nil {
				errorsMu.Lock()
				errors = append(errors, err)
				errorsMu.Unlock()
			}
		}(handler)
	}

	// Wait for all handlers to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		duration := time.Since(start)
		if len(errors) > 0 {
			m.logger.WithFields(logrus.Fields{
				"duration": duration.Seconds(),
				"errors":   len(errors),
			}).Warn("Shutdown completed with errors")
		} else {
			m.logger.WithFields(logrus.Fields{
				"duration": duration.Seconds(),
			}).Info("Shutdown completed successfully")
		}
	case <-ctx.Done():
		m.logger.WithFields(logrus.Fields{
			"timeout": m.timeout.Seconds(),
		}).Error("Shutdown timeout exceeded")
	}
}

// IsShuttingDown returns true if shutdown has been initiated
func (m *Manager) IsShuttingDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isShuttingDown
}

// TriggerShutdown manually triggers a shutdown (for testing or programmatic shutdown)
func (m *Manager) TriggerShutdown() {
	m.shutdownChan <- syscall.SIGTERM
}

// ShutdownCoordinator coordinates shutdown of multiple components
type ShutdownCoordinator struct {
	stopAcceptingRequests func()
	stopWorkerPool        func(context.Context) error
	closeQueue            func()
	cleanupResources      func()
	logger                *logrus.Logger
}

// NewShutdownCoordinator creates a new shutdown coordinator
func NewShutdownCoordinator(logger *logrus.Logger) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		logger: logger,
	}
}

// SetStopAcceptingRequests sets the function to stop accepting new requests
func (sc *ShutdownCoordinator) SetStopAcceptingRequests(fn func()) {
	sc.stopAcceptingRequests = fn
}

// SetStopWorkerPool sets the function to stop the worker pool
func (sc *ShutdownCoordinator) SetStopWorkerPool(fn func(context.Context) error) {
	sc.stopWorkerPool = fn
}

// SetCloseQueue sets the function to close the queue
func (sc *ShutdownCoordinator) SetCloseQueue(fn func()) {
	sc.closeQueue = fn
}

// SetCleanupResources sets the function to cleanup resources
func (sc *ShutdownCoordinator) SetCleanupResources(fn func()) {
	sc.cleanupResources = fn
}

// ExecuteShutdown performs the coordinated shutdown sequence
func (sc *ShutdownCoordinator) ExecuteShutdown(ctx context.Context) error {
	sc.logger.Info("Executing coordinated shutdown")

	// Step 1: Stop accepting new requests
	if sc.stopAcceptingRequests != nil {
		sc.logger.Info("Stopping acceptance of new webhooks")
		sc.stopAcceptingRequests()
	}

	// Step 2: Wait for worker pool to finish in-flight scans
	if sc.stopWorkerPool != nil {
		sc.logger.Info("Waiting for in-flight scans to complete")
		if err := sc.stopWorkerPool(ctx); err != nil {
			sc.logger.WithError(err).Warn("Worker pool shutdown had errors")
		}
	}

	// Step 3: Close the queue
	if sc.closeQueue != nil {
		sc.logger.Info("Closing scan queue")
		sc.closeQueue()
	}

	// Step 4: Cleanup resources
	if sc.cleanupResources != nil {
		sc.logger.Info("Cleaning up resources")
		sc.cleanupResources()
	}

	sc.logger.Info("Coordinated shutdown complete")
	return nil
}

// GracefulShutdownHandler creates a shutdown handler from a coordinator
func GracefulShutdownHandler(coordinator *ShutdownCoordinator) ShutdownHandler {
	return func(ctx context.Context) error {
		return coordinator.ExecuteShutdown(ctx)
	}
}

// WaitWithContext waits for context cancellation or timeout
func WaitWithContext(ctx context.Context, logger *logrus.Logger) error {
	<-ctx.Done()
	if ctx.Err() == context.DeadlineExceeded {
		logger.Warn("Shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout exceeded")
	}
	return nil
}

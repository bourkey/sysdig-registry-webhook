package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// ScanHandler is a function that processes a scan request
type ScanHandler func(ctx context.Context, req *models.ScanRequest) error

// WorkerPool manages a pool of worker goroutines that process scan requests
type WorkerPool struct {
	queue       *ScanQueue
	workers     int
	handler     ScanHandler
	logger      *logrus.Logger
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	stopOnce    sync.Once
	inFlight    int64 // atomic counter for in-flight scans
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queue *ScanQueue, workers int, handler ScanHandler, logger *logrus.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		queue:    queue,
		workers:  workers,
		handler:  handler,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		inFlight: 0,
	}
}

// Start starts all worker goroutines
func (wp *WorkerPool) Start() {
	wp.logger.WithFields(logrus.Fields{
		"workers": wp.workers,
	}).Info("Starting worker pool")

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
// Waits for in-flight scans to complete with the given timeout
func (wp *WorkerPool) Stop(timeout time.Duration) error {
	var stopErr error

	wp.stopOnce.Do(func() {
		wp.logger.Info("Stopping worker pool")

		// Signal workers to stop
		wp.cancel()

		// Wait for workers to finish with timeout
		done := make(chan struct{})
		go func() {
			wp.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			wp.logger.Info("All workers stopped gracefully")
		case <-time.After(timeout):
			stopErr = fmt.Errorf("worker pool shutdown timeout after %v", timeout)
			wp.logger.Warn("Worker pool shutdown timeout, some workers may still be running")
		}
	})

	return stopErr
}

// worker is a goroutine that processes scan requests from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	workerLogger := wp.logger.WithField("worker_id", id)
	workerLogger.Debug("Worker started")

	for {
		select {
		case <-wp.ctx.Done():
			workerLogger.Debug("Worker stopping")
			return
		default:
			// Try to dequeue a scan request
			req, err := wp.queue.Dequeue(wp.ctx)
			if err != nil {
				if wp.ctx.Err() != nil {
					// Context cancelled, worker should stop
					return
				}
				// Queue closed or other error
				workerLogger.WithError(err).Debug("Dequeue error")
				return
			}

			// Process the scan request
			wp.processScan(workerLogger, req)
		}
	}
}

// processScan processes a single scan request with error handling and recovery
func (wp *WorkerPool) processScan(logger *logrus.Logger, req *models.ScanRequest) {
	// Recover from panics in scan handler
	defer func() {
		if r := recover(); r != nil {
			logger.WithFields(logrus.Fields{
				"image_ref":  req.ImageRef,
				"request_id": req.RequestID,
				"panic":      r,
			}).Error("Worker panic recovered")
		}
	}()

	logger.WithFields(logrus.Fields{
		"image_ref":  req.ImageRef,
		"request_id": req.RequestID,
	}).Info("Processing scan request")

	// Call the scan handler
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := wp.handler(ctx, req); err != nil {
		logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
			"error":      err.Error(),
		}).Error("Scan processing failed")
	} else {
		logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
		}).Info("Scan processing completed")
	}
}

// Stats returns worker pool statistics
func (wp *WorkerPool) Stats() WorkerPoolStats {
	return WorkerPoolStats{
		Workers:    wp.workers,
		InFlight:   0, // TODO: track in-flight count
		QueueDepth: wp.queue.Depth(),
	}
}

// WorkerPoolStats represents worker pool statistics
type WorkerPoolStats struct {
	Workers    int
	InFlight   int
	QueueDepth int
}

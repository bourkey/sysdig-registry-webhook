package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/internal/models"
)

// ScanQueue represents an in-memory queue for scan requests
type ScanQueue struct {
	queue      chan *models.ScanRequest
	capacity   int
	depth      int64 // atomic counter for current queue depth
	logger     *logrus.Logger
	mu         sync.RWMutex
	closed     bool
}

// NewScanQueue creates a new scan queue with the specified capacity
func NewScanQueue(capacity int, logger *logrus.Logger) *ScanQueue {
	return &ScanQueue{
		queue:    make(chan *models.ScanRequest, capacity),
		capacity: capacity,
		depth:    0,
		logger:   logger,
		closed:   false,
	}
}

// Enqueue adds a scan request to the queue
// Returns error if queue is full or closed
func (q *ScanQueue) Enqueue(ctx context.Context, req *models.ScanRequest) error {
	q.mu.RLock()
	if q.closed {
		q.mu.RUnlock()
		return fmt.Errorf("queue is closed")
	}
	q.mu.RUnlock()

	select {
	case q.queue <- req:
		atomic.AddInt64(&q.depth, 1)
		q.logger.WithFields(logrus.Fields{
			"image_ref":  req.ImageRef,
			"request_id": req.RequestID,
			"queue_depth": atomic.LoadInt64(&q.depth),
		}).Debug("Scan request enqueued")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("enqueue cancelled: %w", ctx.Err())
	default:
		return fmt.Errorf("queue is full (capacity: %d)", q.capacity)
	}
}

// Dequeue removes and returns a scan request from the queue (FIFO)
// Blocks until a request is available or context is cancelled
func (q *ScanQueue) Dequeue(ctx context.Context) (*models.ScanRequest, error) {
	select {
	case req, ok := <-q.queue:
		if !ok {
			return nil, fmt.Errorf("queue is closed")
		}
		atomic.AddInt64(&q.depth, -1)
		return req, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("dequeue cancelled: %w", ctx.Err())
	}
}

// Depth returns the current number of items in the queue
func (q *ScanQueue) Depth() int {
	return int(atomic.LoadInt64(&q.depth))
}

// Capacity returns the maximum capacity of the queue
func (q *ScanQueue) Capacity() int {
	return q.capacity
}

// IsFull returns true if the queue is at capacity
func (q *ScanQueue) IsFull() bool {
	return q.Depth() >= q.capacity
}

// IsEmpty returns true if the queue is empty
func (q *ScanQueue) IsEmpty() bool {
	return q.Depth() == 0
}

// Close closes the queue, preventing new enqueues
// Existing items remain in the queue for processing
func (q *ScanQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.closed {
		q.closed = true
		close(q.queue)
		q.logger.Info("Scan queue closed")
	}
}

// IsClosed returns true if the queue has been closed
func (q *ScanQueue) IsClosed() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.closed
}

// Stats returns queue statistics
func (q *ScanQueue) Stats() QueueStats {
	depth := q.Depth()
	return QueueStats{
		Depth:       depth,
		Capacity:    q.capacity,
		Utilization: float64(depth) / float64(q.capacity) * 100,
		IsFull:      depth >= q.capacity,
		IsEmpty:     depth == 0,
	}
}

// QueueStats represents queue statistics
type QueueStats struct {
	Depth       int
	Capacity    int
	Utilization float64 // Percentage (0-100)
	IsFull      bool
	IsEmpty     bool
}

package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ScannerAPIDuration tracks Registry Scanner API call duration in seconds
	ScannerAPIDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "scanner_api_duration_seconds",
			Help:    "Duration of Registry Scanner API calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "status_code"},
	)

	// ScannerPollAttempts tracks number of poll attempts per Registry Scanner scan
	ScannerPollAttempts = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "scanner_poll_attempts_total",
			Help:    "Number of polling attempts per Registry Scanner scan",
			Buckets: []float64{1, 2, 3, 5, 10, 20, 50, 100},
		},
	)

	// ScannerAPIErrors tracks Registry Scanner API errors by type
	ScannerAPIErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scanner_api_errors_total",
			Help: "Total number of Registry Scanner API errors by type",
		},
		[]string{"error_type", "status_code"},
	)

	// ScannerTypeDistribution tracks scanner type usage (CLI vs Registry)
	ScannerTypeDistribution = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scanner_type_total",
			Help: "Total number of scans by scanner type",
		},
		[]string{"scanner_type", "status"},
	)

	// ScanDuration tracks overall scan duration by scanner type
	ScanDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "scan_duration_seconds",
			Help:    "Duration of image scans in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"scanner_type", "status"},
	)

	// ScanTotal tracks total number of scans
	ScanTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scan_total",
			Help: "Total number of scans",
		},
		[]string{"scanner_type", "registry", "status"},
	)
)

// RecordScannerAPIDuration records the duration of a Registry Scanner API call
func RecordScannerAPIDuration(endpoint string, statusCode int, duration float64) {
	ScannerAPIDuration.WithLabelValues(endpoint, fmt.Sprintf("%d", statusCode)).Observe(duration)
}

// RecordScannerPollAttempts records the number of poll attempts for a scan
func RecordScannerPollAttempts(attempts int) {
	ScannerPollAttempts.Observe(float64(attempts))
}

// RecordScannerAPIError records a Registry Scanner API error
func RecordScannerAPIError(errorType string, statusCode int) {
	ScannerAPIErrors.WithLabelValues(errorType, fmt.Sprintf("%d", statusCode)).Inc()
}

// RecordScannerType records scanner type usage
func RecordScannerType(scannerType, status string) {
	ScannerTypeDistribution.WithLabelValues(scannerType, status).Inc()
}

// RecordScanDuration records the overall scan duration
func RecordScanDuration(scannerType, status string, duration float64) {
	ScanDuration.WithLabelValues(scannerType, status).Observe(duration)
}

// RecordScan records a completed scan
func RecordScan(scannerType, registry, status string) {
	ScanTotal.WithLabelValues(scannerType, registry, status).Inc()
}

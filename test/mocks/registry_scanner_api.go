package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// MockRegistryScannerAPI provides a mock implementation of the Sysdig Registry Scanner API
type MockRegistryScannerAPI struct {
	Server   *httptest.Server
	mu       sync.Mutex
	scans    map[string]*MockScan
	callLog  []APICall
	behavior APIBehavior
}

// MockScan represents a scan in progress
type MockScan struct {
	ID          string
	Status      string
	StartTime   time.Time
	CompletedAt *time.Time
	Error       string
	Result      *ScanResult
}

// ScanResult represents scan results
type ScanResult struct {
	Vulnerabilities VulnerabilityCounts `json:"vulnerabilities"`
}

// VulnerabilityCounts represents vulnerability counts by severity
type VulnerabilityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// APICall logs API calls for verification
type APICall struct {
	Method   string
	Path     string
	Time     time.Time
	Response int
}

// APIBehavior controls mock API behavior
type APIBehavior struct {
	// InitiateScanDelay adds artificial delay to scan initiation
	InitiateScanDelay time.Duration
	
	// InitiateScanStatus overrides response status code (0 = success)
	InitiateScanStatus int
	
	// PollDelay simulates polling delay before scan completes
	PollDelay time.Duration
	
	// FailAfterPolls causes scan to fail after N poll attempts
	FailAfterPolls int
	
	// RateLimitAfter causes rate limiting after N requests
	RateLimitAfter int
	
	// RateLimitDuration sets Retry-After duration for rate limiting
	RateLimitDuration time.Duration
	
	// UnauthorizedRequests makes all requests return 401
	UnauthorizedRequests bool
	
	// CompletionPollCount sets how many polls before scan completes
	CompletionPollCount int
}

// NewMockRegistryScannerAPI creates a new mock API server
func NewMockRegistryScannerAPI() *MockRegistryScannerAPI {
	mock := &MockRegistryScannerAPI{
		scans:   make(map[string]*MockScan),
		callLog: make([]APICall, 0),
		behavior: APIBehavior{
			CompletionPollCount: 3, // Default: complete after 3 polls
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scanning/v1/registry/scan", mock.handleInitiateScan)
	mux.HandleFunc("/api/scanning/v1/registry/scan/", mock.handleGetScanStatus)

	mock.Server = httptest.NewServer(mux)
	return mock
}

// Close stops the mock server
func (m *MockRegistryScannerAPI) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockRegistryScannerAPI) URL() string {
	return m.Server.URL
}

// GetCallLog returns all API calls made
func (m *MockRegistryScannerAPI) GetCallLog() []APICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]APICall{}, m.callLog...)
}

// GetScan returns a mock scan by ID
func (m *MockRegistryScannerAPI) GetScan(id string) *MockScan {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scans[id]
}

// SetBehavior configures mock API behavior
func (m *MockRegistryScannerAPI) SetBehavior(behavior APIBehavior) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.behavior = behavior
}

// Reset clears all scans and call logs
func (m *MockRegistryScannerAPI) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scans = make(map[string]*MockScan)
	m.callLog = make([]APICall, 0)
}

func (m *MockRegistryScannerAPI) logCall(method, path string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callLog = append(m.callLog, APICall{
		Method:   method,
		Path:     path,
		Time:     time.Now(),
		Response: status,
	})
}

func (m *MockRegistryScannerAPI) handleInitiateScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mu.Lock()
	behavior := m.behavior
	callCount := len(m.callLog)
	m.mu.Unlock()

	// Artificial delay
	if behavior.InitiateScanDelay > 0 {
		time.Sleep(behavior.InitiateScanDelay)
	}

	// Check for unauthorized behavior
	if behavior.UnauthorizedRequests {
		m.logCall("POST", r.URL.Path, http.StatusUnauthorized)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check for rate limiting
	if behavior.RateLimitAfter > 0 && callCount >= behavior.RateLimitAfter {
		m.logCall("POST", r.URL.Path, http.StatusTooManyRequests)
		retryAfter := "5"
		if behavior.RateLimitDuration > 0 {
			retryAfter = string(rune(int(behavior.RateLimitDuration.Seconds())))
		}
		w.Header().Set("Retry-After", retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limit exceeded"))
		return
	}

	// Check for custom status code
	if behavior.InitiateScanStatus > 0 {
		m.logCall("POST", r.URL.Path, behavior.InitiateScanStatus)
		http.Error(w, "Custom error", behavior.InitiateScanStatus)
		return
	}

	// Create new scan
	scanID := generateScanID()
	scan := &MockScan{
		ID:        scanID,
		Status:    "running",
		StartTime: time.Now(),
		Result: &ScanResult{
			Vulnerabilities: VulnerabilityCounts{
				Critical: 2,
				High:     5,
				Medium:   10,
				Low:      15,
			},
		},
	}

	m.mu.Lock()
	m.scans[scanID] = scan
	m.mu.Unlock()

	m.logCall("POST", r.URL.Path, http.StatusCreated)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"scan_id": scanID,
	})
}

func (m *MockRegistryScannerAPI) handleGetScanStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract scan ID from path
	scanID := r.URL.Path[len("/api/scanning/v1/registry/scan/"):]
	if scanID == "" {
		http.Error(w, "Scan ID required", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	scan, exists := m.scans[scanID]
	behavior := m.behavior
	
	// Count polls for this scan
	pollCount := 0
	for _, call := range m.callLog {
		if call.Method == "GET" && call.Path == r.URL.Path {
			pollCount++
		}
	}
	m.mu.Unlock()

	if !exists {
		m.logCall("GET", r.URL.Path, http.StatusNotFound)
		http.Error(w, "Scan not found", http.StatusNotFound)
		return
	}

	// Check for unauthorized behavior
	if behavior.UnauthorizedRequests {
		m.logCall("GET", r.URL.Path, http.StatusUnauthorized)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Simulate completion after N polls
	if pollCount >= behavior.CompletionPollCount && scan.Status == "running" {
		m.mu.Lock()
		scan.Status = "completed"
		now := time.Now()
		scan.CompletedAt = &now
		m.mu.Unlock()
	}

	// Check if scan should fail
	if behavior.FailAfterPolls > 0 && pollCount >= behavior.FailAfterPolls && scan.Status == "running" {
		m.mu.Lock()
		scan.Status = "failed"
		scan.Error = "Scan failed due to test configuration"
		now := time.Now()
		scan.CompletedAt = &now
		m.mu.Unlock()
	}

	m.logCall("GET", r.URL.Path, http.StatusOK)

	// Build response
	response := map[string]interface{}{
		"status": scan.Status,
	}

	if scan.Error != "" {
		response["error"] = scan.Error
	}

	if scan.Status == "completed" && scan.Result != nil {
		response["result"] = scan.Result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

var scanCounter int

func generateScanID() string {
	scanCounter++
	return fmt.Sprintf("scan-%d-%d", time.Now().Unix(), scanCounter)
}

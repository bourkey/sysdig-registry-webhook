package webhook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// Server represents the HTTP webhook server
type Server struct {
	config     *config.Config
	router     *mux.Router
	httpServer *http.Server
	logger     *logrus.Logger
	ready      bool
}

// NewServer creates a new webhook server instance
func NewServer(cfg *config.Config, logger *logrus.Logger) *Server {
	s := &Server{
		config: cfg,
		router: mux.NewRouter(),
		logger: logger,
		ready:  false,
	}

	// Setup routes
	s.setupRoutes()

	// Create HTTP server
	readTimeout, _ := cfg.ParseDuration(cfg.Server.ReadTimeout)
	writeTimeout, _ := cfg.ParseDuration(cfg.Server.WriteTimeout)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	return s
}

// setupRoutes configures HTTP routes and middleware
func (s *Server) setupRoutes() {
	// Apply global middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.requestSizeLimitMiddleware)

	// Webhook endpoint
	s.router.HandleFunc("/webhook", s.handleWebhook).Methods(http.MethodPost)

	// Health endpoint
	s.router.HandleFunc("/health", s.handleHealth).Methods(http.MethodGet)

	// Readiness endpoint
	s.router.HandleFunc("/ready", s.handleReadiness).Methods(http.MethodGet)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.WithFields(logrus.Fields{
		"port": s.config.Server.Port,
	}).Info("Starting HTTP server")

	s.ready = true

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	s.ready = false

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// SetReady sets the readiness status
func (s *Server) SetReady(ready bool) {
	s.ready = ready
}

// handleWebhook processes incoming webhook requests
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement webhook processing
	// This will be implemented with authentication and registry parsing

	s.logger.WithFields(logrus.Fields{
		"method":     r.Method,
		"remote_addr": r.RemoteAddr,
	}).Debug("Webhook received")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"accepted"}`))
}

// handleHealth returns the health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// handleReadiness returns the readiness status
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !s.ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not ready"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		s.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
			"status_code": rw.statusCode,
			"duration_ms": duration.Milliseconds(),
		}).Info("HTTP request")
	})
}

// requestSizeLimitMiddleware enforces maximum request size
func (s *Server) requestSizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, s.config.Server.MaxRequestSize)
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

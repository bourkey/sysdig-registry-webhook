package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// NewLogger creates and configures a new structured logger
func NewLogger(level LogLevel) *logrus.Logger {
	logger := logrus.New()

	// Set output to stdout
	logger.SetOutput(os.Stdout)

	// Use JSON formatter for structured logging
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	// Set log level
	logger.SetLevel(parseLogLevel(level))

	// Add default fields
	logger.WithFields(logrus.Fields{
		"service": "registry-webhook-scanner",
	})

	return logger
}

// parseLogLevel converts string log level to logrus.Level
func parseLogLevel(level LogLevel) logrus.Level {
	switch level {
	case LogLevelDebug:
		return logrus.DebugLevel
	case LogLevelInfo:
		return logrus.InfoLevel
	case LogLevelWarn:
		return logrus.WarnLevel
	case LogLevelError:
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}

// LogStartup logs service startup information
func LogStartup(logger *logrus.Logger, version, port string) {
	logger.WithFields(logrus.Fields{
		"event":   "startup",
		"version": version,
		"port":    port,
	}).Info("Registry Webhook Scanner starting")
}

// LogConfigurationLoaded logs successful configuration loading
func LogConfigurationLoaded(logger *logrus.Logger, configPath string, registries int) {
	logger.WithFields(logrus.Fields{
		"event":      "configuration_loaded",
		"config_path": configPath,
		"registries":  registries,
	}).Info("Configuration loaded successfully")
}

// LogShutdownInitiated logs when shutdown is initiated
func LogShutdownInitiated(logger *logrus.Logger, signal string) {
	logger.WithFields(logrus.Fields{
		"event":  "shutdown_initiated",
		"signal": signal,
	}).Warn("Shutdown initiated")
}

// LogShutdownComplete logs when shutdown completes
func LogShutdownComplete(logger *logrus.Logger, duration float64) {
	logger.WithFields(logrus.Fields{
		"event":            "shutdown_complete",
		"duration_seconds": duration,
	}).Info("Shutdown complete")
}

// LogError logs an error with context
func LogError(logger *logrus.Logger, err error, context string, fields map[string]interface{}) {
	logFields := logrus.Fields{
		"error":   err.Error(),
		"context": context,
	}

	// Merge additional fields
	for k, v := range fields {
		logFields[k] = v
	}

	logger.WithFields(logFields).Error("Error occurred")
}

// LogWithRequestID returns a logger with request ID field
func LogWithRequestID(logger *logrus.Logger, requestID string) *logrus.Entry {
	return logger.WithField("request_id", requestID)
}

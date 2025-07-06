package logger

import (
	"sync"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/opensearch"
)

var (
	globalLogger *SystemLogger
	once         sync.Once
)

// InitGlobalLogger initializes the global system logger
func InitGlobalLogger(openSearchLogger any) {
	once.Do(func() {
		config := SystemLoggerConfig{
			EnableConsole:    true,
			EnableOpenSearch: openSearchLogger != nil,
			MinLevel:         LevelInfo,
			Service:          "gopay",
			Version:          "1.0.0",
			Environment:      config.GetEnv("ENVIRONMENT", "development"),
		}

		// Adjust log level based on environment
		if config.Environment == "development" {
			config.MinLevel = LevelDebug
		}

		// Type assert to the expected type
		var osLogger *opensearch.Logger
		if openSearchLogger != nil {
			if logger, ok := openSearchLogger.(*opensearch.Logger); ok {
				osLogger = logger
			}
		}

		globalLogger = NewSystemLogger(osLogger, config)
	})
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *SystemLogger {
	if globalLogger == nil {
		// Fallback to console-only logger if not initialized
		config := SystemLoggerConfig{
			EnableConsole:    true,
			EnableOpenSearch: false,
			MinLevel:         LevelInfo,
			Service:          "gopay",
			Version:          "1.0.0",
			Environment:      "development",
		}
		globalLogger = NewSystemLogger(nil, config)
	}
	return globalLogger
}

// Convenience functions for global logging

// Debug logs a debug message using the global logger
func Debug(message string, ctx ...LogContext) {
	GetGlobalLogger().Debug(message, ctx...)
}

// Info logs an info message using the global logger
func Info(message string, ctx ...LogContext) {
	GetGlobalLogger().Info(message, ctx...)
}

// Warn logs a warning message using the global logger
func Warn(message string, ctx ...LogContext) {
	GetGlobalLogger().Warn(message, ctx...)
}

// Error logs an error message using the global logger
func Error(message string, err error, ctx ...LogContext) {
	GetGlobalLogger().Error(message, err, ctx...)
}

// Fatal logs a fatal message using the global logger and exits
func Fatal(message string, err error, ctx ...LogContext) {
	GetGlobalLogger().Fatal(message, err, ctx...)
}

// WithContext creates a context logger from the global logger
func WithContext(ctx LogContext) *ContextLogger {
	return GetGlobalLogger().WithContext(ctx)
}

// WithTenant creates a context logger with tenant ID
func WithTenant(tenantID string) *ContextLogger {
	return WithContext(LogContext{TenantID: tenantID})
}

// WithProvider creates a context logger with provider
func WithProvider(provider string) *ContextLogger {
	return WithContext(LogContext{Provider: provider})
}

// WithTenantAndProvider creates a context logger with tenant and provider
func WithTenantAndProvider(tenantID, provider string) *ContextLogger {
	return WithContext(LogContext{
		TenantID: tenantID,
		Provider: provider,
	})
}

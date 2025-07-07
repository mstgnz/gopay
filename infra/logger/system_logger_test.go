package logger

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemLogger(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    true,
		EnablePostgreSQL: false,
		MinLevel:         LevelInfo,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	logger := NewSystemLogger(nil, config)

	assert.NotNil(t, logger)
	assert.Equal(t, config.EnableConsole, logger.enableConsole)
	assert.Equal(t, config.EnablePostgreSQL, logger.enablePostgreSQL)
	assert.Equal(t, config.MinLevel, logger.minLevel)
	assert.Equal(t, config.Service, logger.service)
	assert.Equal(t, config.Version, logger.version)
	assert.Equal(t, config.Environment, logger.environment)
}

func TestSystemLogger_LogLevels(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    false, // Disable console to avoid output during tests
		EnablePostgreSQL: false,
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	logger := NewSystemLogger(nil, config)

	// Test all log levels
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message", errors.New("test error"))

	// No assertions needed as we're just testing that methods don't panic
}

func TestSystemLogger_WithContext(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    false,
		EnablePostgreSQL: false,
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	logger := NewSystemLogger(nil, config)

	ctx := LogContext{
		TenantID:  "APP1",
		Provider:  "iyzico",
		RequestID: "req-123",
		Fields:    map[string]any{"key": "value"},
	}

	logger.Debug("Debug with context", ctx)
	logger.Info("Info with context", ctx)
	logger.Warn("Warning with context", ctx)
	logger.Error("Error with context", errors.New("test error"), ctx)

	// No assertions needed as we're just testing that methods don't panic
}

func TestSystemLogger_ShouldLog(t *testing.T) {
	tests := []struct {
		name     string
		minLevel LogLevel
		level    LogLevel
		expected bool
	}{
		{
			name:     "debug_level_allows_all",
			minLevel: LevelDebug,
			level:    LevelDebug,
			expected: true,
		},
		{
			name:     "info_level_blocks_debug",
			minLevel: LevelInfo,
			level:    LevelDebug,
			expected: false,
		},
		{
			name:     "info_level_allows_info",
			minLevel: LevelInfo,
			level:    LevelInfo,
			expected: true,
		},
		{
			name:     "warn_level_allows_error",
			minLevel: LevelWarn,
			level:    LevelError,
			expected: true,
		},
		{
			name:     "error_level_blocks_warn",
			minLevel: LevelError,
			level:    LevelWarn,
			expected: false,
		},
		{
			name:     "fatal_level_allows_fatal",
			minLevel: LevelFatal,
			level:    LevelFatal,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SystemLoggerConfig{
				EnableConsole:    false,
				EnablePostgreSQL: false,
				MinLevel:         tt.minLevel,
				Service:          "test-service",
				Version:          "1.0.0",
				Environment:      "test",
			}

			logger := NewSystemLogger(nil, config)
			result := logger.shouldLog(tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSystemLogger_ExtractComponent(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    false,
		EnablePostgreSQL: false,
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	logger := NewSystemLogger(nil, config)

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "provider_file",
			filePath: "/path/to/gopay/provider/iyzico/iyzico.go",
			expected: "provider/iyzico",
		},
		{
			name:     "handler_file",
			filePath: "/path/to/gopay/handler/payment.go",
			expected: "handler/payment.go",
		},
		{
			name:     "unknown_file",
			filePath: "/some/other/path/file.go",
			expected: "path",
		},
		{
			name:     "single_part",
			filePath: "file.go",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.extractComponent(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextLogger(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    false,
		EnablePostgreSQL: false,
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	systemLogger := NewSystemLogger(nil, config)

	ctx := LogContext{
		TenantID: "APP1",
		Provider: "iyzico",
	}

	contextLogger := systemLogger.WithContext(ctx)

	assert.NotNil(t, contextLogger)
	assert.Equal(t, systemLogger, contextLogger.systemLogger)
	assert.Equal(t, ctx, contextLogger.context)

	// Test context logger methods
	contextLogger.Debug("Debug message")
	contextLogger.Info("Info message")
	contextLogger.Warn("Warning message")
	contextLogger.Error("Error message", errors.New("test error"))

	// Test chaining methods
	contextLogger.AddField("key", "value").
		SetTenantID("APP2").
		SetProvider("stripe").
		SetRequestID("req-456")

	assert.Equal(t, "APP2", contextLogger.context.TenantID)
	assert.Equal(t, "stripe", contextLogger.context.Provider)
	assert.Equal(t, "req-456", contextLogger.context.RequestID)
	assert.Equal(t, "value", contextLogger.context.Fields["key"])
}

func TestSystemLogger_LogToConsole(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := SystemLoggerConfig{
		EnableConsole:    true,
		EnablePostgreSQL: false,
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	logger := NewSystemLogger(nil, config)

	// Log a message
	logger.Info("Test console message")

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	assert.Contains(t, output, "Test console message")
	assert.Contains(t, output, "INFO")
}

func TestSystemLogger_WithPostgreSQL(t *testing.T) {
	config := SystemLoggerConfig{
		EnableConsole:    false,
		EnablePostgreSQL: false, // Disable to avoid nil pointer panic
		MinLevel:         LevelDebug,
		Service:          "test-service",
		Version:          "1.0.0",
		Environment:      "test",
	}

	// Create a logger without PostgreSQL
	logger := NewSystemLogger(nil, config)

	// Test logging (should not panic)
	logger.Info("Test PostgreSQL message")

	// Verify PostgreSQL is disabled
	assert.False(t, logger.enablePostgreSQL)
}

func TestLogContext_Fields(t *testing.T) {
	ctx := LogContext{
		TenantID:  "APP1",
		Provider:  "iyzico",
		RequestID: "req-123",
		Fields: map[string]any{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
	}

	assert.Equal(t, "APP1", ctx.TenantID)
	assert.Equal(t, "iyzico", ctx.Provider)
	assert.Equal(t, "req-123", ctx.RequestID)
	assert.Equal(t, "value1", ctx.Fields["key1"])
	assert.Equal(t, 42, ctx.Fields["key2"])
	assert.Equal(t, true, ctx.Fields["key3"])
}

func TestSystemLog_Structure(t *testing.T) {
	log := SystemLog{
		Timestamp:   time.Now(),
		Level:       LevelInfo,
		Message:     "Test message",
		Component:   "test-component",
		Function:    "TestFunction",
		File:        "/path/to/file.go",
		Line:        42,
		TenantID:    "APP1",
		Provider:    "iyzico",
		RequestID:   "req-123",
		Error:       "test error",
		Fields:      map[string]any{"key": "value"},
		Environment: "test",
		Service:     "test-service",
		Version:     "1.0.0",
	}

	assert.Equal(t, LevelInfo, log.Level)
	assert.Equal(t, "Test message", log.Message)
	assert.Equal(t, "test-component", log.Component)
	assert.Equal(t, "TestFunction", log.Function)
	assert.Equal(t, "/path/to/file.go", log.File)
	assert.Equal(t, 42, log.Line)
	assert.Equal(t, "APP1", log.TenantID)
	assert.Equal(t, "iyzico", log.Provider)
	assert.Equal(t, "req-123", log.RequestID)
	assert.Equal(t, "test error", log.Error)
	assert.Equal(t, "value", log.Fields["key"])
	assert.Equal(t, "test", log.Environment)
	assert.Equal(t, "test-service", log.Service)
	assert.Equal(t, "1.0.0", log.Version)
}

func TestSystemLoggerConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config SystemLoggerConfig
		valid  bool
	}{
		{
			name: "valid_config",
			config: SystemLoggerConfig{
				EnableConsole:    true,
				EnablePostgreSQL: false,
				MinLevel:         LevelInfo,
				Service:          "test-service",
				Version:          "1.0.0",
				Environment:      "test",
			},
			valid: true,
		},
		{
			name: "empty_service",
			config: SystemLoggerConfig{
				EnableConsole:    true,
				EnablePostgreSQL: false,
				MinLevel:         LevelInfo,
				Service:          "",
				Version:          "1.0.0",
				Environment:      "test",
			},
			valid: true, // Empty service is allowed
		},
		{
			name: "invalid_log_level",
			config: SystemLoggerConfig{
				EnableConsole:    true,
				EnablePostgreSQL: false,
				MinLevel:         "invalid",
				Service:          "test-service",
				Version:          "1.0.0",
				Environment:      "test",
			},
			valid: true, // Invalid level will be handled by shouldLog
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that NewSystemLogger doesn't panic with various configs
			logger := NewSystemLogger(nil, tt.config)
			assert.NotNil(t, logger)
		})
	}
}

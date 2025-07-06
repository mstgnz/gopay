package logger

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitGlobalLogger(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	// Test initialization
	InitGlobalLogger(nil)

	assert.NotNil(t, globalLogger)
	assert.Equal(t, "gopay", globalLogger.service)
	assert.Equal(t, "1.0.0", globalLogger.version)
}

func TestGetGlobalLogger(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	// Test getting logger before initialization
	logger := GetGlobalLogger()
	assert.NotNil(t, logger)
	assert.Equal(t, "gopay", logger.service)
}

func TestGlobalLoggerConvenienceFunctions(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	// Initialize with console disabled to avoid output during tests
	InitGlobalLogger(nil)
	globalLogger.enableConsole = false

	// Test convenience functions
	Debug("Debug message")
	Info("Info message")
	Warn("Warning message")
	Error("Error message", nil)

	// Test with context
	ctx := LogContext{TenantID: "APP1"}
	Debug("Debug with context", ctx)
	Info("Info with context", ctx)
	Warn("Warning with context", ctx)
	Error("Error with context", nil, ctx)

	// No assertions needed as we're just testing that methods don't panic
}

func TestWithContext(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	InitGlobalLogger(nil)

	ctx := LogContext{
		TenantID: "APP1",
		Provider: "iyzico",
	}

	contextLogger := WithContext(ctx)
	assert.NotNil(t, contextLogger)
	assert.Equal(t, "APP1", contextLogger.context.TenantID)
	assert.Equal(t, "iyzico", contextLogger.context.Provider)
}

func TestWithTenant(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	InitGlobalLogger(nil)

	contextLogger := WithTenant("APP1")
	assert.NotNil(t, contextLogger)
	assert.Equal(t, "APP1", contextLogger.context.TenantID)
}

func TestWithProvider(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	InitGlobalLogger(nil)

	contextLogger := WithProvider("iyzico")
	assert.NotNil(t, contextLogger)
	assert.Equal(t, "iyzico", contextLogger.context.Provider)
}

func TestWithTenantAndProvider(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	InitGlobalLogger(nil)

	contextLogger := WithTenantAndProvider("APP1", "iyzico")
	assert.NotNil(t, contextLogger)
	assert.Equal(t, "APP1", contextLogger.context.TenantID)
	assert.Equal(t, "iyzico", contextLogger.context.Provider)
}

func TestInitGlobalLogger_OnlyOnce(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	// Initialize multiple times
	InitGlobalLogger(nil)
	firstLogger := globalLogger

	InitGlobalLogger(nil)
	secondLogger := globalLogger

	// Should be the same instance due to sync.Once
	assert.Equal(t, firstLogger, secondLogger)
}

func TestGlobalLogger_EnvironmentConfiguration(t *testing.T) {
	// Reset global state for testing
	globalLogger = nil
	once = sync.Once{}

	// Test with development environment
	InitGlobalLogger(nil)

	// In development, min level should be Debug
	assert.Equal(t, LevelDebug, globalLogger.minLevel)
}

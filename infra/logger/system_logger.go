package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/opensearch"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelFatal LogLevel = "fatal"
)

// SystemLog represents a structured system log entry
type SystemLog struct {
	Timestamp   time.Time      `json:"timestamp"`
	Level       LogLevel       `json:"level"`
	Message     string         `json:"message"`
	Component   string         `json:"component"`
	Function    string         `json:"function"`
	File        string         `json:"file"`
	Line        int            `json:"line"`
	TenantID    string         `json:"tenant_id,omitempty"`
	Provider    string         `json:"provider,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	Error       string         `json:"error,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Environment string         `json:"environment"`
	Service     string         `json:"service"`
	Version     string         `json:"version"`
}

// SystemLogger handles structured logging to OpenSearch and console
type SystemLogger struct {
	openSearchLogger *opensearch.Logger
	enableConsole    bool
	enableOpenSearch bool
	minLevel         LogLevel
	service          string
	version          string
	environment      string
}

// NewSystemLogger creates a new system logger
func NewSystemLogger(openSearchLogger *opensearch.Logger, config SystemLoggerConfig) *SystemLogger {
	return &SystemLogger{
		openSearchLogger: openSearchLogger,
		enableConsole:    config.EnableConsole,
		enableOpenSearch: config.EnableOpenSearch && openSearchLogger != nil,
		minLevel:         config.MinLevel,
		service:          config.Service,
		version:          config.Version,
		environment:      config.Environment,
	}
}

// SystemLoggerConfig represents configuration for system logger
type SystemLoggerConfig struct {
	EnableConsole    bool     `yaml:"enable_console"`
	EnableOpenSearch bool     `yaml:"enable_opensearch"`
	MinLevel         LogLevel `yaml:"min_level"`
	Service          string   `yaml:"service"`
	Version          string   `yaml:"version"`
	Environment      string   `yaml:"environment"`
}

// LogContext holds contextual information for logging
type LogContext struct {
	TenantID  string
	Provider  string
	RequestID string
	Fields    map[string]any
}

// Debug logs a debug message
func (sl *SystemLogger) Debug(message string, ctx ...LogContext) {
	sl.log(LevelDebug, message, ctx...)
}

// Info logs an info message
func (sl *SystemLogger) Info(message string, ctx ...LogContext) {
	sl.log(LevelInfo, message, ctx...)
}

// Warn logs a warning message
func (sl *SystemLogger) Warn(message string, ctx ...LogContext) {
	sl.log(LevelWarn, message, ctx...)
}

// Error logs an error message
func (sl *SystemLogger) Error(message string, err error, ctx ...LogContext) {
	logCtx := LogContext{}
	if len(ctx) > 0 {
		logCtx = ctx[0]
	}

	if logCtx.Fields == nil {
		logCtx.Fields = make(map[string]any)
	}

	if err != nil {
		logCtx.Fields["error"] = err.Error()
	}

	sl.log(LevelError, message, logCtx)
}

// Fatal logs a fatal message and exits
func (sl *SystemLogger) Fatal(message string, err error, ctx ...LogContext) {
	sl.Error(message, err, ctx...)
	os.Exit(1)
}

// log is the core logging function
func (sl *SystemLogger) log(level LogLevel, message string, ctx ...LogContext) {
	if !sl.shouldLog(level) {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Extract function name
	pc, _, _, ok := runtime.Caller(3)
	function := "unknown"
	if ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			function = fn.Name()
			// Clean up function name
			if idx := strings.LastIndex(function, "."); idx != -1 {
				function = function[idx+1:]
			}
		}
	}

	// Extract component from file path
	component := sl.extractComponent(file)

	// Build log entry
	logEntry := SystemLog{
		Timestamp:   time.Now().UTC(),
		Level:       level,
		Message:     message,
		Component:   component,
		Function:    function,
		File:        file,
		Line:        line,
		Environment: sl.environment,
		Service:     sl.service,
		Version:     sl.version,
	}

	// Add context if provided
	if len(ctx) > 0 {
		logCtx := ctx[0]
		logEntry.TenantID = logCtx.TenantID
		logEntry.Provider = logCtx.Provider
		logEntry.RequestID = logCtx.RequestID
		logEntry.Fields = logCtx.Fields

		// Extract error from fields
		if logCtx.Fields != nil {
			if errMsg, ok := logCtx.Fields["error"].(string); ok {
				logEntry.Error = errMsg
			}
		}
	}

	// Log to console if enabled
	if sl.enableConsole {
		sl.logToConsole(logEntry)
	}

	// Log to OpenSearch if enabled
	if sl.enableOpenSearch {
		go sl.logToOpenSearch(logEntry)
	}
}

// shouldLog checks if the log level should be logged
func (sl *SystemLogger) shouldLog(level LogLevel) bool {
	levelOrder := map[LogLevel]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
		LevelFatal: 4,
	}

	return levelOrder[level] >= levelOrder[sl.minLevel]
}

// extractComponent extracts component name from file path
func (sl *SystemLogger) extractComponent(file string) string {
	// Extract component from file path
	// e.g., /path/to/gopay/provider/iyzico/iyzico.go -> provider/iyzico
	parts := strings.Split(file, "/")

	for i, part := range parts {
		if part == "gopay" && i+1 < len(parts) {
			if i+2 < len(parts) {
				return parts[i+1] + "/" + parts[i+2]
			}
			return parts[i+1]
		}
	}

	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	return "unknown"
}

// logToConsole logs to console with colored output
func (sl *SystemLogger) logToConsole(entry SystemLog) {
	// Color codes
	colors := map[LogLevel]string{
		LevelDebug: "\033[36m", // Cyan
		LevelInfo:  "\033[32m", // Green
		LevelWarn:  "\033[33m", // Yellow
		LevelError: "\033[31m", // Red
		LevelFatal: "\033[35m", // Magenta
	}

	reset := "\033[0m"

	// Format timestamp
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	// Build context string
	var contextParts []string
	if entry.TenantID != "" {
		contextParts = append(contextParts, fmt.Sprintf("tenant=%s", entry.TenantID))
	}
	if entry.Provider != "" {
		contextParts = append(contextParts, fmt.Sprintf("provider=%s", entry.Provider))
	}
	if entry.RequestID != "" {
		contextParts = append(contextParts, fmt.Sprintf("req_id=%s", entry.RequestID[:8]))
	}

	context := ""
	if len(contextParts) > 0 {
		context = fmt.Sprintf("[%s] ", strings.Join(contextParts, " "))
	}

	// Log format: [TIMESTAMP] [LEVEL] [COMPONENT] [CONTEXT] MESSAGE
	color := colors[entry.Level]
	levelStr := strings.ToUpper(string(entry.Level))

	fmt.Printf("%s[%s] [%s] [%s] %s%s%s\n",
		timestamp,
		color+levelStr+reset,
		entry.Component,
		context,
		entry.Message,
		func() string {
			if entry.Error != "" {
				return fmt.Sprintf(" - Error: %s", entry.Error)
			}
			return ""
		}(),
		reset,
	)

	// Print fields if any
	if len(entry.Fields) > 0 {
		for key, value := range entry.Fields {
			if key != "error" { // Error already printed above
				fmt.Printf("  %s: %v\n", key, value)
			}
		}
	}
}

// logToOpenSearch logs to OpenSearch asynchronously
func (sl *SystemLogger) logToOpenSearch(entry SystemLog) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sl.openSearchLogger.LogSystemEvent(ctx, entry); err != nil {
		// Fallback to standard log if OpenSearch fails
		log.Printf("Failed to log to OpenSearch: %v", err)
	}
}

// WithContext creates a new logger with context
func (sl *SystemLogger) WithContext(ctx LogContext) *ContextLogger {
	return &ContextLogger{
		systemLogger: sl,
		context:      ctx,
	}
}

// ContextLogger wraps SystemLogger with context
type ContextLogger struct {
	systemLogger *SystemLogger
	context      LogContext
}

// Debug logs a debug message with context
func (cl *ContextLogger) Debug(message string) {
	cl.systemLogger.Debug(message, cl.context)
}

// Info logs an info message with context
func (cl *ContextLogger) Info(message string) {
	cl.systemLogger.Info(message, cl.context)
}

// Warn logs a warning message with context
func (cl *ContextLogger) Warn(message string) {
	cl.systemLogger.Warn(message, cl.context)
}

// Error logs an error message with context
func (cl *ContextLogger) Error(message string, err error) {
	cl.systemLogger.Error(message, err, cl.context)
}

// Fatal logs a fatal message with context and exits
func (cl *ContextLogger) Fatal(message string, err error) {
	cl.systemLogger.Fatal(message, err, cl.context)
}

// AddField adds a field to the context
func (cl *ContextLogger) AddField(key string, value any) *ContextLogger {
	if cl.context.Fields == nil {
		cl.context.Fields = make(map[string]any)
	}
	cl.context.Fields[key] = value
	return cl
}

// SetTenantID sets the tenant ID in context
func (cl *ContextLogger) SetTenantID(tenantID string) *ContextLogger {
	cl.context.TenantID = tenantID
	return cl
}

// SetProvider sets the provider in context
func (cl *ContextLogger) SetProvider(provider string) *ContextLogger {
	cl.context.Provider = provider
	return cl
}

// SetRequestID sets the request ID in context
func (cl *ContextLogger) SetRequestID(requestID string) *ContextLogger {
	cl.context.RequestID = requestID
	return cl
}

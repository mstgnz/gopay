package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/mstgnz/gopay/infra/conn"
)

// PaymentLog represents a structured payment log entry
type PaymentLog struct {
	ID           int64          `json:"id,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	TenantID     int            `json:"tenant_id,omitempty"`
	Provider     string         `json:"provider"`
	Method       string         `json:"method"`
	Endpoint     string         `json:"endpoint"`
	RequestID    string         `json:"request_id"`
	UserAgent    string         `json:"user_agent,omitempty"`
	ClientIP     string         `json:"client_ip,omitempty"`
	Request      map[string]any `json:"request"`
	Response     map[string]any `json:"response"`
	PaymentInfo  *PaymentInfo   `json:"payment_info,omitempty"`
	Error        *ErrorInfo     `json:"error,omitempty"`
	ProcessingMs int64          `json:"processing_ms"`
}

// PaymentInfo represents payment-specific information
type PaymentInfo struct {
	PaymentID     string  `json:"payment_id,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	CustomerEmail string  `json:"customer_email,omitempty"`
	Status        string  `json:"status,omitempty"`
	Use3D         bool    `json:"use_3d,omitempty"`
}

// ErrorInfo represents error details
type ErrorInfo struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// SystemLog represents a structured system log entry
type SystemLog struct {
	ID        int64          `json:"id,omitempty"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Component string         `json:"component,omitempty"`
	TenantID  *int           `json:"tenant_id,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Logger handles PostgreSQL logging operations
type Logger struct {
	db *sql.DB
}

// NewLogger creates a new PostgreSQL logger
func NewLogger(db *conn.DB) *Logger {
	return &Logger{
		db: db.DB,
	}
}

// LogPaymentRequest logs a payment request to PostgreSQL
func (l *Logger) LogPaymentRequest(ctx context.Context, logEntry PaymentLog) error {
	// Convert maps to JSON strings
	requestJSON, err := json.Marshal(logEntry.Request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	responseJSON, err := json.Marshal(logEntry.Response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Determine which table to use based on provider
	tableName := l.getProviderTableName(logEntry.Provider)

	// Insert into provider-specific table
	query := fmt.Sprintf(`
		INSERT INTO %s (tenant_id, request, response, request_at, response_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, tableName)

	var responseAt *time.Time
	if timestamp, ok := logEntry.Response["timestamp"].(time.Time); ok && !timestamp.IsZero() {
		responseAt = &timestamp
	}

	var id int64
	err = l.db.QueryRowContext(ctx, query,
		logEntry.TenantID,
		string(requestJSON),
		string(responseJSON),
		logEntry.Timestamp,
		responseAt,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to insert payment log: %w", err)
	}

	log.Printf("Logged payment request to %s with ID: %d", tableName, id)
	return nil
}

// LogSystemEvent logs a system event to PostgreSQL
func (l *Logger) LogSystemEvent(ctx context.Context, logEntry SystemLog) error {

	// Insert into system_logs table
	query := `
		INSERT INTO system_logs (level, log, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		RETURNING id
	`

	// Prepare log JSON structure
	logData := map[string]any{
		"level":     logEntry.Level,
		"message":   logEntry.Message,
		"component": logEntry.Component,
		"data":      logEntry.Data,
	}

	if logEntry.TenantID != nil {
		logData["tenant_id"] = *logEntry.TenantID
	}

	logJSON, err := json.Marshal(logData)
	if err != nil {
		return fmt.Errorf("failed to marshal system log: %w", err)
	}

	var id int64
	err = l.db.QueryRowContext(ctx, query, logEntry.Level, string(logJSON)).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to insert system log: %w", err)
	}

	return nil
}

// SearchPaymentLogs searches for payment logs based on criteria
func (l *Logger) SearchPaymentLogs(ctx context.Context, tenantID int, provider string, filters map[string]any) ([]PaymentLog, error) {
	tableName := l.getProviderTableName(provider)

	// Build query with filters
	query := fmt.Sprintf(`
		SELECT id, tenant_id, request, response, request_at, response_at
		FROM %s
		WHERE tenant_id = $1
	`, tableName)

	args := []any{tenantID}
	argIndex := 2

	// Add date range filter if provided
	if startDate, ok := filters["start_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND request_at >= $%d", argIndex)
		args = append(args, startDate)
		argIndex++
	}

	if endDate, ok := filters["end_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND request_at <= $%d", argIndex)
		args = append(args, endDate)
		argIndex++
	}

	// Add payment ID filter if provided
	if paymentID, ok := filters["payment_id"].(string); ok {
		query += fmt.Sprintf(" AND request::text LIKE $%d", argIndex)
		args = append(args, "%"+paymentID+"%")
		argIndex++
	}

	query += " ORDER BY request_at DESC LIMIT 100"

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search payment logs: %w", err)
	}
	defer rows.Close()

	var logs []PaymentLog
	for rows.Next() {
		var log PaymentLog
		var requestJSON, responseJSON string
		var responseAt *time.Time

		err := rows.Scan(
			&log.ID,
			&log.TenantID,
			&requestJSON,
			&responseJSON,
			&log.Timestamp,
			&responseAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment log row: %w", err)
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(requestJSON), &log.Request); err != nil {
			log.Request = map[string]any{"raw": requestJSON}
		}

		if err := json.Unmarshal([]byte(responseJSON), &log.Response); err != nil {
			log.Response = map[string]any{"raw": responseJSON}
		}

		log.Provider = provider
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payment log rows: %w", err)
	}

	return logs, nil
}

// SearchSystemLogs searches for system logs based on criteria
func (l *Logger) SearchSystemLogs(ctx context.Context, filters map[string]any) ([]SystemLog, error) {
	query := `
		SELECT id, level, log, created_at
		FROM system_logs
		WHERE 1=1
	`

	args := []any{}
	argIndex := 1

	// Add level filter if provided
	if level, ok := filters["level"].(string); ok {
		query += fmt.Sprintf(" AND level = $%d", argIndex)
		args = append(args, level)
		argIndex++
	}

	// Add date range filter if provided
	if startDate, ok := filters["start_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, startDate)
		argIndex++
	}

	if endDate, ok := filters["end_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, endDate)
		argIndex++
	}

	// Add component filter if provided
	if component, ok := filters["component"].(string); ok {
		query += fmt.Sprintf(" AND log::text LIKE $%d", argIndex)
		args = append(args, "%"+component+"%")
		argIndex++
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search system logs: %w", err)
	}
	defer rows.Close()

	var logs []SystemLog
	for rows.Next() {
		var log SystemLog
		var logJSON string

		err := rows.Scan(
			&log.ID,
			&log.Level,
			&logJSON,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan system log row: %w", err)
		}

		// Parse JSON log data
		var logData map[string]any
		if err := json.Unmarshal([]byte(logJSON), &logData); err != nil {
			log.Data = map[string]any{"raw": logJSON}
		} else {
			if message, ok := logData["message"].(string); ok {
				log.Message = message
			}
			if component, ok := logData["component"].(string); ok {
				log.Component = component
			}
			if data, ok := logData["data"].(map[string]any); ok {
				log.Data = data
			}
			if tenantIDFloat, ok := logData["tenant_id"].(float64); ok {
				tenantID := int(tenantIDFloat)
				log.TenantID = &tenantID
			}
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating system log rows: %w", err)
	}

	return logs, nil
}

// GetPaymentStats retrieves payment statistics for a provider
func (l *Logger) GetPaymentStats(ctx context.Context, tenantID int, provider string, hours int) (map[string]any, error) {
	tableName := l.getProviderTableName(provider)

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_requests,
			COUNT(CASE WHEN response::text LIKE '%%"success":true%%' THEN 1 END) as success_count,
			COUNT(CASE WHEN response::text LIKE '%%"success":false%%' THEN 1 END) as error_count,
			AVG(EXTRACT(EPOCH FROM (response_at - request_at)) * 1000) as avg_processing_ms
		FROM %s
		WHERE tenant_id = $1 
		AND request_at >= NOW() - INTERVAL '%d hours'
	`, tableName, hours)

	var stats struct {
		TotalRequests   int      `json:"total_requests"`
		SuccessCount    int      `json:"success_count"`
		ErrorCount      int      `json:"error_count"`
		AvgProcessingMs *float64 `json:"avg_processing_ms"`
	}

	err := l.db.QueryRowContext(ctx, query, tenantID).Scan(
		&stats.TotalRequests,
		&stats.SuccessCount,
		&stats.ErrorCount,
		&stats.AvgProcessingMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment stats: %w", err)
	}

	result := map[string]any{
		"total_requests": stats.TotalRequests,
		"success_count":  stats.SuccessCount,
		"error_count":    stats.ErrorCount,
		"success_rate":   0.0,
	}

	if stats.TotalRequests > 0 {
		result["success_rate"] = float64(stats.SuccessCount) / float64(stats.TotalRequests) * 100
	}

	if stats.AvgProcessingMs != nil {
		result["avg_processing_ms"] = *stats.AvgProcessingMs
	}

	return result, nil
}

// getProviderTableName returns the PostgreSQL table name for a provider
func (l *Logger) getProviderTableName(provider string) string {
	// Map provider names to table names
	providerTables := map[string]string{
		"iyzico":  "iyzico",
		"ozanpay": "ozanpay",
		"paycell": "paycell",
		"stripe":  "stripe",
		"papara":  "papara",
		"nkolay":  "nkolay",
		"paytr":   "paytr",
		"payu":    "payu",
		"shopier": "shopier",
	}

	if tableName, exists := providerTables[strings.ToLower(provider)]; exists {
		return tableName
	}

	// Default fallback
	return "payment_logs"
}

// SanitizeForLog removes sensitive information from data before logging
func SanitizeForLog(data map[string]any) map[string]any {
	sanitized := make(map[string]any)

	sensitiveFields := []string{
		"cardNumber", "card_number", "cvv", "cvc", "cardHolderName", "card_holder_name",
		"apiKey", "api_key", "secretKey", "secret_key", "password", "token",
		"authorization", "x-api-key", "x-secret-key",
	}

	for key, value := range data {
		shouldSanitize := false
		keyLower := strings.ToLower(key)

		for _, sensitiveField := range sensitiveFields {
			if strings.Contains(keyLower, strings.ToLower(sensitiveField)) {
				shouldSanitize = true
				break
			}
		}

		if shouldSanitize {
			if strValue, ok := value.(string); ok && len(strValue) > 4 {
				sanitized[key] = strValue[:2] + "***" + strValue[len(strValue)-2:]
			} else {
				sanitized[key] = "***REDACTED***"
			}
		} else {
			sanitized[key] = value
		}
	}

	return sanitized
}

// GetTenantIDFromString converts string tenant ID to integer
func GetTenantIDFromString(tenantIDStr string) (int, error) {
	if tenantIDStr == "" || tenantIDStr == "legacy" {
		return 0, fmt.Errorf("invalid tenant ID")
	}

	tenantID, err := strconv.Atoi(tenantIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid tenant ID format: %w", err)
	}

	return tenantID, nil
}

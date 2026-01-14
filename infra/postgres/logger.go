package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/mstgnz/gopay/infra/conn"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
		//nolint:gosec
		argIndex++
	}

	if endDate, ok := filters["end_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND request_at <= $%d", argIndex)
		args = append(args, endDate)
		//nolint:gosec
		argIndex++
	}

	// Add payment ID filter if provided
	if paymentID, ok := filters["payment_id"].(string); ok {
		query += fmt.Sprintf(" AND request::text LIKE $%d", argIndex)
		args = append(args, "%"+paymentID+"%")
		//nolint:gosec
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
		//nolint:gosec
		argIndex++
	}

	// Add date range filter if provided
	if startDate, ok := filters["start_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, startDate)
		//nolint:gosec
		argIndex++
	}

	if endDate, ok := filters["end_date"].(time.Time); ok {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, endDate)
		//nolint:gosec
		argIndex++
	}

	// Add component filter if provided
	if component, ok := filters["component"].(string); ok {
		query += fmt.Sprintf(" AND log::text LIKE $%d", argIndex)
		args = append(args, "%"+component+"%")
		//nolint:gosec
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
	// Validate hours parameter to prevent SQL injection
	if hours <= 0 || hours > 8760 { // Max 1 year (365*24 hours)
		return nil, fmt.Errorf("invalid hours parameter: must be between 1 and 8760")
	}

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
	result := sanitizeRecursive(data)
	if sanitizedMap, ok := result.(map[string]any); ok {
		return sanitizedMap
	}
	return data // fallback to original data if type assertion fails
}

// sanitizeRecursive recursively sanitizes nested objects and arrays
func sanitizeRecursive(data any) any {
	switch v := data.(type) {
	case map[string]any:
		return sanitizeMap(v)
	case []any:
		return sanitizeSlice(v)
	case []map[string]any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = sanitizeRecursive(item)
		}
		return result
	default:
		return v
	}
}

// sanitizeMap sanitizes a map of string to any
func sanitizeMap(data map[string]any) map[string]any {
	sanitized := make(map[string]any)

	// Define sensitive field patterns
	sensitiveFields := []string{
		"cardnumber", "card_number", "credit", "pan",
	}

	for key, value := range data {
		keyLower := strings.ToLower(key)
		shouldSanitize := false
		isCardNumber := false
		isCVV := false

		// Check if field should be sanitized
		for _, sensitiveField := range sensitiveFields {
			if strings.Contains(keyLower, sensitiveField) {
				shouldSanitize = true
				if strings.Contains(keyLower, "cardnumber") || strings.Contains(keyLower, "card_number") {
					isCardNumber = true
				}
				if strings.Contains(keyLower, "cvv") || strings.Contains(keyLower, "cvc") {
					isCVV = true
				}
				break
			}
		}

		if shouldSanitize {
			if strValue, ok := value.(string); ok {
				if isCardNumber {
					sanitized[key] = maskCardNumber(strValue)
				} else if isCVV {
					sanitized[key] = "***"
				} else {
					sanitized[key] = maskGenericSensitive(strValue)
				}
			} else {
				sanitized[key] = "***REDACTED***"
			}
		} else {
			// Recursively sanitize nested objects
			sanitized[key] = sanitizeRecursive(value)
		}
	}

	return sanitized
}

// sanitizeSlice sanitizes a slice of any type
func sanitizeSlice(data []any) []any {
	sanitized := make([]any, len(data))
	for i, item := range data {
		sanitized[i] = sanitizeRecursive(item)
	}
	return sanitized
}

// maskCardNumber masks a card number showing only first 4 and last 4 digits
func maskCardNumber(cardNumber string) string {
	if len(cardNumber) <= 8 {
		return "****"
	}

	// Remove any spaces or dashes
	cleaned := strings.ReplaceAll(strings.ReplaceAll(cardNumber, " ", ""), "-", "")

	if len(cleaned) <= 8 {
		return "****"
	}

	return cleaned[:4] + "********" + cleaned[len(cleaned)-4:]
}

// maskGenericSensitive masks generic sensitive data
func maskGenericSensitive(value string) string {
	if len(value) <= 4 {
		return "***REDACTED***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}

// GetPaymentStatsComparison retrieves payment statistics comparison between two periods
func (l *Logger) GetPaymentStatsComparison(ctx context.Context, tenantID int, provider string, currentHours, previousHours int) (map[string]any, error) {
	// Validate hours parameters to prevent SQL injection
	if currentHours <= 0 || currentHours > 8760 {
		return nil, fmt.Errorf("invalid currentHours parameter: must be between 1 and 8760")
	}
	if previousHours <= 0 || previousHours > 8760 {
		return nil, fmt.Errorf("invalid previousHours parameter: must be between 1 and 8760")
	}

	if l.db == nil {
		return map[string]any{
			"current_total":          0,
			"current_success":        0,
			"current_volume":         0.0,
			"current_processing_ms":  0.0,
			"previous_total":         0,
			"previous_success":       0,
			"previous_volume":        0.0,
			"previous_processing_ms": 0.0,
		}, nil
	}

	tableName := l.getProviderTableName(provider)

	query := fmt.Sprintf(`
		WITH current_period AS (
			SELECT 
				COUNT(*) as total_requests,
				COUNT(CASE WHEN response::text LIKE '%%"success":true%%' THEN 1 END) as success_count,
				SUM(CASE WHEN amount IS NOT NULL THEN amount ELSE 0 END) as total_volume,
				AVG(CASE WHEN processing_ms IS NOT NULL THEN processing_ms ELSE 0 END) as avg_processing_ms
			FROM %s
			WHERE tenant_id = $1 
			AND request_at >= NOW() - INTERVAL '%d hours'
		),
		previous_period AS (
			SELECT 
				COUNT(*) as total_requests,
				COUNT(CASE WHEN response::text LIKE '%%"success":true%%' THEN 1 END) as success_count,
				SUM(CASE WHEN amount IS NOT NULL THEN amount ELSE 0 END) as total_volume,
				AVG(CASE WHEN processing_ms IS NOT NULL THEN processing_ms ELSE 0 END) as avg_processing_ms
			FROM %s
			WHERE tenant_id = $1 
			AND request_at >= NOW() - INTERVAL '%d hours'
			AND request_at < NOW() - INTERVAL '%d hours'
		)
		SELECT 
			c.total_requests as current_total,
			c.success_count as current_success,
			c.total_volume as current_volume,
			c.avg_processing_ms as current_processing_ms,
			p.total_requests as previous_total,
			p.success_count as previous_success,
			p.total_volume as previous_volume,
			p.avg_processing_ms as previous_processing_ms
		FROM current_period c, previous_period p
	`, tableName, currentHours, tableName, previousHours+currentHours, currentHours)

	var stats struct {
		CurrentTotal         int      `json:"current_total"`
		CurrentSuccess       int      `json:"current_success"`
		CurrentVolume        *float64 `json:"current_volume"`
		CurrentProcessingMs  *float64 `json:"current_processing_ms"`
		PreviousTotal        int      `json:"previous_total"`
		PreviousSuccess      int      `json:"previous_success"`
		PreviousVolume       *float64 `json:"previous_volume"`
		PreviousProcessingMs *float64 `json:"previous_processing_ms"`
	}

	err := l.db.QueryRowContext(ctx, query, tenantID).Scan(
		&stats.CurrentTotal,
		&stats.CurrentSuccess,
		&stats.CurrentVolume,
		&stats.CurrentProcessingMs,
		&stats.PreviousTotal,
		&stats.PreviousSuccess,
		&stats.PreviousVolume,
		&stats.PreviousProcessingMs,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return zero stats if no data found
			return map[string]any{
				"current_total":          0,
				"current_success":        0,
				"current_volume":         0.0,
				"current_processing_ms":  0.0,
				"previous_total":         0,
				"previous_success":       0,
				"previous_volume":        0.0,
				"previous_processing_ms": 0.0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get payment stats comparison: %w", err)
	}

	result := map[string]any{
		"current_total":    stats.CurrentTotal,
		"current_success":  stats.CurrentSuccess,
		"previous_total":   stats.PreviousTotal,
		"previous_success": stats.PreviousSuccess,
	}

	if stats.CurrentVolume != nil {
		result["current_volume"] = *stats.CurrentVolume
	} else {
		result["current_volume"] = 0.0
	}

	if stats.CurrentProcessingMs != nil {
		result["current_processing_ms"] = *stats.CurrentProcessingMs
	} else {
		result["current_processing_ms"] = 0.0
	}

	if stats.PreviousVolume != nil {
		result["previous_volume"] = *stats.PreviousVolume
	} else {
		result["previous_volume"] = 0.0
	}

	if stats.PreviousProcessingMs != nil {
		result["previous_processing_ms"] = *stats.PreviousProcessingMs
	} else {
		result["previous_processing_ms"] = 0.0
	}

	return result, nil
}

// GetPaymentTrendsMonthly retrieves daily payment trends for a specific month/year
func (l *Logger) GetPaymentTrendsMonthly(ctx context.Context, tenantID int, provider string, month, year int) (map[string]any, error) {
	// Validate month and year parameters
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month parameter: must be between 1 and 12")
	}
	if year < 2020 || year > 2030 {
		return nil, fmt.Errorf("invalid year parameter: must be between 2020 and 2030")
	}

	if l.db == nil {
		return map[string]any{
			"labels": []string{},
			"datasets": []map[string]any{
				{
					"label":           "Successful Payments",
					"data":            []int{},
					"borderColor":     "#10B981",
					"backgroundColor": "rgba(16, 185, 129, 0.1)",
				},
				{
					"label":           "Failed Payments",
					"data":            []int{},
					"borderColor":     "#EF4444",
					"backgroundColor": "rgba(239, 68, 68, 0.1)",
				},
			},
			"volume": []float64{},
		}, nil
	}

	tableName := l.getProviderTableName(provider)

	// Get the first and last day of the month
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, 0).Add(-time.Second) // Last second of the month

	query := fmt.Sprintf(`
		WITH daily_data AS (
			SELECT 
				DATE_TRUNC('day', request_at) as day,
				COUNT(*) as total_payments,
				COUNT(CASE WHEN response::text LIKE '%%"success":true%%' THEN 1 END) as successful_payments,
				COUNT(CASE WHEN response::text LIKE '%%"success":false%%' THEN 1 END) as failed_payments,
				SUM(CASE WHEN amount IS NOT NULL THEN amount ELSE 0 END) as volume
			FROM %s
			WHERE tenant_id = $1 
			AND request_at >= $2
			AND request_at <= $3
			GROUP BY DATE_TRUNC('day', request_at)
			ORDER BY day ASC
		)
		SELECT 
			day,
			total_payments,
			successful_payments,
			failed_payments,
			volume
		FROM daily_data
	`, tableName)

	rows, err := l.db.QueryContext(ctx, query, tenantID, firstDay, lastDay)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly payment trends: %w", err)
	}
	defer rows.Close()

	var labels []string
	var successData []int
	var failedData []int
	var volumeData []float64

	// Create a map to store data by day for filling gaps
	dailyDataMap := make(map[string]struct {
		successful int
		failed     int
		volume     float64
	})

	for rows.Next() {
		var day time.Time
		var total, successful, failed int
		var volume float64

		err := rows.Scan(&day, &total, &successful, &failed, &volume)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trend row: %w", err)
		}

		dayKey := day.Format("2006-01-02")
		dailyDataMap[dayKey] = struct {
			successful int
			failed     int
			volume     float64
		}{successful, failed, volume}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trend rows: %w", err)
	}

	// Generate all days in the month and fill with data or zeros
	currentDay := firstDay
	for currentDay.Month() == time.Month(month) && currentDay.Year() == year {
		dayKey := currentDay.Format("2006-01-02")
		dayLabel := currentDay.Format("Jan 2") // e.g., "Jan 1", "Jan 2"

		labels = append(labels, dayLabel)

		if data, exists := dailyDataMap[dayKey]; exists {
			successData = append(successData, data.successful)
			failedData = append(failedData, data.failed)
			volumeData = append(volumeData, data.volume)
		} else {
			// No data for this day, fill with zeros
			successData = append(successData, 0)
			failedData = append(failedData, 0)
			volumeData = append(volumeData, 0.0)
		}

		currentDay = currentDay.AddDate(0, 0, 1)
	}

	return map[string]any{
		"labels": labels,
		"datasets": []map[string]any{
			{
				"label":           "Successful Payments",
				"data":            successData,
				"borderColor":     "#10B981",
				"backgroundColor": "rgba(16, 185, 129, 0.1)",
			},
			{
				"label":           "Failed Payments",
				"data":            failedData,
				"borderColor":     "#EF4444",
				"backgroundColor": "rgba(239, 68, 68, 0.1)",
			},
		},
		"volume": volumeData,
	}, nil
}

// GetPaymentTrends retrieves hourly payment trends for analytics
func (l *Logger) GetPaymentTrends(ctx context.Context, tenantID int, provider string, hours int) (map[string]any, error) {
	// Validate hours parameter to prevent SQL injection
	if hours <= 0 || hours > 8760 { // Max 1 year (365*24 hours)
		return nil, fmt.Errorf("invalid hours parameter: must be between 1 and 8760")
	}

	if l.db == nil {
		return map[string]any{
			"labels": []string{},
			"datasets": []map[string]any{
				{
					"label":           "Successful Payments",
					"data":            []int{},
					"borderColor":     "#10B981",
					"backgroundColor": "rgba(16, 185, 129, 0.1)",
				},
				{
					"label":           "Failed Payments",
					"data":            []int{},
					"borderColor":     "#EF4444",
					"backgroundColor": "rgba(239, 68, 68, 0.1)",
				},
			},
			"volume": []float64{},
		}, nil
	}

	tableName := l.getProviderTableName(provider)

	query := fmt.Sprintf(`
		WITH hourly_data AS (
			SELECT 
				DATE_TRUNC('hour', request_at) as hour,
				COUNT(*) as total_payments,
				COUNT(CASE WHEN response::text LIKE '%%"success":true%%' THEN 1 END) as successful_payments,
				COUNT(CASE WHEN response::text LIKE '%%"success":false%%' THEN 1 END) as failed_payments,
				SUM(CASE WHEN amount IS NOT NULL THEN amount ELSE 0 END) as volume
			FROM %s
			WHERE tenant_id = $1 
			AND request_at >= NOW() - INTERVAL '%d hours'
			GROUP BY DATE_TRUNC('hour', request_at)
			ORDER BY hour DESC
		)
		SELECT 
			hour,
			total_payments,
			successful_payments,
			failed_payments,
			volume
		FROM hourly_data
		LIMIT %d
	`, tableName, hours, hours)

	rows, err := l.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment trends: %w", err)
	}
	defer rows.Close()

	var labels []string
	var successData []int
	var failedData []int
	var volumeData []float64

	for rows.Next() {
		var hour time.Time
		var total, successful, failed int
		var volume float64

		err := rows.Scan(&hour, &total, &successful, &failed, &volume)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trend row: %w", err)
		}

		// Calculate hours ago from now
		hoursAgo := int(time.Since(hour).Hours())
		if hoursAgo == 0 {
			labels = append(labels, "Now")
		} else {
			labels = append(labels, fmt.Sprintf("%dh ago", hoursAgo))
		}

		successData = append(successData, successful)
		failedData = append(failedData, failed)
		volumeData = append(volumeData, volume)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trend rows: %w", err)
	}

	// Reverse arrays to show oldest to newest
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
		successData[i], successData[j] = successData[j], successData[i]
		failedData[i], failedData[j] = failedData[j], failedData[i]
		volumeData[i], volumeData[j] = volumeData[j], volumeData[i]
	}

	return map[string]any{
		"labels": labels,
		"datasets": []map[string]any{
			{
				"label":           "Successful Payments",
				"data":            successData,
				"borderColor":     "#10B981",
				"backgroundColor": "rgba(16, 185, 129, 0.1)",
			},
			{
				"label":           "Failed Payments",
				"data":            failedData,
				"borderColor":     "#EF4444",
				"backgroundColor": "rgba(239, 68, 68, 0.1)",
			},
		},
		"volume": volumeData,
	}, nil
}

// GetAllRecentActivity retrieves recent payment activity from all provider tables
func (l *Logger) GetAllRecentActivity(ctx context.Context, limit int) ([]map[string]any, error) {
	// Validate limit parameter
	if limit <= 0 || limit > 1000 {
		return nil, fmt.Errorf("invalid limit parameter: must be between 1 and 1000")
	}

	// Get all provider tables that exist
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	var allActivities []map[string]any

	for _, provider := range providers {
		tableName := l.getProviderTableName(provider)

		query := fmt.Sprintf(`
			SELECT 
				request_at,
				tenant_id,
				payment_id,
				amount,
				currency,
				CASE 
					WHEN response::text LIKE '%%"success":true%%' OR status = 'success' THEN 'success'
					WHEN response::text LIKE '%%"success":false%%' OR status = 'failed' THEN 'failed'
					ELSE 'processing'
				END as activity_status,
				method,
				endpoint,
				request,
				response
			FROM %s
			WHERE request_at >= NOW() - INTERVAL '24 hours'
			AND payment_id IS NOT NULL
			AND amount > 0
			ORDER BY request_at DESC
			LIMIT $1
		`, tableName)

		rows, err := l.db.QueryContext(ctx, query, limit)
		if err != nil {
			// Skip this provider if table doesn't exist or has error
			continue
		}

		for rows.Next() {
			var requestAt time.Time
			var tenantID int
			var paymentID, currency, activityStatus, method, endpoint sql.NullString
			var request, response sql.NullString
			var amount sql.NullFloat64

			err := rows.Scan(&requestAt, &tenantID, &paymentID, &amount, &currency, &activityStatus, &method, &endpoint, &request, &response)
			if err != nil {
				continue
			}

			// Calculate time ago
			timeAgo := time.Since(requestAt)
			var timeString string
			if timeAgo.Hours() >= 1 {
				timeString = fmt.Sprintf("%.0fh ago", timeAgo.Hours())
			} else {
				timeString = fmt.Sprintf("%.0fm ago", timeAgo.Minutes())
			}

			// Determine activity type
			activityType := "payment"
			if endpoint.Valid && (strings.Contains(endpoint.String, "refund") ||
				(method.Valid && strings.Contains(strings.ToLower(method.String), "refund"))) {
				activityType = "refund"
			}

			// Format amount with currency
			amountStr := "₺0.00"
			if amount.Valid && currency.Valid {
				amountStr = fmt.Sprintf("₺%.2f", amount.Float64)
			} else if amount.Valid {
				amountStr = fmt.Sprintf("₺%.2f", amount.Float64)
			}

			activity := map[string]any{
				"timestamp": requestAt,
				"time":      timeString,
				"provider":  cases.Title(language.English).String(provider),
				"type":      activityType,
				"status":    activityStatus.String,
				"amount":    amountStr,
				"tenant_id": tenantID,
				"id":        paymentID.String,
				"env":       "production", // Default, could be enhanced to detect from data
				"request":   request.String,
				"response":  response.String,
				"endpoint":  endpoint.String,
			}

			allActivities = append(allActivities, activity)
		}
		rows.Close()
	}

	// Sort all activities by timestamp (newest first)
	sort.Slice(allActivities, func(i, j int) bool {
		timeI := allActivities[i]["timestamp"].(time.Time)
		timeJ := allActivities[j]["timestamp"].(time.Time)
		return timeI.After(timeJ)
	})

	// Limit results
	if len(allActivities) > limit {
		allActivities = allActivities[:limit]
	}

	return allActivities, nil
}

// SearchPaymentByID searches for a specific payment by ID in a provider table
func (l *Logger) SearchPaymentByID(ctx context.Context, tenantID int, provider, paymentID string) ([]map[string]any, error) {
	tableName := l.getProviderTableName(provider)

	query := fmt.Sprintf(`
		SELECT 
			request_at,
			tenant_id,
			payment_id,
			amount,
			currency,
			CASE 
				WHEN response::text LIKE '%%"success":true%%' OR status = 'success' THEN 'success'
				WHEN response::text LIKE '%%"success":false%%' OR status = 'failed' THEN 'failed'
				ELSE 'processing'
			END as activity_status,
			method,
			endpoint,
			request,
			response
		FROM %s
		WHERE tenant_id = $1 
		AND payment_id = $2
		ORDER BY request_at DESC
	`, tableName)

	rows, err := l.db.QueryContext(ctx, query, tenantID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment: %w", err)
	}
	defer rows.Close()

	var payments []map[string]any

	for rows.Next() {
		var requestAt time.Time
		var returnedTenantID int
		var returnedPaymentID, currency, activityStatus, method, endpoint sql.NullString
		var request, response sql.NullString
		var amount sql.NullFloat64

		err := rows.Scan(&requestAt, &returnedTenantID, &returnedPaymentID, &amount, &currency, &activityStatus, &method, &endpoint, &request, &response)
		if err != nil {
			continue
		}

		// Calculate time ago
		timeAgo := time.Since(requestAt)
		var timeString string
		if timeAgo.Hours() >= 1 {
			timeString = fmt.Sprintf("%.0fh ago", timeAgo.Hours())
		} else {
			timeString = fmt.Sprintf("%.0fm ago", timeAgo.Minutes())
		}

		// Determine activity type
		activityType := "payment"
		if endpoint.Valid && (strings.Contains(endpoint.String, "refund") ||
			(method.Valid && strings.Contains(strings.ToLower(method.String), "refund"))) {
			activityType = "refund"
		}

		// Format amount with currency
		amountStr := "₺0.00"
		if amount.Valid && currency.Valid {
			amountStr = fmt.Sprintf("₺%.2f", amount.Float64)
		} else if amount.Valid {
			amountStr = fmt.Sprintf("₺%.2f", amount.Float64)
		}

		payment := map[string]any{
			"timestamp": requestAt,
			"time":      timeString,
			"provider":  cases.Title(language.English).String(provider),
			"type":      activityType,
			"status":    activityStatus.String,
			"amount":    amountStr,
			"tenant_id": returnedTenantID,
			"id":        returnedPaymentID.String,
			"env":       "production", // Default, could be enhanced to detect from data
			"request":   request.String,
			"response":  response.String,
			"endpoint":  endpoint.String,
		}

		payments = append(payments, payment)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payment rows: %w", err)
	}

	// Return all payments found
	return payments, nil
}

// GetAllProvidersStats retrieves stats for all providers for a tenant
func (l *Logger) GetAllProvidersStats(ctx context.Context, tenantID int, hours int) (map[string]map[string]any, error) {
	// Validate hours parameter (this will also be validated in GetPaymentStats but adding here for consistency)
	if hours <= 0 || hours > 8760 { // Max 1 year (365*24 hours)
		return nil, fmt.Errorf("invalid hours parameter: must be between 1 and 8760")
	}

	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	allStats := make(map[string]map[string]any)

	for _, provider := range providers {
		stats, err := l.GetPaymentStats(ctx, tenantID, provider, hours)
		if err != nil {
			// Continue with other providers if one fails
			continue
		}
		allStats[provider] = stats
	}

	return allStats, nil
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

// GetAllTenants retrieves all tenants with id and username from the database
func (l *Logger) GetAllTenants(ctx context.Context) ([]map[string]any, error) {
	query := `SELECT id, username FROM tenants ORDER BY id`

	rows, err := l.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenants: %w", err)
	}
	defer rows.Close()

	var tenants []map[string]any
	for rows.Next() {
		var id int
		var username string

		if err := rows.Scan(&id, &username); err != nil {
			return nil, fmt.Errorf("failed to scan tenant row: %w", err)
		}

		tenants = append(tenants, map[string]any{
			"id":   id,
			"name": username,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tenant rows: %w", err)
	}

	return tenants, nil
}

// GetActiveProviders retrieves providers that have tenant configurations
func (l *Logger) GetActiveProviders(ctx context.Context) ([]map[string]any, error) {
	query := `
		SELECT p.name, COUNT(DISTINCT tc.tenant_id) as tenant_count
		FROM providers p
		INNER JOIN tenant_configs tc ON p.id = tc.provider_id
		WHERE p.active = true
		GROUP BY p.id, p.name
		ORDER BY p.name`

	rows, err := l.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active providers: %w", err)
	}
	defer rows.Close()

	var providers []map[string]any
	for rows.Next() {
		var name string
		var tenantCount int

		if err := rows.Scan(&name, &tenantCount); err != nil {
			return nil, fmt.Errorf("failed to scan provider row: %w", err)
		}

		providers = append(providers, map[string]any{
			"id":           name,
			"name":         cases.Title(language.English).String(name),
			"tenant_count": tenantCount,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating provider rows: %w", err)
	}

	return providers, nil
}

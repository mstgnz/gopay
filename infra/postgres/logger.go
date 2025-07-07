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

// GetRecentPaymentActivity retrieves recent payment activity for analytics
func (l *Logger) GetRecentPaymentActivity(ctx context.Context, tenantID int, provider string, limit int) ([]map[string]any, error) {
	// Validate limit parameter
	if limit <= 0 || limit > 1000 { // Max 1000 records
		return nil, fmt.Errorf("invalid limit parameter: must be between 1 and 1000")
	}

	tableName := l.getProviderTableName(provider)

	query := fmt.Sprintf(`
		SELECT 
			request_at,
			payment_id,
			amount,
			currency,
			status,
			CASE 
				WHEN response::text LIKE '%%"success":true%%' THEN 'success'
				WHEN response::text LIKE '%%"success":false%%' THEN 'failed'
				ELSE 'processing'
			END as activity_status,
			method
		FROM %s
		WHERE tenant_id = $1 
		AND request_at >= NOW() - INTERVAL '24 hours'
		ORDER BY request_at DESC
		LIMIT $2
	`, tableName)

	rows, err := l.db.QueryContext(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}
	defer rows.Close()

	var activities []map[string]any

	for rows.Next() {
		var requestAt time.Time
		var paymentID, currency, status, activityStatus, method sql.NullString
		var amount sql.NullFloat64

		err := rows.Scan(&requestAt, &paymentID, &amount, &currency, &status, &activityStatus, &method)
		if err != nil {
			return nil, fmt.Errorf("failed to scan activity row: %w", err)
		}

		// Calculate time ago
		timeAgo := time.Since(requestAt)
		var timeString string
		if timeAgo.Hours() >= 1 {
			timeString = fmt.Sprintf("%.0fh ago", timeAgo.Hours())
		} else {
			timeString = fmt.Sprintf("%.0fm ago", timeAgo.Minutes())
		}

		activity := map[string]any{
			"timestamp": requestAt,
			"time":      timeString,
			"provider":  provider,
			"type":      "payment",
			"status":    activityStatus.String,
		}

		if paymentID.Valid {
			activity["id"] = paymentID.String
		}

		if amount.Valid && currency.Valid {
			activity["amount"] = fmt.Sprintf("%.2f %s", amount.Float64, currency.String)
		}

		activities = append(activities, activity)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activity rows: %w", err)
	}

	return activities, nil
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

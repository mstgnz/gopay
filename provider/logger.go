package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/conn"
	"github.com/mstgnz/gopay/infra/logger"
)

// PaymentLogger handles database logging for payment operations
type PaymentLogger interface {
	LogRequest(ctx context.Context, tenantID int, providerName string, method, endpoint string, request any, userAgent, clientIP string) (int64, error)
	LogResponse(ctx context.Context, logID int64, response any, processingMs int64) error
	LogError(ctx context.Context, logID int64, errorCode, errorMsg string, processingMs int64) error
}

// DBPaymentLogger implements PaymentLogger interface using SQL database
type DBPaymentLogger struct {
	db *conn.DB
	// Track which provider table each log ID belongs to for efficient updates
	logProviderMap map[int64]string
	mapMutex       sync.RWMutex
}

// NewDBPaymentLogger creates a new database payment logger
func NewDBPaymentLogger(db *conn.DB) PaymentLogger {
	return &DBPaymentLogger{
		db:             db,
		logProviderMap: make(map[int64]string),
	}
}

// LogRequest logs the payment request to the appropriate provider table
func (l *DBPaymentLogger) LogRequest(ctx context.Context, tenantID int, providerName string, method, endpoint string, request any, userAgent, clientIP string) (int64, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Generate payment ID and transaction ID from request if available
	var paymentID, transactionID string
	if req, ok := request.(PaymentRequest); ok {
		paymentID = req.ID
		if paymentID == "" {
			paymentID = req.ReferenceID
		}
		transactionID = req.ConversationID
	}

	// Clean provider name to get actual table name (remove tenant prefix if present)
	tableName, err := l.getActualProviderName(providerName)
	if err != nil {
		return 0, fmt.Errorf("invalid provider name: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (tenant_id, request, request_at, method, endpoint, payment_id, transaction_id, user_agent, client_ip)
		VALUES ($1, $2, NOW(), $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, tableName)

	var logID int64
	err = l.db.QueryRowContext(ctx, query, tenantID, string(requestJSON), method, endpoint, paymentID, transactionID, userAgent, clientIP).Scan(&logID)
	if err != nil {
		return 0, fmt.Errorf("failed to log request to %s table: %w", tableName, err)
	}

	// Store the mapping for efficient updates later
	l.mapMutex.Lock()
	l.logProviderMap[logID] = tableName
	l.mapMutex.Unlock()

	logger.Info("Request logged successfully", logger.LogContext{
		Provider: tableName,
		Fields: map[string]any{
			"log_id":   logID,
			"method":   method,
			"endpoint": endpoint,
		},
	})

	return logID, nil
}

// LogResponse logs the payment response and updates the existing record
func (l *DBPaymentLogger) LogResponse(ctx context.Context, logID int64, response any, processingMs int64) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Extract status and other fields from response if it's a PaymentResponse
	var status, errorCode string
	var amount float64
	var currency string

	if resp, ok := response.(*PaymentResponse); ok {
		status = string(resp.Status)
		errorCode = resp.ErrorCode
		amount = resp.Amount
		currency = resp.Currency
	}

	// Get the provider table name from our mapping
	l.mapMutex.RLock()
	tableName, exists := l.logProviderMap[logID]
	l.mapMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no provider table mapping found for log ID: %d", logID)
	}

	// Update the specific table directly instead of trying all tables
	query := fmt.Sprintf(`
		UPDATE %s 
		SET response = $1, response_at = NOW(), status = $2, error_code = $3, amount = $4, currency = $5, processing_ms = $6
		WHERE id = $7
	`, tableName)

	result, err := l.db.ExecContext(ctx, query, string(responseJSON), status, errorCode, amount, currency, processingMs, logID)
	if err != nil {
		return fmt.Errorf("failed to update response in %s table for log ID %d: %w", tableName, logID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for log ID %d: %w", logID, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows updated for log ID: %d in table: %s", logID, tableName)
	}

	logger.Info("Response logged successfully", logger.LogContext{
		Provider: tableName,
		Fields: map[string]any{
			"log_id":        logID,
			"processing_ms": processingMs,
			"status":        status,
		},
	})

	// Clean up the mapping to prevent memory leaks
	l.mapMutex.Lock()
	delete(l.logProviderMap, logID)
	l.mapMutex.Unlock()

	return nil
}

// LogError logs error information for failed requests
func (l *DBPaymentLogger) LogError(ctx context.Context, logID int64, errorCode, errorMsg string, processingMs int64) error {
	errorResponse := map[string]any{
		"error":   true,
		"code":    errorCode,
		"message": errorMsg,
		"time":    time.Now(),
	}

	return l.LogResponse(ctx, logID, errorResponse, processingMs)
}

// getActualProviderName extracts the actual provider name from providers table
func (l *DBPaymentLogger) getActualProviderName(providerName string) (string, error) {
	query := `
		SELECT name FROM providers WHERE active = true AND name = $1
	`

	stmt, err := l.db.Prepare(query)
	if err != nil {
		return providerName, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(providerName)
	if err != nil {
		return providerName, err
	}

	defer rows.Close()

	if !rows.Next() {
		return providerName, errors.New("provider not found")
	}

	return providerName, nil
}

func GetProvider(tenantID int, name, environment string) (PaymentProvider, error) {
	query := `
		SELECT tc.tenant_id, p.name as provider_name, tc.environment, tc.key, tc.value 
		FROM tenant_configs tc
		JOIN providers p ON tc.provider_id = p.id
		WHERE p.active = true AND p.name = $1 AND tc.environment = $2 AND tc.tenant_id = $3
		ORDER BY tc.tenant_id, p.name, tc.key
	`

	rows, err := config.App().DB.Query(query, name, environment, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant configs: %w", err)
	}
	defer rows.Close()

	configs := make(map[string]string)

	for rows.Next() {
		var tenantID int
		var providerName, environment, key, value string
		if err := rows.Scan(&tenantID, &providerName, &environment, &key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		configs[key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	providerFactory, err := Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	provider := providerFactory()
	if err := provider.Initialize(configs); err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	return provider, nil
}

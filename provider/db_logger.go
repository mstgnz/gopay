package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// PaymentLogger handles database logging for payment operations
type PaymentLogger interface {
	LogRequest(ctx context.Context, tenantID int, providerName string, method, endpoint string, request any, userAgent, clientIP string) (int64, error)
	LogResponse(ctx context.Context, logID int64, response any, processingMs int64) error
	LogError(ctx context.Context, logID int64, errorCode, errorMsg string, processingMs int64) error
}

// DBPaymentLogger implements PaymentLogger interface using SQL database
type DBPaymentLogger struct {
	db *sql.DB
}

// NewDBPaymentLogger creates a new database payment logger
func NewDBPaymentLogger(db *sql.DB) PaymentLogger {
	return &DBPaymentLogger{db: db}
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

	tableName := providerName
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

	log.Printf("Request logged to %s table with ID: %d", tableName, logID)
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

	// We need to determine which table this log belongs to
	// For now, we'll update all possible provider tables where this logID exists
	providerTables := []string{"iyzico", "paycell", "papara", "paytr", "payu", "nkolay", "ozanpay", "stripe", "shopier"}

	updated := false
	for _, tableName := range providerTables {
		query := fmt.Sprintf(`
			UPDATE %s 
			SET response = $1, response_at = NOW(), status = $2, error_code = $3, amount = $4, currency = $5, processing_ms = $6
			WHERE id = $7
		`, tableName)

		result, err := l.db.ExecContext(ctx, query, string(responseJSON), status, errorCode, amount, currency, processingMs, logID)
		if err != nil {
			continue // Try next table
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			continue
		}

		if rowsAffected > 0 {
			log.Printf("Response logged to %s table for ID: %d", tableName, logID)
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("failed to update response for log ID: %d", logID)
	}

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

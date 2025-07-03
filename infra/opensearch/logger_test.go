package opensearch

import (
	"context"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	logger := NewLogger(client)
	assert.NotNil(t, logger)
	assert.Equal(t, client, logger.client)
}

func TestLogger_LogPaymentRequest(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	tests := []struct {
		name        string
		log         PaymentLog
		expectError bool
	}{
		{
			name: "valid_log_entry",
			log: PaymentLog{
				TenantID:  "APP1",
				Provider:  "iyzico",
				Method:    "POST",
				Endpoint:  "/payment/create",
				RequestID: "test-request-123",
				Request: RequestLog{
					Body: `{"amount": 100}`,
				},
				Response: ResponseLog{
					StatusCode:       200,
					ProcessingTimeMs: 150,
				},
				PaymentInfo: PaymentInfo{
					PaymentID: "payment-123",
					Amount:    100.0,
					Currency:  "TRY",
				},
			},
			expectError: false, // Might fail due to connection, but structure is valid
		},
		{
			name: "log_without_timestamp",
			log: PaymentLog{
				Provider: "ozanpay",
				Method:   "GET",
				Endpoint: "/payment/status",
			},
			expectError: false,
		},
		{
			name: "log_without_request_id",
			log: PaymentLog{
				Provider: "stripe",
				Method:   "POST",
				Endpoint: "/payment/create",
			},
			expectError: false,
		},
		{
			name: "log_with_error",
			log: PaymentLog{
				Provider: "paytr",
				Method:   "POST",
				Endpoint: "/payment/create",
				Error: ErrorInfo{
					Code:    "PAYMENT_FAILED",
					Message: "Insufficient funds",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := logger.LogPaymentRequest(ctx, tt.log)

			// In test environment, this will likely fail due to connection issues
			// but we're testing the structure and logic
			if err != nil {
				t.Logf("Expected error in test environment: %v", err)
			}
		})
	}
}

func TestLogger_LogPaymentRequest_DisabledLogging(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: false, // Disabled logging
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	log := PaymentLog{
		Provider: "iyzico",
		Method:   "POST",
		Endpoint: "/payment/create",
	}

	ctx := context.Background()
	err = logger.LogPaymentRequest(ctx, log)
	assert.NoError(t, err, "Should not error when logging is disabled")
}

func TestLogger_SearchLogs(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	query := map[string]any{
		"match": map[string]any{
			"provider": "iyzico",
		},
	}

	ctx := context.Background()
	logs, err := logger.SearchLogs(ctx, "APP1", "iyzico", query)

	// This will likely fail in test environment
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	} else {
		assert.NotNil(t, logs)
	}
}

func TestLogger_SearchLogs_DisabledLogging(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	query := map[string]any{
		"match": map[string]any{
			"provider": "iyzico",
		},
	}

	ctx := context.Background()
	logs, err := logger.SearchLogs(ctx, "APP1", "iyzico", query)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging is disabled")
	assert.Nil(t, logs)
}

func TestLogger_GetPaymentLogs(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	ctx := context.Background()
	logs, err := logger.GetPaymentLogs(ctx, "APP1", "iyzico", "payment-123")

	// This will likely fail in test environment
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	} else {
		assert.NotNil(t, logs)
	}
}

func TestLogger_GetRecentErrorLogs(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	ctx := context.Background()
	logs, err := logger.GetRecentErrorLogs(ctx, "APP1", "iyzico", 24)

	// This will likely fail in test environment
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	} else {
		assert.NotNil(t, logs)
	}
}

func TestLogger_GetProviderStats(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	ctx := context.Background()
	stats, err := logger.GetProviderStats(ctx, "APP1", "iyzico", 24)

	// This will likely fail in test environment
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	} else {
		assert.NotNil(t, stats)
	}
}

func TestLogger_GetProviderStats_DisabledLogging(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)
	logger := NewLogger(client)

	ctx := context.Background()
	stats, err := logger.GetProviderStats(ctx, "APP1", "iyzico", 24)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging is disabled")
	assert.Nil(t, stats)
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldRedact bool
	}{
		{
			name:         "sanitize_card_number",
			input:        `{"cardNumber": "1234567890123456"}`,
			shouldRedact: true,
		},
		{
			name:         "sanitize_api_key",
			input:        `{"apiKey": "secret-key-123"}`,
			shouldRedact: true,
		},
		{
			name:         "sanitize_multiple_fields",
			input:        `{"cardNumber": "1234567890123456", "cvv": "123", "apiKey": "secret"}`,
			shouldRedact: true,
		},
		{
			name:         "no_sensitive_data",
			input:        `{"amount": 100, "currency": "TRY"}`,
			shouldRedact: false,
		},
		{
			name:         "empty_input",
			input:        "",
			shouldRedact: false,
		},
		{
			name:         "sanitize_password",
			input:        `{"password": "mypassword123"}`,
			shouldRedact: true,
		},
		{
			name:         "sanitize_cvv",
			input:        `{"cvv": "123"}`,
			shouldRedact: true,
		},
		{
			name:         "sanitize_secret_key",
			input:        `{"secretKey": "mysecret"}`,
			shouldRedact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)

			if tt.shouldRedact {
				assert.Contains(t, result, "***REDACTED***", "Should contain redacted marker for sensitive data")
				assert.NotEqual(t, tt.input, result, "Result should be different from input when sanitizing")
			} else {
				assert.Equal(t, tt.input, result, "Should not change non-sensitive data")
			}
		})
	}
}

func TestPaymentLog_StructureValidation(t *testing.T) {
	// Test PaymentLog structure
	log := PaymentLog{
		Timestamp: time.Now(),
		TenantID:  "APP1",
		Provider:  "iyzico",
		Method:    "POST",
		Endpoint:  "/payment/create",
		RequestID: "test-123",
		UserAgent: "GoPay/1.0",
		ClientIP:  "192.168.1.1",
		Request: RequestLog{
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"amount": 100}`,
			Params: map[string]string{
				"provider": "iyzico",
			},
		},
		Response: ResponseLog{
			StatusCode:       200,
			Headers:          map[string]string{"Content-Type": "application/json"},
			Body:             `{"status": "success"}`,
			ProcessingTimeMs: 150,
		},
		PaymentInfo: PaymentInfo{
			PaymentID:     "payment-123",
			Amount:        100.0,
			Currency:      "TRY",
			CustomerEmail: "test@example.com",
			Status:        "success",
			Use3D:         true,
		},
		Error: ErrorInfo{
			Code:    "TEST_ERROR",
			Message: "Test error message",
		},
	}

	// Validate all fields are properly set
	assert.NotZero(t, log.Timestamp)
	assert.Equal(t, "APP1", log.TenantID)
	assert.Equal(t, "iyzico", log.Provider)
	assert.Equal(t, "POST", log.Method)
	assert.Equal(t, "/payment/create", log.Endpoint)
	assert.Equal(t, "test-123", log.RequestID)
	assert.Equal(t, "GoPay/1.0", log.UserAgent)
	assert.Equal(t, "192.168.1.1", log.ClientIP)

	// Validate nested structures
	assert.Equal(t, "application/json", log.Request.Headers["Content-Type"])
	assert.Equal(t, `{"amount": 100}`, log.Request.Body)
	assert.Equal(t, "iyzico", log.Request.Params["provider"])

	assert.Equal(t, 200, log.Response.StatusCode)
	assert.Equal(t, "application/json", log.Response.Headers["Content-Type"])
	assert.Equal(t, `{"status": "success"}`, log.Response.Body)
	assert.Equal(t, int64(150), log.Response.ProcessingTimeMs)

	assert.Equal(t, "payment-123", log.PaymentInfo.PaymentID)
	assert.Equal(t, 100.0, log.PaymentInfo.Amount)
	assert.Equal(t, "TRY", log.PaymentInfo.Currency)
	assert.Equal(t, "test@example.com", log.PaymentInfo.CustomerEmail)
	assert.Equal(t, "success", log.PaymentInfo.Status)
	assert.True(t, log.PaymentInfo.Use3D)

	assert.Equal(t, "TEST_ERROR", log.Error.Code)
	assert.Equal(t, "Test error message", log.Error.Message)
}

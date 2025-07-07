package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/postgres"
)

func main() {
	fmt.Println("🔧 GoPay System Logging Example")
	fmt.Println("===============================")

	// Initialize PostgreSQL logger (optional)
	var postgresLogger *postgres.Logger
	// postgresLogger = postgres.NewLogger(db) // If PostgreSQL is available

	// Initialize global logger
	logger.InitGlobalLogger(postgresLogger)

	// Basic logging examples
	basicLoggingExamples()

	// Context logging examples
	contextLoggingExamples()

	// Tenant-specific logging examples
	tenantLoggingExamples()

	// Provider-specific logging examples
	providerLoggingExamples()

	// Error logging examples
	errorLoggingExamples()

	fmt.Println("\n✅ System logging examples completed!")
}

func basicLoggingExamples() {
	fmt.Println("\n1. Basic Logging Examples")
	fmt.Println("-------------------------")

	// Debug logging (only shown in development)
	logger.Debug("This is a debug message")

	// Info logging
	logger.Info("Application started successfully")

	// Warning logging
	logger.Warn("This is a warning message")

	// Error logging
	err := errors.New("sample error")
	logger.Error("An error occurred", err)

	// Info with additional fields
	logger.Info("User action completed", logger.LogContext{
		Fields: map[string]any{
			"user_id": "12345",
			"action":  "payment_created",
			"amount":  100.50,
		},
	})
}

func contextLoggingExamples() {
	fmt.Println("\n2. Context Logging Examples")
	fmt.Println("---------------------------")

	// Create a context logger
	ctx := logger.LogContext{
		RequestID: "req-123456",
		Fields: map[string]any{
			"session_id": "sess-abcdef",
			"user_id":    "user-789",
		},
	}

	contextLogger := logger.WithContext(ctx)

	contextLogger.Info("Processing user request")
	contextLogger.Debug("Validating input parameters")
	contextLogger.Warn("Rate limit approaching")

	// Add more fields to context
	contextLogger.AddField("processing_time_ms", 150)
	contextLogger.AddField("cache_hit", true)
	contextLogger.Info("Request processing completed")
}

func tenantLoggingExamples() {
	fmt.Println("\n3. Tenant-Specific Logging Examples")
	fmt.Println("------------------------------------")

	// Create tenant-specific logger
	tenantLogger := logger.WithTenant("COMPANY_ABC")

	tenantLogger.Info("Tenant configuration loaded")
	tenantLogger.Debug("Validating tenant permissions")

	// Add more context
	tenantLogger.AddField("config_version", "1.2.3")
	tenantLogger.AddField("feature_flags", []string{"3d_secure", "refunds"})
	tenantLogger.Info("Tenant setup completed")

	// Error with tenant context
	tenantLogger.Error("Failed to load tenant configuration", errors.New("database connection failed"))
}

func providerLoggingExamples() {
	fmt.Println("\n4. Provider-Specific Logging Examples")
	fmt.Println("--------------------------------------")

	// Create provider-specific logger
	providerLogger := logger.WithProvider("iyzico")

	providerLogger.Info("Initializing payment provider")
	providerLogger.Debug("Loading provider configuration")

	// Combined tenant and provider logging
	tenantProviderLogger := logger.WithTenantAndProvider("COMPANY_XYZ", "stripe")

	tenantProviderLogger.Info("Processing payment request")
	tenantProviderLogger.AddField("payment_id", "pay_123456")
	tenantProviderLogger.AddField("amount", 250.75)
	tenantProviderLogger.AddField("currency", "USD")
	tenantProviderLogger.Info("Payment request sent to provider")

	// Simulate payment processing
	time.Sleep(100 * time.Millisecond)

	tenantProviderLogger.AddField("provider_response_time_ms", 98)
	tenantProviderLogger.AddField("provider_transaction_id", "txn_stripe_789")
	tenantProviderLogger.Info("Payment processed successfully")
}

func errorLoggingExamples() {
	fmt.Println("\n5. Error Logging Examples")
	fmt.Println("-------------------------")

	// Simple error
	logger.Error("Database connection failed", errors.New("connection timeout"))

	// Error with context
	logger.Error("Payment processing failed", errors.New("invalid card number"), logger.LogContext{
		TenantID: "TENANT_123",
		Provider: "paycell",
		Fields: map[string]any{
			"payment_id":     "pay_failed_456",
			"customer_email": "customer@example.com",
			"amount":         150.00,
			"currency":       "TRY",
			"error_code":     "INVALID_CARD",
		},
	})

	// Provider-specific error
	providerLogger := logger.WithTenantAndProvider("ECOMMERCE_SITE", "ozanpay")
	providerLogger.Error("3D Secure authentication failed", errors.New("user cancelled authentication"))

	// System-level error
	logger.Error("PostgreSQL connection lost", errors.New("connection refused"), logger.LogContext{
		Fields: map[string]any{
			"postgres_url": "postgresql://localhost:5432/gopay",
			"retry_count":  3,
			"last_attempt": time.Now().Format(time.RFC3339),
		},
	})
}

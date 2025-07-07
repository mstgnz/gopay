// Package provider implements a unified payment processing interface that abstracts
// multiple payment gateways behind a single, consistent API.
//
// This package provides the core abstraction layer for payment processing in GoPay,
// allowing applications to work with different payment providers without worrying
// about their specific implementation details.
//
// # Core Concepts
//
// The provider package is built around several key interfaces and types:
//
//   - PaymentProvider: The main interface that all payment providers must implement
//   - PaymentService: Manages multiple providers and handles routing
//   - PaymentRequest/PaymentResponse: Standard request/response structures
//   - ProviderRegistry: Manages provider registration and discovery
//
// # Basic Usage
//
// Creating a payment service and processing payments:
//
//	// Create a new payment service
//	service := provider.NewPaymentService()
//
//	// Configure a provider
//	config := map[string]string{
//	    "apiKey":      "your-api-key",
//	    "secretKey":   "your-secret-key",
//	    "environment": "sandbox",
//	}
//
//	// Add the provider
//	err := service.AddProvider("iyzico", config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a payment request
//	request := PaymentRequest{
//	    Amount:      100.50,
//	    Currency:    "TRY",
//	    CallbackURL: "https://myapp.com/callback",
//	    Customer: Customer{
//	        Name:    "John",
//	        Surname: "Doe",
//	        Email:   "john@example.com",
//	    },
//	    CardInfo: CardInfo{
//	        CardHolderName: "John Doe",
//	        CardNumber:     "5528790000000008",
//	        ExpireMonth:    "12",
//	        ExpireYear:     "2030",
//	        CVV:            "123",
//	    },
//	    Use3D: true,
//	}
//
//	// Process the payment
//	response, err := service.CreatePayment(ctx, "iyzico", request)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Handle the response
//	if response.Success {
//	    if response.RedirectURL != "" {
//	        // Redirect user to 3D secure page
//	        fmt.Printf("Redirect to: %s\n", response.RedirectURL)
//	    } else {
//	        // Payment completed successfully
//	        fmt.Printf("Payment completed: %s\n", response.PaymentID)
//	    }
//	}
//
// # Multi-Tenant Usage
//
// The provider package supports multi-tenant architectures where different
// applications or customers can use different provider configurations:
//
//	// Add tenant-specific provider configurations
//	service.AddProvider("TENANT1_iyzico", tenant1Config)
//	service.AddProvider("TENANT2_stripe", tenant2Config)
//
//	// Use with tenant-specific provider names
//	response, err := service.CreatePayment(ctx, "TENANT1_iyzico", request)
//
// # Environment Support
//
// All providers support both sandbox (test) and production environments:
//
//	sandboxConfig := map[string]string{
//	    "apiKey":      "sandbox-api-key",
//	    "secretKey":   "sandbox-secret-key",
//	    "environment": "sandbox",
//	}
//
//	prodConfig := map[string]string{
//	    "apiKey":      "production-api-key",
//	    "secretKey":   "production-secret-key",
//	    "environment": "production",
//	}
//
// # Supported Operations
//
// The PaymentProvider interface supports the following operations:
//
//   - CreatePayment: Process a direct payment
//   - Create3DPayment: Initiate a 3D Secure payment flow
//   - Complete3DPayment: Complete a 3D Secure payment after authentication
//   - GetPaymentStatus: Check the status of a payment
//   - CancelPayment: Cancel a pending or authorized payment
//   - RefundPayment: Issue a full or partial refund
//   - ValidateWebhook: Validate incoming webhook notifications
//
// # Error Handling
//
// The package provides structured error handling with specific error types
// and codes that can be used to determine the appropriate response:
//
//	response, err := service.CreatePayment(ctx, "iyzico", request)
//	if err != nil {
//	    // Handle connection or configuration errors
//	    log.Printf("Payment error: %v", err)
//	    return
//	}
//
//	if !response.Success {
//	    // Handle payment-specific failures
//	    switch response.ErrorCode {
//	    case "INSUFFICIENT_FUNDS":
//	        // Handle insufficient funds
//	    case "INVALID_CARD":
//	        // Handle invalid card
//	    default:
//	        // Handle other payment failures
//	    }
//	}
//
// # Provider Registration
//
// New payment providers can be registered using the registration system:
//
//	import _ "github.com/mstgnz/gopay/provider/iyzico" // Auto-registers iyzico
//
// Or manually:
//
//	provider.Register("myprovider", func() provider.PaymentProvider {
//	    return &MyCustomProvider{}
//	})
//
// # Webhook Handling
//
// Providers support webhook validation for asynchronous payment notifications:
//
//	isValid, data, err := provider.ValidateWebhook(ctx, webhookData, headers)
//	if err != nil {
//	    log.Printf("Webhook validation error: %v", err)
//	    return
//	}
//
//	if isValid {
//	    // Process the webhook data
//	    paymentID := data["paymentId"]
//	    status := data["status"]
//	    // Update payment status in your system
//	}
//
// # Supported Providers
//
// Currently supported payment providers include:
//
//   - İyzico: Turkish payment gateway with comprehensive features
//   - Stripe: International payment processing platform
//   - OzanPay: Turkish digital payment solutions
//   - Paycell: Turkcell's payment gateway
//   - Papara: Digital wallet and payment services
//   - Nkolay: Enterprise payment processing
//   - PayTR: Popular Turkish payment gateway
//   - PayU: International payment processing
//
// Each provider is implemented in its own subpackage and can be imported
// individually as needed.
//
// # Security Features
//
// The provider package implements comprehensive security measures:
//
//   - SQL injection protection with input validation
//   - Secure data handling with sensitive information masking
//   - Provider-specific authentication and encryption
//   - Request signing and verification
//   - Webhook signature validation
//   - Connection security with HTTPS enforcement
//
// # Database Integration
//
// All payment operations are logged to PostgreSQL for:
//
//   - Comprehensive audit trails
//   - Payment tracking and reconciliation
//   - Performance monitoring and analytics
//   - Error analysis and debugging
//   - Compliance and reporting
//
// # Thread Safety
//
// The PaymentService and all provider implementations are designed to be
// thread-safe and can be used concurrently from multiple goroutines.
//
// # Performance Considerations
//
// - Connection pooling is handled automatically by the HTTP clients
// - Provider instances are reused across requests
// - Timeouts are configurable per provider
// - Logging and metrics are built-in for monitoring
// - Efficient request/response processing
// - Memory-optimized data structures
//
// # Testing Support
//
// All providers include comprehensive test suites:
//
//   - Unit tests for all core functionality
//   - Integration tests with real provider APIs
//   - Test data and mock responses
//   - Performance benchmarks
//   - Security vulnerability tests
//
// # Production Considerations
//
// When deploying to production:
//
//   - Use production provider credentials
//   - Enable comprehensive logging and monitoring
//   - Configure appropriate timeouts and retry policies
//   - Implement proper error handling and alerting
//   - Set up health checks and metrics
//   - Configure rate limiting and security measures
//
// For more specific information about individual providers, see their
// respective package documentation in the subpackages.
package provider

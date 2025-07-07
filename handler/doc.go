// Package handler provides HTTP request handlers for the GoPay payment gateway.
//
// All persistent data, configuration, and audit logs are stored in **PostgreSQL**.
//
// This package contains all the HTTP handlers that implement the REST API
// endpoints for payment processing, configuration management, analytics,
// and logging. The handlers bridge the HTTP layer with the underlying
// payment provider services.
//
// # Core Handlers
//
// The package includes several specialized handlers:
//
//   - PaymentHandler: Handles payment operations (create, status, cancel, refund)
//   - ConfigHandler: Manages tenant configurations and provider settings
//   - LogsHandler: Provides access to payment logs and audit trails
//   - AnalyticsHandler: Serves analytics data and dashboard statistics
//
// # Payment Handler
//
// The PaymentHandler manages all payment-related HTTP requests:
//
//	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
//
//	// Routes
//	r.Post("/v1/payments/{provider}", paymentHandler.ProcessPayment)
//	r.Get("/v1/payments/{provider}/{paymentID}", paymentHandler.GetPaymentStatus)
//	r.Delete("/v1/payments/{provider}/{paymentID}", paymentHandler.CancelPayment)
//	r.Post("/v1/payments/{provider}/refund", paymentHandler.RefundPayment)
//
// # Multi-Tenant Support
//
// All handlers support multi-tenant operations via the X-Tenant-ID header:
//
//	POST /v1/payments/iyzico
//	Headers:
//	  X-Tenant-ID: APP1
//	  Authorization: Bearer your-api-key
//	  Content-Type: application/json
//
//	Body:
//	{
//	  "amount": 100.50,
//	  "currency": "TRY",
//	  "callbackUrl": "https://myapp.com/callback",
//	  "customer": {
//	    "name": "John",
//	    "surname": "Doe",
//	    "email": "john@example.com"
//	  }
//	}
//
// # Configuration Handler
//
// The ConfigHandler manages tenant-specific provider configurations:
//
//	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)
//
//	// Set tenant configuration
//	r.Post("/v1/set-env", configHandler.SetEnv)
//
//	// Get tenant configuration
//	r.Get("/v1/config/tenant-config", configHandler.GetTenantConfig)
//
//	// Delete tenant configuration
//	r.Delete("/v1/config/tenant-config", configHandler.DeleteTenantConfig)
//
// Example configuration request:
//
//	POST /v1/set-env
//	Headers:
//	  X-Tenant-ID: APP1
//	  Content-Type: application/json
//
//	Body:
//	{
//	  "IYZICO_API_KEY": "sandbox-api-key",
//	  "IYZICO_SECRET_KEY": "sandbox-secret-key",
//	  "IYZICO_ENVIRONMENT": "sandbox"
//	}
//
// # Callback and Webhook Handling
//
// The PaymentHandler also manages provider callbacks and webhooks:
//
//	// 3D Secure callbacks
//	r.HandleFunc("/callback/{provider}", paymentHandler.HandleCallback)
//
//	// Payment webhooks
//	r.Post("/webhooks/{provider}", paymentHandler.HandleWebhook)
//
// These endpoints automatically preserve tenant information and route
// responses back to the correct application.
//
// # Analytics Handler
//
// The AnalyticsHandler provides business intelligence endpoints:
//
//	analyticsHandler := handler.NewAnalyticsHandler(logger)
//
//	// Dashboard statistics
//	r.Get("/v1/analytics/dashboard", analyticsHandler.GetDashboardStats)
//
//	// Provider performance stats
//	r.Get("/v1/analytics/providers", analyticsHandler.GetProviderStats)
//
//	// Recent payment activity
//	r.Get("/v1/analytics/activity", analyticsHandler.GetRecentActivity)
//
// # Logs Handler
//
// The LogsHandler provides access to payment logs and audit trails:
//
//	logsHandler := handler.NewLogsHandler(logger)
//
//	// Get payment logs for a provider
//	r.Get("/v1/logs/{provider}", logsHandler.GetLogs)
//
//	// Search logs
//	r.Get("/v1/logs/search", logsHandler.SearchLogs)
//
// # Request Validation
//
// All handlers use structured validation for incoming requests:
//
//	type PaymentRequest struct {
//	    Amount      float64  `json:"amount" validate:"required,gt=0"`
//	    Currency    string   `json:"currency" validate:"required,len=3"`
//	    CallbackURL string   `json:"callbackUrl" validate:"required,url"`
//	    Customer    Customer `json:"customer" validate:"required"`
//	    CardInfo    CardInfo `json:"cardInfo" validate:"required"`
//	}
//
// Validation errors are returned with detailed messages:
//
//	{
//	  "success": false,
//	  "message": "Validation error",
//	  "error": {
//	    "amount": "must be greater than 0",
//	    "currency": "must be exactly 3 characters"
//	  }
//	}
//
// # Error Handling
//
// All handlers implement consistent error handling with structured responses:
//
//	// Success response
//	{
//	  "success": true,
//	  "message": "Payment processed",
//	  "data": {
//	    "paymentId": "12345",
//	    "status": "pending",
//	    "threeDUrl": "https://provider.com/3d-secure"
//	  }
//	}
//
//	// Error response
//	{
//	  "success": false,
//	  "message": "Payment failed",
//	  "error": {
//	    "code": "INSUFFICIENT_FUNDS",
//	    "message": "Insufficient funds on card"
//	  }
//	}
//
// # Authentication and Authorization
//
// API endpoints require Bearer token authentication:
//
//	Authorization: Bearer your-api-key
//
// Some endpoints (callbacks, webhooks, health checks) are public and
// don't require authentication.
//
// # Rate Limiting and Security
//
// Handlers are protected by several middleware layers:
//
//   - Rate limiting per IP and API key
//   - IP whitelisting for sensitive operations
//   - Request size validation
//   - Security headers (CORS, CSP, etc.)
//   - Request logging and monitoring
//
// # Content Type Support
//
// All handlers support JSON content type for requests and responses:
//
//	Content-Type: application/json
//	Accept: application/json
//
// # HTTP Status Codes
//
// Handlers use standard HTTP status codes:
//
//   - 200 OK: Successful operation
//   - 201 Created: Resource created successfully
//   - 400 Bad Request: Invalid request format or validation error
//   - 401 Unauthorized: Missing or invalid authentication
//   - 404 Not Found: Resource not found
//   - 429 Too Many Requests: Rate limit exceeded
//   - 500 Internal Server Error: Server-side error
//
// # Logging and Monitoring
//
// All handlers automatically log requests and responses for monitoring:
//
//   - Request/response timing
//   - HTTP status codes
//   - Error rates
//   - Payment success rates
//   - Provider performance metrics
//
// Logs are structured and can be sent to OpenSearch for analysis.
//
// # Testing
//
// All handlers include comprehensive tests covering:
//
//   - Valid request scenarios
//   - Invalid request validation
//   - Authentication failures
//   - Provider errors
//   - Multi-tenant scenarios
//   - Edge cases and error conditions
//
// Example test:
//
//	func TestPaymentHandler_ProcessPayment(t *testing.T) {
//	    handler := NewPaymentHandler(mockService, validator)
//
//	    req := httptest.NewRequest("POST", "/payments/iyzico", requestBody)
//	    req.Header.Set("X-Tenant-ID", "TEST")
//	    req.Header.Set("Content-Type", "application/json")
//
//	    w := httptest.NewRecorder()
//	    handler.ProcessPayment(w, req)
//
//	    assert.Equal(t, 200, w.Code)
//	}
//
// # Performance Considerations
//
// Handlers are optimized for high throughput:
//
//   - Streaming JSON parsing for large requests
//   - Connection pooling for provider API calls
//   - Efficient logging with batching
//   - Minimal memory allocations
//   - Concurrent request processing
//
// For production deployments, consider using multiple handler instances
// behind a load balancer for optimal performance.
package handler

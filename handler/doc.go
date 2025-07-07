// Package handler provides HTTP request handlers for the GoPay payment gateway.
//
// All persistent data, configuration, and audit logs are stored in **PostgreSQL**.
//
// This package contains all the HTTP handlers that implement the REST API
// endpoints for payment processing, configuration management, analytics,
// logging, and authentication. The handlers bridge the HTTP layer with the underlying
// payment provider services and implement JWT-based authentication.
//
// # Core Handlers
//
// The package includes several specialized handlers:
//
//   - AuthHandler: Manages JWT authentication (login, register, token management)
//   - PaymentHandler: Handles payment operations (create, status, cancel, refund)
//   - ConfigHandler: Manages tenant configurations and provider settings
//   - LogsHandler: Provides access to payment logs and audit trails
//   - AnalyticsHandler: Serves analytics data and dashboard statistics
//   - TenantRateLimitHandler: Provides rate limiting statistics for tenants
//
// # Authentication System
//
// GoPay uses JWT (JSON Web Token) based authentication with auto-rotating secret keys:
//
//	authHandler := handler.NewAuthHandler(tenantService, jwtService, validator)
//
//	// Authentication routes
//	r.Post("/v1/auth/login", authHandler.Login)
//	r.Post("/v1/auth/register", authHandler.Register)        // First user only
//	r.Post("/v1/auth/create-tenant", authHandler.CreateTenant) // Admin only
//	r.Post("/v1/auth/refresh", authHandler.RefreshToken)
//	r.Post("/v1/auth/validate", authHandler.ValidateToken)
//	r.Get("/v1/auth/profile", authHandler.GetProfile)
//	r.Post("/v1/auth/change-password", authHandler.ChangePassword)
//
// Example login request:
//
//	POST /v1/auth/login
//	Content-Type: application/json
//
//	{
//	  "username": "admin",
//	  "password": "password123"
//	}
//
//	Response:
//	{
//	  "success": true,
//	  "data": {
//	    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
//	    "expires_at": "2024-01-16T10:30:00Z",
//	    "username": "admin",
//	    "tenant_id": "1"
//	  }
//	}
//
// # JWT Security Model
//
// Enhanced security features:
//
//   - Auto-rotating JWT secret keys (regenerate on each service restart)
//   - All tokens become invalid after service restart
//   - Users must re-authenticate after restart
//   - No persistent secret storage (UUID-generated keys)
//   - 24-hour token expiry with refresh capability
//
// # Payment Handler
//
// The PaymentHandler manages all payment-related HTTP requests with JWT authentication:
//
//	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
//
//	// Routes (all require JWT authentication)
//	r.Post("/v1/payments/{provider}", paymentHandler.ProcessPayment)
//	r.Get("/v1/payments/{provider}/{paymentID}", paymentHandler.GetPaymentStatus)
//	r.Delete("/v1/payments/{provider}/{paymentID}", paymentHandler.CancelPayment)
//	r.Post("/v1/payments/{provider}/refund", paymentHandler.RefundPayment)
//
// # Multi-Tenant Support
//
// All handlers support multi-tenant operations via JWT token authentication.
// Tenant information is automatically extracted from the JWT token:
//
//	POST /v1/payments/iyzico
//	Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
//	Content-Type: application/json
//
//	{
//	  "amount": 100.50,
//	  "currency": "TRY",
//	  "callbackUrl": "https://myapp.com/callback",
//	  "customer": {
//	    "name": "John",
//	    "surname": "Doe",
//	    "email": "john@example.com"
//	  },
//	  "cardInfo": {
//	    "cardHolderName": "John Doe",
//	    "cardNumber": "5528790000000008",
//	    "expireMonth": "12",
//	    "expireYear": "2030",
//	    "cvv": "123"
//	  },
//	  "use3D": true
//	}
//
// Note: No X-Tenant-ID header needed - tenant is extracted from JWT token automatically.
//
// # Configuration Handler
//
// The ConfigHandler manages tenant-specific provider configurations via JWT authentication:
//
//	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)
//
//	// Configuration routes (require JWT authentication)
//	r.Post("/v1/set-env", configHandler.SetEnv)
//	r.Get("/v1/config/tenant-config", configHandler.GetTenantConfig)
//	r.Delete("/v1/config/tenant-config", configHandler.DeleteTenantConfig)
//
// Example configuration request:
//
//	POST /v1/set-env
//	Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
//	Content-Type: application/json
//
//	{
//	  "IYZICO_API_KEY": "sandbox-api-key",
//	  "IYZICO_SECRET_KEY": "sandbox-secret-key",
//	  "IYZICO_ENVIRONMENT": "sandbox"
//	}
//
// Tenant information is automatically extracted from the JWT token.
//
// # Rate Limiting Handler
//
// The TenantRateLimitHandler provides rate limiting statistics for authenticated tenants:
//
//	rateLimitHandler := handler.NewTenantRateLimitHandler(rateLimiter)
//
//	// Rate limiting routes (require JWT authentication)
//	r.Get("/v1/rate-limit/stats", rateLimitHandler.GetTenantStats)
//
// Example response:
//
//	{
//	  "success": true,
//	  "data": {
//	    "tenant_id": "123",
//	    "global_limit": 100,
//	    "global_used": 15,
//	    "global_remaining": 85,
//	    "actions": {
//	      "payment": {
//	        "limit": 50,
//	        "used": 5,
//	        "remaining": 45,
//	        "reset_time": "2024-01-15T10:31:00Z"
//	      }
//	    }
//	  }
//	}
//
// # Tenant-Based Rate Limiting
//
// All handlers are protected by sophisticated rate limiting:
//
//   - Individual rate limits per tenant (extracted from JWT token)
//   - Action-specific limits (payment, refund, status, auth, config)
//   - Premium tenant support with higher limits
//   - IP-based rate limiting for unauthenticated requests
//   - Automatic cleanup and burst allowance
//
// Rate limiting configuration:
//
//   - Global requests: 100/minute per tenant (default)
//   - Payment requests: 50/minute per tenant (default)
//   - Refund requests: 20/minute per tenant (default)
//   - Status requests: 200/minute per tenant (default)
//   - Unauthenticated requests: 10/minute per IP
//   - Premium tenants: 2x rate multiplier
//
// # Callback and Webhook Handling
//
// The PaymentHandler also manages provider callbacks and webhooks with multi-tenant support:
//
//	// 3D Secure callbacks (no authentication required - called by payment providers)
//	r.HandleFunc("/v1/callback/{provider}", paymentHandler.HandleCallback)
//	r.HandleFunc("/callback/{provider}", paymentHandler.HandleCallback) // Legacy
//
//	// Payment webhooks (no authentication required - signature validated)
//	r.Post("/v1/webhooks/{provider}", paymentHandler.HandleWebhook)
//	r.Post("/webhooks/{provider}", paymentHandler.HandleWebhook) // Legacy
//
// These endpoints automatically preserve tenant information via query parameters:
//
//	/v1/callback/iyzico?tenantId=APP1&paymentId=123&originalCallbackUrl=...
//	/v1/webhooks/iyzico?tenantId=APP1
//
// # Analytics Handler
//
// The AnalyticsHandler provides business intelligence endpoints (no authentication required for dashboard display):
//
//	analyticsHandler := handler.NewAnalyticsHandler(logger)
//
//	// Public analytics routes
//	r.Get("/v1/analytics/dashboard", analyticsHandler.GetDashboardStats)
//	r.Get("/v1/analytics/providers", analyticsHandler.GetProviderStats)
//	r.Get("/v1/analytics/activity", analyticsHandler.GetRecentActivity)
//	r.Get("/v1/analytics/trends", analyticsHandler.GetPaymentTrends)
//
// # Logs Handler
//
// The LogsHandler provides access to payment logs and audit trails (requires JWT authentication):
//
//	logsHandler := handler.NewLogsHandler(logger)
//
//	// Authenticated log routes
//	r.Get("/v1/logs/{provider}", logsHandler.GetLogs)
//	r.Get("/v1/logs/{provider}/payment/{paymentID}", logsHandler.GetPaymentLogs)
//	r.Get("/v1/logs/{provider}/errors", logsHandler.GetErrorLogs)
//	r.Get("/v1/logs/{provider}/stats", logsHandler.GetLoggingStats)
//
// Tenant isolation is enforced - each tenant can only access their own logs.
//
// # Request Validation
//
// All handlers use structured validation for incoming requests:
//
//	type PaymentRequest struct {
//	    Amount       float64  `json:"amount" validate:"required,gt=0"`
//	    Currency     string   `json:"currency" validate:"required,len=3"`
//	    CallbackURL  string   `json:"callbackUrl" validate:"required,url"`
//	    Customer     Customer `json:"customer" validate:"required"`
//	    CardInfo     CardInfo `json:"cardInfo" validate:"required"`
//	    Use3D        bool     `json:"use3D"`
//	    Description  string   `json:"description"`
//	}
//
// Validation errors are returned with detailed messages:
//
//	{
//	  "success": false,
//	  "message": "Validation error",
//	  "error": {
//	    "amount": "must be greater than 0",
//	    "currency": "must be exactly 3 characters",
//	    "callbackUrl": "must be a valid URL"
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
//	  "message": "Payment processed successfully",
//	  "data": {
//	    "paymentId": "12345",
//	    "status": "pending",
//	    "redirectUrl": "https://provider.com/3d-secure"
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
// Most API endpoints require JWT Bearer token authentication:
//
//	Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
//
// Public endpoints (no authentication required):
//   - /health - Health check
//   - /v1/auth/login - User login
//   - /v1/auth/register - First user registration
//   - /v1/analytics/* - Dashboard analytics
//   - /v1/callback/* - 3D Secure callbacks
//   - /v1/webhooks/* - Payment webhooks
//
// Admin-only endpoints (require admin tenant):
//   - /v1/auth/create-tenant - Create new tenant
//   - /v1/auth/change-password (for other users)
//
// # JWT Middleware Integration
//
// JWT authentication is handled by middleware that extracts tenant information:
//
//	// JWT middleware validates token and extracts tenant info
//	func JWTAuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler
//
//	// Usage in handlers
//	func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
//	    // Tenant ID automatically available from JWT context
//	    tenantID := middle.GetTenantIDFromContext(r.Context())
//	    // Process payment with tenant-specific configuration
//	}
//
// # Security Features
//
// Handlers are protected by comprehensive security measures:
//
//   - JWT authentication with auto-rotating secret keys
//   - Tenant-based rate limiting with action-specific limits
//   - SQL injection protection with input validation
//   - Request size validation and limits
//   - Security headers (CORS, CSP, etc.)
//   - Sensitive data masking in logs and responses
//   - Webhook signature validation
//   - Comprehensive audit logging
//
// # Content Type Support
//
// All handlers support JSON content type for requests and responses:
//
//	Content-Type: application/json
//	Accept: application/json
//
// Form data is supported for webhook and callback endpoints:
//
//	Content-Type: application/x-www-form-urlencoded
//
// # HTTP Status Codes
//
// Handlers use standard HTTP status codes:
//
//   - 200 OK: Successful operation
//   - 201 Created: Resource created successfully
//   - 400 Bad Request: Invalid request format or validation error
//   - 401 Unauthorized: Missing or invalid JWT token
//   - 403 Forbidden: Insufficient permissions
//   - 404 Not Found: Resource not found
//   - 409 Conflict: Resource already exists
//   - 429 Too Many Requests: Rate limit exceeded
//   - 500 Internal Server Error: Unexpected server error
//
// # Database Integration
//
// All handlers integrate with PostgreSQL for:
//
//   - User authentication and tenant management
//   - Payment logging with tenant isolation
//   - Configuration storage and retrieval
//   - Analytics and business intelligence
//   - Audit trails and compliance
//
// # Performance Considerations
//
// Handlers are optimized for performance:
//
//   - Connection pooling for database operations
//   - Efficient JWT token validation
//   - Rate limiting to prevent abuse
//   - Structured logging for monitoring
//   - Error handling to prevent cascading failures
//
// For specific implementation details, see the individual handler files
// and their corresponding test files.
package handler

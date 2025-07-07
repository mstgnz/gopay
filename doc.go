// Package gopay provides a unified payment gateway that abstracts multiple payment providers
// behind a single, standardized API. It acts as a bridge between your applications and
// payment providers, handling callbacks, webhooks, and logging seamlessly.
//
// # Overview
//
// GoPay solves the problem of having to integrate with multiple payment providers,
// each with different APIs, authentication methods, callback mechanisms, and response formats.
// Instead, GoPay standardizes everything into one consistent interface.
//
// # Architecture
//
// The payment flow follows this pattern:
//
//	┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
//	│                 │    │                 │    │                 │
//	│   Your Apps     │◄──►│     GoPay       │◄──►│   Payment       │
//	│  (APP1, APP2)   │    │   (Gateway)     │    │   Providers     │
//	│                 │    │                 │    │                 │
//	└─────────────────┘    └─────────────────┘    └─────────────────┘
//
// # Supported Providers
//
// Currently supported payment providers include:
//   - İyzico: Full payment, 3D secure, refund, and cancellation support
//   - Stripe: Complete payment processing with international support
//   - OzanPay: Turkish market focused payment solutions
//   - Paycell: Comprehensive payment gateway for Turkey
//   - Papara: Modern digital wallet and payment solutions
//   - Nkolay: Enterprise payment processing
//   - PayTR: Popular Turkish payment gateway
//   - PayU: International payment processing
//
// # Authentication System
//
// GoPay uses JWT (JSON Web Token) based authentication with auto-rotating secret keys:
//
//	// 1. Register or login to get JWT token
//	POST /v1/auth/login
//	{
//	  "username": "admin",
//	  "password": "password123"
//	}
//
//	// Response includes JWT token
//	{
//	  "success": true,
//	  "data": {
//	    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
//	    "expires_at": "2024-01-16T10:30:00Z",
//	    "tenant_id": "1"
//	  }
//	}
//
//	// 2. Use JWT token in Authorization header
//	Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
//
// # Enhanced Security Features
//
// Auto-Rotating JWT Secret Keys:
//   - JWT secret key regenerates on every service restart
//   - All existing tokens become invalid after restart
//   - Users must re-authenticate after service restart
//   - No persistent secret storage (UUID-generated keys)
//
// Tenant-Based Rate Limiting:
//   - Individual rate limits per tenant extracted from JWT token
//   - Action-specific limits (payment, refund, status, auth, config)
//   - Premium tenant support with higher limits
//   - IP-based rate limiting for unauthenticated requests
//   - Automatic cleanup and burst allowance
//
// # Quick Start
//
// Basic usage example with JWT authentication:
//
//	package main
//
//	import (
//	    "context"
//	    "net/http"
//	    "github.com/mstgnz/gopay/provider"
//	    _ "github.com/mstgnz/gopay/provider/iyzico" // Import to register provider
//	)
//
//	func main() {
//	    // 1. Authenticate and get JWT token
//	    loginReq := map[string]string{
//	        "username": "admin",
//	        "password": "password123",
//	    }
//
//	    token := authenticateAndGetToken(loginReq)
//
//	    // 2. Configure provider with JWT authentication
//	    configReq := map[string]string{
//	        "IYZICO_API_KEY":    "your-api-key",
//	        "IYZICO_SECRET_KEY": "your-secret-key",
//	        "IYZICO_ENVIRONMENT": "sandbox",
//	    }
//
//	    err := configureProvider(token, configReq)
//	    if err != nil {
//	        panic(err)
//	    }
//
//	    // 3. Create payment request with JWT authentication
//	    paymentReq := map[string]any{
//	        "amount":   100.50,
//	        "currency": "TRY",
//	        "customer": map[string]string{
//	            "name":    "John",
//	            "surname": "Doe",
//	            "email":   "john@example.com",
//	        },
//	        "cardInfo": map[string]string{
//	            "cardHolderName": "John Doe",
//	            "cardNumber":     "5528790000000008",
//	            "expireMonth":    "12",
//	            "expireYear":     "2030",
//	            "cvv":            "123",
//	        },
//	        "use3D":       true,
//	        "callbackUrl": "https://yourapp.com/callback",
//	    }
//
//	    // 4. Process payment with JWT token (tenant automatically detected from token)
//	    response := processPaymentWithToken(token, "iyzico", paymentReq)
//
//	    // 5. Handle response
//	    if response.Success {
//	        if response.RedirectURL != "" {
//	            // Redirect user to 3D secure page
//	            fmt.Printf("Redirect to: %s\n", response.RedirectURL)
//	        }
//	    }
//	}
//
// # Multi-Tenant Support
//
// GoPay supports multi-tenant architecture where tenant information is automatically
// extracted from JWT tokens:
//
//	// Each JWT token contains tenant information
//	// No need for X-Tenant-ID headers - tenant is extracted from JWT
//
//	// 1. Configure tenant-specific provider settings
//	POST /v1/set-env
//	Authorization: Bearer <tenant_jwt_token>
//	{
//	  "IYZICO_API_KEY": "tenant-specific-api-key",
//	  "IYZICO_SECRET_KEY": "tenant-specific-secret-key",
//	  "IYZICO_ENVIRONMENT": "sandbox"
//	}
//
//	// 2. Process payments (tenant automatically detected from JWT)
//	POST /v1/payments/iyzico
//	Authorization: Bearer <tenant_jwt_token>
//	{
//	  "amount": 100.50,
//	  "currency": "TRY",
//	  "customer": {...}
//	}
//
// # Environment Support
//
// Each provider supports both test (sandbox) and production environments:
//
//	config := map[string]string{
//	    "IYZICO_API_KEY":     "your-api-key",
//	    "IYZICO_SECRET_KEY":  "your-secret-key",
//	    "IYZICO_ENVIRONMENT": "production", // or "sandbox"
//	}
//
// # HTTP API
//
// GoPay provides a comprehensive REST API with JWT authentication:
//
//	# Authentication
//	POST /v1/auth/login          - User login
//	POST /v1/auth/register       - First user registration (admin)
//	POST /v1/auth/create-tenant  - Create new tenant (admin only)
//	POST /v1/auth/refresh        - Refresh JWT token
//	POST /v1/auth/validate       - Validate JWT token
//
//	# Configuration
//	POST /v1/set-env                    - Configure payment provider
//	GET  /v1/config/tenant-config       - Get tenant configuration
//	DELETE /v1/config/tenant-config     - Delete tenant configuration
//
//	# Payments
//	POST /v1/payments/{provider}                 - Create payment
//	GET  /v1/payments/{provider}/{paymentID}     - Check payment status
//	DELETE /v1/payments/{provider}/{paymentID}   - Cancel payment
//	POST /v1/payments/{provider}/refund          - Process refund
//
//	# Rate Limiting
//	GET /v1/rate-limit/stats  - Get tenant rate limit statistics
//
//	# Analytics
//	GET /v1/analytics/dashboard  - Dashboard statistics
//	GET /v1/analytics/providers  - Provider performance stats
//	GET /v1/analytics/activity   - Recent payment activity
//
//	# Logs
//	GET /v1/logs/{provider}                      - Get payment logs
//	GET /v1/logs/{provider}/payment/{paymentID}  - Get specific payment logs
//	GET /v1/logs/{provider}/errors               - Get error logs
//
// All API endpoints require JWT authentication:
//
//	Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
//
// # Rate Limiting
//
// GoPay implements sophisticated tenant-based rate limiting:
//
//   - Global requests: 100/minute per tenant (default)
//   - Payment requests: 50/minute per tenant (default)
//   - Refund requests: 20/minute per tenant (default)
//   - Status requests: 200/minute per tenant (default)
//   - Unauthenticated requests: 10/minute per IP
//   - Premium tenants: 2x rate multiplier
//   - Burst allowance: Additional 10 requests above limits
//   - Automatic cleanup: Old entries cleaned every 5 minutes
//
// Rate limit configuration via environment variables:
//
//	TENANT_GLOBAL_RATE_LIMIT=100      # Global requests per minute
//	TENANT_PAYMENT_RATE_LIMIT=50      # Payment requests per minute
//	TENANT_REFUND_RATE_LIMIT=20       # Refund requests per minute
//	TENANT_STATUS_RATE_LIMIT=200      # Status requests per minute
//	UNAUTHENTICATED_RATE_LIMIT=10     # Unauthenticated requests per minute
//	PREMIUM_TENANTS=tenant1,tenant2   # Premium tenant list
//
// # Callbacks and Webhooks
//
// GoPay handles provider callbacks and webhooks automatically with multi-tenant support:
//
//   - Callback URLs: /v1/callback/{provider}?tenantId={tenantId}
//   - Webhook URLs: /v1/webhooks/{provider}?tenantId={tenantId}
//   - Legacy URLs: /callback/{provider}, /webhooks/{provider}
//
// The system preserves tenant information and routes responses back to
// the correct application.
//
// # Logging and Analytics
//
// GoPay integrates with PostgreSQL for comprehensive logging and analytics:
//
//   - Real-time payment tracking with tenant isolation
//   - Provider-specific performance metrics
//   - Comprehensive request/response logging
//   - SQL injection protection with input validation
//   - Dashboard analytics with business intelligence
//   - Audit trails for all operations
//
// # Configuration
//
// Configuration is done programmatically via JWT-authenticated API calls:
//
//	# Configure via API (recommended)
//	POST /v1/set-env
//	Authorization: Bearer <jwt_token>
//	{
//	  "IYZICO_API_KEY": "your-api-key",
//	  "IYZICO_SECRET_KEY": "your-secret-key",
//	  "IYZICO_ENVIRONMENT": "sandbox"
//	}
//
//	# Environment variables (legacy/global)
//	IYZICO_API_KEY=your-api-key
//	IYZICO_SECRET_KEY=your-secret-key
//	IYZICO_ENVIRONMENT=sandbox
//
// # Security Features
//
// GoPay includes comprehensive security features:
//
//   - JWT authentication with auto-rotating secret keys
//   - Tenant-based rate limiting with action-specific limits
//   - SQL injection protection with input validation
//   - IP whitelisting support
//   - Request validation and size limits
//   - Webhook signature validation
//   - Secure data handling with sensitive data masking
//   - Audit logging for all operations
//   - CORS and security headers
//
// # Development and Testing
//
// All providers support sandbox environments for development and testing.
// Test credentials and card numbers are available in each provider's documentation.
//
// For detailed provider-specific documentation, check the provider/{provider}/README.md files.
//
// # Examples
//
// Comprehensive examples are available in the examples/ directory:
//   - examples/iyzico_example.go - İyzico integration example
//   - examples/multi_tenant/ - Multi-tenant setup examples
//   - examples/*_curl_examples.sh - cURL examples for each provider
//
// # Production Deployment
//
// For production deployment:
//
//   - Use Kubernetes manifests in k8s/ directory
//   - Configure PostgreSQL for persistence
//   - Set up monitoring with Prometheus
//   - Use nginx for load balancing
//   - Enable rate limiting and security features
//   - Configure backup and disaster recovery
//
// For comprehensive documentation, visit the API documentation at /docs endpoint.
package gopay

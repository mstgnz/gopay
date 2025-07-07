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
// # Quick Start
//
// Basic usage example:
//
//	package main
//
//	import (
//	    "context"
//	    "github.com/mstgnz/gopay/provider"
//	    _ "github.com/mstgnz/gopay/provider/iyzico" // Import to register provider
//	)
//
//	func main() {
//	    // Create payment service
//	    service := provider.NewPaymentService()
//
//	    // Configure provider
//	    config := map[string]string{
//	        "apiKey":      "your-api-key",
//	        "secretKey":   "your-secret-key",
//	        "environment": "sandbox", // or "production"
//	    }
//
//	    // Add provider
//	    err := service.AddProvider("iyzico", config)
//	    if err != nil {
//	        panic(err)
//	    }
//
//	    // Create payment request
//	    paymentReq := provider.PaymentRequest{
//	        Amount:      100.50,
//	        Currency:    "TRY",
//	        CallbackURL: "https://yourapp.com/callback",
//	        Customer: provider.Customer{
//	            Name:    "John",
//	            Surname: "Doe",
//	            Email:   "john@example.com",
//	        },
//	        CardInfo: provider.CardInfo{
//	            CardNumber:  "5528790000000008",
//	            ExpireMonth: "12",
//	            ExpireYear:  "2030",
//	            CVV:         "123",
//	        },
//	    }
//
//	    // Process payment
//	    ctx := context.Background()
//	    response, err := service.CreatePayment(ctx, "iyzico", paymentReq)
//	    if err != nil {
//	        panic(err)
//	    }
//
//	    // Handle response
//	    if response.Success {
//	        // Payment successful or requires 3D authentication
//	        if response.ThreeDURL != "" {
//	            // Redirect user to 3D secure page
//	            fmt.Printf("Redirect to: %s\n", response.ThreeDURL)
//	        }
//	    }
//	}
//
// # Multi-Tenant Support
//
// GoPay supports multi-tenant architecture where different applications can use
// different provider configurations:
//
//	// Setup tenant-specific configuration
//	err := providerConfig.SetTenantConfig("APP1", "iyzico", map[string]string{
//	    "apiKey":      "app1-api-key",
//	    "secretKey":   "app1-secret-key",
//	    "environment": "sandbox",
//	})
//
//	// Use with tenant header
//	// X-Tenant-ID: APP1
//	// The system automatically routes to tenant-specific configuration
//
// # Environment Support
//
// Each provider supports both test (sandbox) and production environments:
//
//	config := map[string]string{
//	    "apiKey":      "your-api-key",
//	    "secretKey":   "your-secret-key",
//	    "environment": "production", // or "sandbox"
//	}
//
// # HTTP API
//
// GoPay also provides a REST API for integration:
//
//	# Create payment
//	POST /v1/payments/{provider}
//	Headers:
//	  Authorization: Bearer your-api-key
//	  X-Tenant-ID: your-tenant-id
//	  Content-Type: application/json
//
//	# Check payment status
//	GET /v1/payments/{provider}/{paymentID}
//
//	# Cancel payment
//	DELETE /v1/payments/{provider}/{paymentID}
//
//	# Process refund
//	POST /v1/payments/{provider}/refund
//
// # Callbacks and Webhooks
//
// GoPay handles provider callbacks and webhooks automatically:
//
//   - Callback URLs: /callback/{provider}?tenantId={tenantId}
//   - Webhook URLs: /webhooks/{provider}?tenantId={tenantId}
//
// The system preserves tenant information and routes responses back to
// the correct application.
//
// # Logging and Analytics
//
// GoPay integrates with PostgreSQL for comprehensive logging and analytics:
//
//   - Real-time payment tracking
//   - Provider-specific performance metrics
//   - Tenant-isolated logging
//   - Dashboard analytics
//
// # Configuration
//
// Configuration can be done via environment variables or programmatically:
//
//	# Environment variables
//	IYZICO_API_KEY=your-api-key
//	IYZICO_SECRET_KEY=your-secret-key
//	IYZICO_ENVIRONMENT=sandbox
//
//	# Or programmatically
//	config := map[string]string{
//	    "apiKey":      "your-api-key",
//	    "secretKey":   "your-secret-key",
//	    "environment": "sandbox",
//	}
//
// # Security Features
//
// GoPay includes several security features:
//
//   - API key authentication
//   - Rate limiting
//   - IP whitelisting
//   - Request validation
//   - Webhook signature validation
//   - Secure data handling
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
//   - examples/*_curl_examples.sh - cURL command examples
//
// # Contributing
//
// To add a new payment provider:
//
//  1. Implement the provider.PaymentProvider interface
//  2. Add the provider package under provider/{provider}/
//  3. Register the provider in provider/{provider}/register.go
//  4. Add comprehensive tests and documentation
//  5. Submit a pull request
//
// For more information, visit: https://github.com/mstgnz/gopay
package gopay

# GoPay

## Unified Payment Integration Service

GoPay is a modular payment integration service developed in Go. It abstracts different payment providers behind a single, standardized API, allowing developers to switch payment systems seamlessly without changing their codebase.

## Features

- **Unified API Interface**: Standardize diverse payment gateway APIs (Iyzico, OzanPay, Stripe, etc.) into a consistent format
- **Plug-and-Play Architecture**: Easily switch between payment providers without code changes
- **Provider Agnostic**: Add new payment gateways without disrupting existing implementations
- **Traceability**: Comprehensive logging with Elasticsearch integration
- **Microservice Ready**: Deploy as a standalone service in any architecture
- **Container Support**: Ready for Docker deployment with minimal configuration
- **Secure by Design**: Built-in callback authentication and security features

## Why GoPay?

Each payment provider implements their own unique API structure with different request formats, response schemas, and authentication methods. GoPay abstracts these differences away by:

1. Translating your standardized requests into provider-specific formats
2. Converting provider-specific responses into a consistent response format
3. Handling the complexities of each provider's authentication and security requirements

## How It Works

GoPay acts as a bridge between your application and payment providers, creating a unified integration layer:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │    │                 │
│   Your App      │◄──►│     GoPay       │◄──►│    Iyzico       │
│   (ABC)         │    │   (Bridge)      │    │ (Payment Sys)   │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Payment Flow Example

**Request Flow:**

```
ABC App ──► GoPay ──► Iyzico
   │           │         │
   │           │         ├─ Validates request
   │           │         ├─ Processes payment
   │           │         └─ Returns provider response
   │           │
   │           ├─ Converts to Iyzico format
   │           ├─ Handles authentication
   │           └─ Sends request
   │
   └─ Sends standard GoPay request
```

**Response Flow:**

```
ABC App ◄── GoPay ◄── Iyzico
   │           │         │
   │           │         └─ Returns Iyzico-specific response
   │           │
   │           ├─ Converts to standard format
   │           ├─ Normalizes error codes
   │           └─ Returns unified response
   │
   └─ Receives standard GoPay response
```

### Provider Switch Example

When switching from Iyzico to OzanPay, your application code remains unchanged:

```
# Before
POST /v1/payments/iyzico

# After
POST /v1/payments/ozanpay
```

No code changes needed in your application - just change the provider parameter!

## Deployment

GoPay is designed to be self-hosted. Simply clone the repository and deploy it within your infrastructure.

## Usage

### API Service

GoPay can be deployed as a standalone API service. This allows applications to integrate with payment providers without implementing each provider's API directly.

#### Setup and Deployment

1. Clone the repository:

   ```bash
   git clone https://github.com/mstgnz/gopay.git
   cd gopay
   ```

2. Copy the example env file and configure your payment providers:

   ```bash
   cp .env.example .env
   ```

   Edit the `.env` file with your payment provider credentials and API key:

   ```bash
   # Required: Set your API key for authentication
   API_KEY=your_super_secret_api_key_here

   # Configure your payment providers
   IYZICO_API_KEY=your_iyzico_api_key
   IYZICO_SECRET_KEY=your_iyzico_secret_key
   IYZICO_ENVIRONMENT=sandbox
   ```

3. Build and run the API service:

   ```bash
   go build -o gopay-api ./cmd/main.go
   ./gopay-api
   ```

   Or with Docker:

   ```bash
   docker-compose up -d
   ```

#### Authentication

All API requests require authentication using an API key. Include the API key in the Authorization header:

```bash
curl -H "Authorization: Bearer your_api_key_here" \
     -H "Content-Type: application/json" \
     -X POST http://localhost:9199/v1/payments \
     -d '{"amount": 100.50, "currency": "TRY", ...}'
```

#### Security Features

GoPay includes multiple security layers:

- ✅ **API Key Authentication**: Bearer token validation
- ✅ **Rate Limiting**: Configurable requests per minute (default: 100/min)
- ✅ **Security Headers**: HSTS, XSS protection, content type validation
- ✅ **Request Validation**: Content type and size limits (max 10MB)
- ✅ **IP Whitelisting**: Optional IP-based access control

#### API Endpoints

GoPay exposes the following API endpoints:

- **Process Payment**

  - `POST /v1/payments` - Process payment with default provider
  - `POST /v1/payments/{provider}` - Process payment with specific provider

- **Get Payment Status**

  - `GET /v1/payments/{paymentID}` - Get status with default provider
  - `GET /v1/payments/{provider}/{paymentID}` - Get status with specific provider

- **Cancel Payment**

  - `DELETE /v1/payments/{paymentID}` - Cancel with default provider
  - `DELETE /v1/payments/{provider}/{paymentID}` - Cancel with specific provider

- **Refund Payment**

  - `POST /v1/payments/refund` - Refund with default provider
  - `POST /v1/payments/{provider}/refund` - Refund with specific provider

- **3D Secure Callbacks**

  - `GET|POST /v1/callback` - 3D callback for default provider
  - `GET|POST /v1/callback/{provider}` - 3D callback for specific provider

- **Webhooks**
  - `POST /v1/webhooks/{provider}` - Webhook endpoint for provider notifications

#### 3D Secure Payment Flow

For 3D Secure payments:

1. Make a payment request with `"use3D": true`
2. In your request, include `"callbackUrl": "https://your-app.com/payment-callback"`
3. GoPay will return a response with `redirectURL` or `html` content for 3D authentication
4. After user completes authentication, they'll be redirected to your callback URL with payment result

You can also include additional parameters in your callback URL:

```
"callbackUrl": "https://your-app.com/payment-callback?successUrl=https://your-app.com/success&errorUrl=https://your-app.com/error"
```

This allows GoPay to redirect back to your application's success or error pages after processing.

### Library Use

In addition to the API service, GoPay can be used as a library in your Go applications.

### Adding Payment Providers

```go
import (
    "github.com/mstgnz/gopay/gateway"
    _ "github.com/mstgnz/gopay/gateway/iyzico"  // Import for side-effect registration
    _ "github.com/mstgnz/gopay/gateway/ozanpay" // Import for side-effect registration
)

// Create payment service
paymentService := gateway.NewPaymentService()

// Configure and add providers
iyzicoConfig := map[string]string{
    "apiKey":      "your-api-key",
    "secretKey":   "your-secret-key",
    "environment": "sandbox", // or "production"
}
paymentService.AddProvider("iyzico", iyzicoConfig)

// Set default provider
paymentService.SetDefaultProvider("iyzico")
```

### Processing Payments

```go
// Create payment request
paymentRequest := gateway.PaymentRequest{
    Amount:   100.50,
    Currency: "TRY",
    Customer: gateway.Customer{
        ID:      "customer123",
        Name:    "John",
        Surname: "Doe",
        Email:   "john@example.com",
    },
    CardInfo: gateway.CardInfo{
        CardHolderName: "John Doe",
        CardNumber:     "5528790000000008", // Test card
        ExpireMonth:    "12",
        ExpireYear:     "2030",
        CVV:            "123",
    },
    Description: "Test payment",
    Use3D:       false, // Set to true for 3D secure
}

// Process payment with default provider
response, err := paymentService.CreatePayment(context.Background(), "", paymentRequest)
if err != nil {
    log.Fatalf("Payment failed: %v", err)
}

// Check payment result
if response.Success {
    fmt.Printf("Payment successful! ID: %s\n", response.PaymentID)
} else {
    fmt.Printf("Payment failed: %s\n", response.Message)
}
```

### Using 3D Secure Payments

For 3D secure payments, the flow is:

1. Create a 3D secure payment request (set `Use3D: true`)
2. Get HTML content or redirect URL from the response
3. Show HTML or redirect the user to complete the 3D authentication
4. Process the callback with the returned data

```go
// Handle 3D secure callback
func callback(w http.ResponseWriter, r *http.Request) {
    // Get callback data from the request
    callbackData := make(map[string]string)
    // ... parse callback data from r.Form or r.PostForm

    // Complete the 3D payment
    response, err := paymentService.Complete3DPayment(
        context.Background(),
        "iyzico",                    // Provider name
        callbackData["paymentId"],   // Payment ID from callback
        callbackData["conversationId"], // Conversation ID
        callbackData,                // All callback data
    )

    // ... handle the response
}
```

See the `examples` directory for complete examples.

## Roadmap

- [x] Create core API structure and interfaces
- [ ] Implement logging and tracing middleware
- [x] Design unified payment response format
- [x] Design unified payment request format
- [ ] Implement webhook handling for callbacks
- [ ] Add authentication/security layer
- [ ] Create comprehensive documentation
- [x] Add example implementation
- [x] Add Iyzico payment provider integration
- [x] Add OzanPay payment provider integration
- [x] Add authentication/security layer
- [x] Implement rate limiting and security headers
- [ ] Add Stripe payment provider integration

## Contributing

This project is open-source, and contributions are welcome. Feel free to contribute or provide feedback of any kind.

## License

This project is licensed under the MIT License with attribution requirements - see the [LICENSE](LICENSE) file for details.

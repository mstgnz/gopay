# GoPay

## Unified Payment Integration Service

GoPay is a modular payment integration service developed in Go. It abstracts different payment providers behind a single, standardized API, allowing developers to switch payment systems seamlessly without changing their codebase.

## Why GoPay?

Each payment provider implements their own unique API structure with different request formats, response schemas, and authentication methods. GoPay abstracts these differences away by:

1. **Translating** your standardized requests into provider-specific formats
2. **Converting** provider-specific responses into a consistent response format
3. **Handling** the complexities of each provider's authentication and security requirements

## How It Works

GoPay acts as a bridge between your application and payment providers, creating a unified integration layer:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │    │                 │
│   Your App      │◄──►│     GoPay       │◄──►│   Payment       │
│                 │    │   (Bridge)      │    │   Provider      │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

**Provider Switch Example:**
When switching from one provider to another, your application code remains unchanged:

```bash
# İyzico
POST /v1/payments/iyzico

# OzanPay
POST /v1/payments/ozanpay

# Nkolay
POST /v1/payments/nkolay

# Papara
POST /v1/payments/papara
```

No code changes needed in your application - just change the provider parameter!

## Features

- ✅ **Unified API Interface**: Standardize diverse payment gateway APIs into a consistent format
- ✅ **Plug-and-Play Architecture**: Easily switch between payment providers without code changes
- ✅ **Provider Agnostic**: Add new payment gateways without disrupting existing implementations
- ✅ **3D Secure Support**: Built-in secure callback handling for 3D authentication flows
- ✅ **Microservice Ready**: Deploy as a standalone service in any architecture
- ✅ **Container Support**: Ready for Docker deployment with minimal configuration
- ✅ **Security Features**: API key authentication, rate limiting, and secure headers
- ✅ **OpenSearch Logging**: Comprehensive request/response logging with provider-specific indexing

## Supported Payment Providers

| Provider    | Status              | Features                           | Documentation                               |
| ----------- | ------------------- | ---------------------------------- | ------------------------------------------- |
| **İyzico**  | ✅ Production Ready | Payment, 3D Secure, Refund, Cancel | [İyzico Guide](provider/iyzico/README.md)   |
| **OzanPay** | ✅ Production Ready | Payment, 3D Secure, Refund         | [OzanPay Guide](provider/ozanpay/README.md) |
| **Paycell** | ✅ Production Ready | Payment, 3D Secure, Refund, Cancel | [Paycell Guide](provider/paycell/README.md) |
| **Papara**  | ✅ Production Ready | Payment, 3D Secure, Refund, Cancel | [Papara Guide](provider/papara/README.md)   |
| **Nkolay**  | ✅ Production Ready | Payment, 3D Secure, Refund, Cancel | [Nkolay Guide](provider/nkolay/README.md)   |
| **Stripe**  | 🚧 Coming Soon      | -                                  | -                                           |
| **PayTR**   | 📋 Planned          | -                                  | -                                           |

## Quick Start

### 1. Installation

```bash
git clone https://github.com/mstgnz/gopay.git
cd gopay
```

### 2. Configuration

```bash
cp .env.example .env
# Edit .env with your configuration
```

**Required Configuration:**

```bash
# Your API key for authentication
API_KEY=your_super_secret_api_key_here

# GoPay base URL for 3D Secure callbacks
APP_URL=https://your-gopay-domain.com

# OpenSearch logging configuration
OPENSEARCH_URL=http://localhost:9200
ENABLE_OPENSEARCH_LOGGING=true
LOGGING_LEVEL=info

# Payment provider credentials (example for İyzico)
IYZICO_API_KEY=your_iyzico_api_key
IYZICO_SECRET_KEY=your_iyzico_secret_key
IYZICO_ENVIRONMENT=sandbox
```

### 3. Run the Service

```bash
# With Go
go run ./cmd/main.go

# With Docker
docker-compose up -d

# With Make
make run
```

The service will start on `http://localhost:9999`

## Usage Examples

### API Service Usage

**Process a Payment:**

```bash
curl -X POST http://localhost:9999/v1/payments/iyzico \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john@example.com"
    },
    "cardInfo": {
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    }
  }'
```

**3D Secure Payment:**

```bash
curl -X POST http://localhost:9999/v1/payments/iyzico \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "use3D": true,
    "callbackUrl": "https://yourapp.com/payment-callback",
    "customer": {...},
    "cardInfo": {...}
  }'
```

### Library Usage

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/iyzico"
    _ "github.com/mstgnz/gopay/provider/nkolay"
    _ "github.com/mstgnz/gopay/provider/ozanpay"
    _ "github.com/mstgnz/gopay/provider/papara"
)

// Create payment service
paymentService := provider.NewPaymentService()

// Add İyzico provider
iyzicoConfig := map[string]string{
    "apiKey":      "your-api-key",
    "secretKey":   "your-secret-key",
    "environment": "sandbox",
}
paymentService.AddProvider("iyzico", iyzicoConfig)

// Add Nkolay provider
nkolayConfig := map[string]string{
    "apiKey":      "your-nkolay-api-key",
    "secretKey":   "your-nkolay-secret-key",
    "merchantId":  "your-merchant-id",
    "environment": "sandbox",
}
paymentService.AddProvider("nkolay", nkolayConfig)

// Process payment with any provider
response, err := paymentService.CreatePayment(ctx, "nkolay", paymentRequest)
```

## API Endpoints

| Method   | Endpoint                              | Description        |
| -------- | ------------------------------------- | ------------------ |
| `POST`   | `/v1/payments/{provider}`             | Process payment    |
| `GET`    | `/v1/payments/{provider}/{paymentID}` | Get payment status |
| `DELETE` | `/v1/payments/{provider}/{paymentID}` | Cancel payment     |
| `POST`   | `/v1/payments/{provider}/refund`      | Process refund     |
| `POST`   | `/v1/callback/{provider}`             | 3D Secure callback |
| `POST`   | `/v1/webhooks/{provider}`             | Webhook endpoint   |

**For detailed API documentation and provider-specific examples, see the individual provider documentation.**

## Payment Provider Documentation

Each payment provider has its own comprehensive documentation:

- **[İyzico Integration Guide](provider/iyzico/README.md)** - Complete İyzico setup, API examples, test cards, integration tests
- **[OzanPay Integration Guide](provider/ozanpay/README.md)** - OzanPay configuration and usage examples
- **[Paycell Integration Guide](provider/paycell/README.md)** - Paycell integration with REST API support

## OpenSearch Logging

GoPay includes comprehensive request/response logging with OpenSearch integration:

### Features

- **Provider-Specific Indexing**: Each payment provider has its own index (e.g., `gopay-iyzico-logs`)
- **Structured Logging**: All requests/responses are logged with structured data
- **Security**: Sensitive data (card numbers, API keys) are automatically redacted
- **Real-time Analytics**: Query and analyze payment data in real-time
- **Error Tracking**: Monitor and analyze payment failures

### Logging Statistics API

Get logging statistics for any provider:

```bash
# Get last 24 hours statistics for İyzico
GET /v1/stats?provider=iyzico&hours=24

# Example response:
{
  "aggregations": {
    "total_requests": { "value": 150 },
    "success_count": { "doc_count": 142 },
    "error_count": { "doc_count": 8 },
    "avg_processing_time": { "value": 245.5 },
    "status_codes": {
      "buckets": [
        { "key": 200, "doc_count": 142 },
        { "key": 400, "doc_count": 5 },
        { "key": 500, "doc_count": 3 }
      ]
    }
  }
}
```

### OpenSearch Queries

Example queries to search payment logs:

```bash
# Search for a specific payment ID
GET gopay-iyzico-logs/_search
{
  "query": {
    "match": {
      "payment_info.payment_id": "payment_123"
    }
  }
}

# Find recent errors
GET gopay-iyzico-logs/_search
{
  "query": {
    "bool": {
      "must": [
        { "range": { "timestamp": { "gte": "now-1h" } } },
        { "exists": { "field": "error.code" } }
      ]
    }
  }
}
```

### Configuration

Enable OpenSearch logging in your `.env` file:

```bash
# Enable OpenSearch logging
ENABLE_OPENSEARCH_LOGGING=true
OPENSEARCH_URL=http://localhost:9200
OPENSEARCH_USER=admin
OPENSEARCH_PASSWORD=admin
LOG_RETENTION_DAYS=30
```

## Development

### Available Commands

```bash
# Development workflow
make dev                 # Format, lint, and test
make test               # Run unit tests
make test-integration   # Run integration tests (requires credentials)
make build             # Build application
make run               # Run development server

# Integration tests
make integration-help   # Show integration test setup
make test-iyzico       # Test İyzico integration
```

### Adding New Payment Providers

1. Create provider directory: `gateway/newprovider/`
2. Implement the `PaymentProvider` interface
3. Add registration in `init()` function
4. Create comprehensive documentation in provider's README
5. Add integration tests

## Deployment

GoPay is designed to be self-hosted and can be deployed in various ways:

- **Docker**: Use provided `docker-compose.yml`
- **Kubernetes**: Ready for containerized deployment
- **Traditional**: Build and deploy the binary
- **Cloud**: Compatible with major cloud providers

## Security

- 🔒 **API Key Authentication**: Bearer token validation
- 🛡️ **Rate Limiting**: Configurable requests per minute
- 🔐 **Secure Headers**: HSTS, XSS protection, content validation
- 🔍 **Request Validation**: Content type and size limits
- 📊 **OpenSearch Integration**: Real-time request/response logging with advanced search capabilities

## Contributing

This project is open-source and contributions are welcome:

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the MIT License with attribution requirements - see the [LICENSE](LICENSE) file for details.

---

**Need help with a specific payment provider?** Check the provider-specific documentation in the `provider/` directory for detailed implementation guides, test cards, and integration examples.

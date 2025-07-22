# GoPay İyzico Integration

This documentation explains GoPay's İyzico payment provider integration.

[Iyzico API Documentation](https://docs.iyzico.com/)

## Features

- **Standard Card Payments**: Regular card payments
- **3D Secure Payments**: Secure 3D authenticated payments
- **Payment Inquiry**: Payment status checking
- **Payment Cancellation**: Payment cancellation
- **Refund Operations**: Full and partial refunds
- **Webhook Support**: Payment notifications
- **Test/Production Environment**: Sandbox and production support
- **Secure Callbacks**: Protected return URL management

## Configuration

### Environment Variables

Configure İyzico in your `.env` file:

```bash
# İyzico API Credentials
IYZICO_API_KEY=your_iyzico_api_key
IYZICO_SECRET_KEY=your_iyzico_secret_key
IYZICO_ENVIRONMENT=sandbox  # or "production"

# GoPay Base URL (required for 3D Secure callbacks)
APP_URL=https://your-gopay-domain.com
```

### Programmatic Configuration

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/iyzico"
)

paymentService := provider.NewPaymentService()

// Add İyzico provider
iyzicoConfig := map[string]string{
    "apiKey":      "your-api-key",
    "secretKey":   "your-secret-key",
    "environment": "sandbox", // "production" for production
}

paymentService.AddProvider("iyzico", iyzicoConfig)
paymentService.SetDefaultProvider("iyzico")
```

## API Endpoints

### Payment Processing

#### Standard Payment

```http
POST /v1/payments/iyzico
Content-Type: application/json
Authorization: Bearer your_jwt_token

{
  "amount": 100.50,
  "currency": "TRY",
  "use3D": false,
  "customer": {
    "name": "John",
    "surname": "Doe",
    "email": "john@example.com"
  },
  "cardInfo": {
    "cardHolderName": "John Doe",
    "cardNumber": "5528790000000008",
    "expireMonth": "12",
    "expireYear": "2030",
    "cvv": "123"
  }
}
```

#### 3D Secure Payment

```http
POST /v1/payments/iyzico
Content-Type: application/json
Authorization: Bearer your_jwt_token

{
  "amount": 250.75,
  "currency": "TRY",
  "use3D": true,
  "callbackUrl": "https://myapp.com/payment-result?successUrl=https://myapp.com/success&errorUrl=https://myapp.com/error",
  "customer": {
    "name": "Jane",
    "surname": "Smith",
    "email": "jane@example.com",
    "address": {
      "city": "Istanbul",
      "country": "Turkey",
      "address": "Besiktas Mah. Test Sok. No:1",
      "zipCode": "34100"
    }
  },
  "cardInfo": {
    "cardHolderName": "Jane Smith",
    "cardNumber": "5528790000000008",
    "expireMonth": "06",
    "expireYear": "2029",
    "cvv": "456"
  },
  "items": [
    {
      "id": "item1",
      "name": "Test Product",
      "category": "Electronics",
      "price": 250.75,
      "quantity": 1
    }
  ]
}
```

### Payment Management

#### Payment Status Inquiry

```http
GET /v1/payments/iyzico/{paymentId}
Authorization: Bearer your_jwt_token
```

#### Payment Cancellation

```http
DELETE /v1/payments/iyzico/{paymentId}
Authorization: Bearer your_jwt_token
Content-Type: application/json

{
  "reason": "Customer request"
}
```

#### Refund Operation

```http
POST /v1/payments/iyzico/refund
Authorization: Bearer your_jwt_token
Content-Type: application/json

{
  "paymentId": "payment_id_here",
  "refundAmount": 50.00,  // For partial refund, leave empty for full refund
  "reason": "Customer request",
  "description": "Refund upon customer request"
}
```

## Test Cards

Card numbers you can use for testing in İyzico sandbox environment:

### Successful Test Cards

- **5528790000000008** - Successful payment
- **4059030000000009** - Successful payment (Visa)

### Error Scenarios

- **5528790000000016** - Insufficient funds
- **5528790000000024** - Do not honor
- **5528790000000032** - Invalid card
- **5528790000000040** - Lost card
- **5528790000000057** - Stolen card
- **5528790000000065** - Expired card
- **5528790000000073** - Invalid security code
- **5528790000000081** - Invalid amount

### 3D Secure Test Cards

- **5528790000000008** - Successful 3D authentication
- **5528790000000016** - Failed 3D authentication

**Note:** For all test cards:

- **CVV**: Any 3-digit number (e.g., 123)
- **Expiry Date**: Any future date (e.g., 12/2030)

## 3D Secure Callback Flow

İyzico 3D Secure payments use GoPay's secure callback flow:

```
1. [Your App] ──→ [GoPay] ──→ [İyzico]
2. [İyzico] ──→ [User 3D Auth Page]
3. [User 3D Auth] ──→ [İyzico] ──→ [GoPay Callback] ──→ [Your App]
```

### Callback URL Structure

**Your Sent URL:**

```
https://myapp.com/payment-result?successUrl=https://myapp.com/success&errorUrl=https://myapp.com/error
```

**GoPay → İyzico URL:**

```
https://gopay.domain.com/v1/callback/iyzico?originalCallbackUrl=https://myapp.com/payment-result?successUrl=...
```

**Returned URLs:**

```bash
# Successful payment
https://myapp.com/success?paymentId=123&status=successful

# Failed payment
https://myapp.com/error?error=Payment failed
```

## Error Codes

Common error codes that may come from İyzico:

| Code | Description              |
| ---- | ------------------------ |
| 5006 | Insufficient funds       |
| 5007 | Invalid card information |
| 5053 | Insufficient limit       |
| 5208 | Suspected fraud          |

## Response Formats

### Successful Payment Response

```json
{
  "success": true,
  "status": "successful",
  "paymentId": "12345678",
  "transactionId": "87654321",
  "amount": 100.5,
  "currency": "TRY",
  "message": "Payment successful",
  "systemTime": "2024-01-15T10:30:00Z"
}
```

### 3D Secure Response

```json
{
  "success": true,
  "status": "pending",
  "paymentId": "12345678",
  "html": "<html>3D form content...</html>",
  "message": "3D Secure authentication required"
}
```

### Error Response

```json
{
  "success": false,
  "status": "failed",
  "errorCode": "5006",
  "message": "Insufficient funds",
  "systemTime": "2024-01-15T10:30:00Z"
}
```

## Production Migration

To migrate from sandbox to production:

1. Obtain your production API keys from İyzico
2. Update environment variables:
   ```bash
   IYZICO_ENVIRONMENT=production
   IYZICO_API_KEY=production_api_key
   IYZICO_SECRET_KEY=production_secret_key
   ```
3. Test with real card information
4. Point your webhook URLs to production environment

## Integration Tests

GoPay İyzico integration includes comprehensive real API tests that verify the integration works correctly with İyzico's sandbox environment.

### Test Coverage

#### **Unit Tests** (`iyzico_test.go`)

- Provider initialization and configuration
- Request validation and mapping
- Response parsing and error handling
- Authentication string generation
- Mock HTTP server scenarios

#### **Integration Tests** (`iyzico_integration_test.go`)

- **Real İyzico Sandbox API**: Tests against actual İyzico endpoints
- **Payment Processing**: Create payments with test cards
- **Error Scenarios**: Test various error conditions (insufficient funds, invalid cards)
- **3D Secure Flow**: Test 3D payment initiation
- **Payment Management**: Status checking, refunds, cancellations
- **Authentication**: Test invalid credentials handling
- **Performance**: Benchmark tests for payment processing
- **Full Workflow**: End-to-end payment lifecycle testing

### Running Tests

#### Unit Tests (Always Available)

```bash
# Run all unit tests
go test ./gateway/iyzico/

# Run specific test
go test ./gateway/iyzico/ -run TestNewProvider

# Run with verbose output
go test -v ./gateway/iyzico/
```

#### Integration Tests (Requires İyzico Credentials)

**Prerequisites:**

1. İyzico sandbox account
2. Valid sandbox API credentials

**Setup:**

```bash
# Set required environment variables
export IYZICO_TEST_ENABLED=true
export IYZICO_TEST_API_KEY=your_sandbox_api_key
export IYZICO_TEST_SECRET_KEY=your_sandbox_secret_key

# Run integration tests
go test ./gateway/iyzico/ -run TestIntegration

# Run specific integration test
go test ./gateway/iyzico/ -run TestIntegration_CreatePayment_Success

# Run full workflow test
go test ./gateway/iyzico/ -run TestIntegration_FullWorkflow -v
```

### Test Scenarios

#### Payment Success Tests

- **Valid Payment**: Uses İyzico test card `5528790000000008`
- **Amount Verification**: Confirms request/response amount matching
- **Currency Handling**: Tests TRY currency processing
- **Customer Data**: Validates customer information handling

#### Error Handling Tests

- **Insufficient Funds**: Card `5528790000000016`
- **Invalid Card**: Card `5528790000000032`
- **Authentication Failure**: Invalid API credentials
- **Request Timeout**: Context timeout handling

#### 3D Secure Tests

- **3D Initiation**: Tests 3D payment flow start
- **Callback URL**: Validates callback URL generation
- **HTML Content**: Checks 3D form content reception
- **Status Verification**: Confirms pending status for 3D payments

#### Payment Lifecycle Tests

- **Status Checking**: Retrieve payment status after creation
- **Partial Refunds**: Process partial refund amounts
- **Payment Cancellation**: Cancel existing payments
- **Full Workflow**: Complete payment → status → refund cycle

### CI/CD Integration

**GitHub Actions Example:**

```yaml
name: İyzico Integration Tests
on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: "1.21"

      - name: Run Unit Tests
        run: go test ./gateway/iyzico/ -v

      - name: Run Integration Tests
        if: ${{ secrets.IYZICO_TEST_API_KEY }}
        env:
          IYZICO_TEST_ENABLED: true
          IYZICO_TEST_API_KEY: ${{ secrets.IYZICO_TEST_API_KEY }}
          IYZICO_TEST_SECRET_KEY: ${{ secrets.IYZICO_TEST_SECRET_KEY }}
        run: go test ./gateway/iyzico/ -run TestIntegration -v
```

## Security

- 🔒 All API communication over HTTPS
- 🔐 Request signing with HMAC-SHA1
- 🛡️ Callback URLs protected by GoPay
- 🔍 Fraud detection and risk analysis
- 📊 Real-time transaction monitoring

## Support

For questions regarding İyzico integration:

- İyzico API Documentation: https://dev.iyzipay.com/
- GoPay GitHub Issues: https://github.com/mstgnz/gopay/issues

# Stripe Payment Provider

This document provides comprehensive information about integrating Stripe payments with GoPay.

[Stripe API Documentation](https://docs.stripe.com/api)

## Overview

Stripe is one of the most popular payment processing platforms globally, offering a complete suite of payment solutions for businesses of all sizes. This provider implementation supports:

- **Direct Payments**: Non-3D secure card payments
- **3D Secure Payments**: Enhanced security with SCA compliance
- **Payment Status Inquiry**: Real-time payment status tracking
- **Payment Cancellation**: Cancel pending payments
- **Refunds**: Full and partial refund support
- **Webhook Validation**: Secure webhook handling
- **International Cards**: Support for global card networks
- **Multiple Currencies**: 135+ supported currencies
- **Modern API**: Uses latest Stripe Go library (v82+)

## Technical Implementation

This provider uses the official **Stripe Go library v82** with the modern `stripe.Client` API. The implementation has been updated to use the latest best practices and eliminates all deprecation warnings.

### Key Features

- **Official Stripe Go Library**: Uses `github.com/stripe/stripe-go/v82`
- **Modern Client API**: Implements the new `stripe.Client` (not deprecated `client.API`)
- **Payment Method + PaymentIntent Flow**: Follows Stripe's recommended payment flow
- **Comprehensive Error Handling**: Handles all Stripe error scenarios
- **3D Secure Support**: Full SCA compliance with proper redirect handling
- **Test API Integration**: Direct integration with Stripe's test environment

## Configuration

### Environment Variables

Add these configuration values to your `.env` file:

```bash
# Stripe Configuration
STRIPE_SECRET_KEY=sk_test_your_stripe_secret_key_here
STRIPE_PUBLIC_KEY=pk_test_your_stripe_public_key_here
STRIPE_ENVIRONMENT=sandbox  # or "production"

# GoPay Configuration
APP_URL=https://your-gopay-domain.com
```

### Provider Registration

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/stripe"  // Import to register
)

// Configure Stripe provider
stripeConfig := map[string]string{
    "secretKey":     "sk_test_your_stripe_secret_key_here",
    "environment":   "sandbox", // or "production"
    "gopayBaseURL":  "https://your-gopay-domain.com", // Optional, defaults to APP_URL
}

paymentService := provider.NewPaymentService()
err := paymentService.AddProvider("stripe", stripeConfig)
```

## API Usage

### 1. Direct Payment (Non-3D)

```bash
curl -X POST http://localhost:9999/v1/payments/stripe \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "USD",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john.doe@example.com",
      "address": {
        "address": "123 Main St",
        "city": "New York",
        "country": "US",
        "zipCode": "10001"
      }
    },
    "cardInfo": {
      "cardHolderName": "John Doe",
      "cardNumber": "4242424242424242",
      "expireMonth": "12",
      "expireYear": "2028",
      "cvv": "123"
    },
    "description": "Test payment",
    "referenceId": "order_12345"
  }'
```

### 2. 3D Secure Payment

```bash
curl -X POST http://localhost:9999/v1/payments/stripe \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "USD",
    "use3D": true,
    "callbackUrl": "https://yourapp.com/payment-callback",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john.doe@example.com"
    },
    "cardInfo": {
      "cardHolderName": "John Doe",
      "cardNumber": "4000000000003220",
      "expireMonth": "12",
      "expireYear": "2028",
      "cvv": "123"
    }
  }'
```

**3D Secure Response:**

```json
{
  "success": true,
  "status": "pending",
  "paymentId": "pi_1234567890abcdef",
  "redirectUrl": "https://js.stripe.com/v3/authorize-with-url...",
  "message": "Payment requires additional action"
}
```

### 3. Payment Status Inquiry

```bash
curl -X GET http://localhost:9999/v1/payments/stripe/pi_1234567890abcdef \
  -H "Authorization: Bearer your_jwt_token"
```

### 4. Cancel Payment

```bash
curl -X DELETE http://localhost:9999/v1/payments/stripe/pi_1234567890abcdef \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "Customer requested cancellation"
  }'
```

### 5. Refund Payment

```bash
curl -X POST http://localhost:9999/v1/payments/stripe/refund \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "pi_1234567890abcdef",
    "refundAmount": 50.25,
    "reason": "Customer return",
    "description": "Partial refund for returned item"
  }'
```

## Test Cards

### Successful Payment Cards

| Card Number        | Brand                 | Description          |
| ------------------ | --------------------- | -------------------- |
| `4242424242424242` | Visa                  | Default success card |
| `4000056655665556` | Visa (debit)          | Debit card success   |
| `5555555555554444` | Mastercard            | Mastercard success   |
| `2223003122003222` | Mastercard (2-series) | 2-series Mastercard  |
| `5200828282828210` | Mastercard (debit)    | Debit Mastercard     |
| `378282246310005`  | American Express      | Amex success         |

### 3D Secure Test Cards

| Card Number        | Description                              |
| ------------------ | ---------------------------------------- |
| `4000000000003220` | 3D Secure authentication required        |
| `4000000000003238` | 3D Secure authentication may be required |
| `4000000000003246` | 3D Secure authentication unavailable     |

### Declined Cards

| Card Number        | Decline Code       | Description        |
| ------------------ | ------------------ | ------------------ |
| `4000000000000002` | generic_decline    | Generic decline    |
| `4000000000009995` | insufficient_funds | Insufficient funds |
| `4000000000009987` | lost_card          | Lost card          |
| `4000000000009979` | stolen_card        | Stolen card        |
| `4000000000000069` | expired_card       | Expired card       |
| `4000000000000127` | incorrect_cvc      | Incorrect CVC      |

### International Cards

| Card Number        | Country   | Description    |
| ------------------ | --------- | -------------- |
| `4000000760000002` | Brazil    | Brazil Visa    |
| `4000001240000000` | Canada    | Canada Visa    |
| `4000000250000003` | France    | France Visa    |
| `4000000276000005` | Germany   | Germany Visa   |
| `4000001560000002` | Singapore | Singapore Visa |

## Supported Currencies

Stripe supports 135+ currencies. Here are some common ones:

| Currency          | Code | Name                   |
| ----------------- | ---- | ---------------------- |
| US Dollar         | USD  | United States Dollar   |
| Euro              | EUR  | Euro                   |
| British Pound     | GBP  | British Pound Sterling |
| Turkish Lira      | TRY  | Turkish Lira           |
| Japanese Yen      | JPY  | Japanese Yen           |
| Canadian Dollar   | CAD  | Canadian Dollar        |
| Australian Dollar | AUD  | Australian Dollar      |

## Response Codes

### Payment Status Mapping

| Stripe Status             | GoPay Status | Description                       |
| ------------------------- | ------------ | --------------------------------- |
| `succeeded`               | `successful` | Payment completed successfully    |
| `requires_action`         | `pending`    | Requires 3D Secure authentication |
| `requires_confirmation`   | `pending`    | Requires confirmation             |
| `processing`              | `processing` | Payment being processed           |
| `requires_capture`        | `processing` | Authorized, awaiting capture      |
| `canceled`                | `cancelled`  | Payment cancelled                 |
| `requires_payment_method` | `failed`     | Payment method failed             |

### Common Error Codes

| Error Code                | Description        | Action                       |
| ------------------------- | ------------------ | ---------------------------- |
| `card_declined`           | Card was declined  | Try different payment method |
| `expired_card`            | Card has expired   | Use valid expiry date        |
| `incorrect_cvc`           | Invalid CVC code   | Check CVC and retry          |
| `insufficient_funds`      | Not enough balance | Contact card issuer          |
| `processing_error`        | Processing issue   | Retry payment                |
| `authentication_required` | 3D Secure needed   | Complete authentication      |

## Webhooks

### Webhook Configuration

Stripe sends webhooks to notify about payment events. Configure your webhook endpoint in the Stripe Dashboard:

```
Webhook URL: https://your-domain.com/v1/webhooks/stripe
Events to send: payment_intent.succeeded, payment_intent.payment_failed
```

### Webhook Security

Webhooks are validated using Stripe's signature verification:

```go
isValid, eventData, err := paymentService.ValidateWebhook(
    ctx,
    "stripe",
    webhookData,
    headers,
)
```

## Library Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/stripe"
)

func main() {
    // Initialize payment service
    paymentService := provider.NewPaymentService()

    // Configure Stripe with modern API
    stripeConfig := map[string]string{
        "secretKey":   "sk_test_...",
        "environment": "sandbox",
    }

    err := paymentService.AddProvider("stripe", stripeConfig)
    if err != nil {
        log.Fatal("Failed to add Stripe provider:", err)
    }

    // Create payment request
    paymentRequest := provider.PaymentRequest{
        Amount:   25.99,
        Currency: "USD",
        Customer: provider.Customer{
            Name:    "John",
            Surname: "Doe",
            Email:   "john.doe@example.com",
            Address: provider.Address{
                Address: "123 Main St",
                City:    "New York",
                Country: "US",
                ZipCode: "10001",
            },
        },
        CardInfo: provider.CardInfo{
            CardHolderName: "John Doe",
            CardNumber:     "4242424242424242",
            ExpireMonth:    "12",
            ExpireYear:     "2028",
            CVV:            "123",
        },
        Description:  "Test payment",
        ReferenceID:  "order_12345",
    }

    // Process payment
    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "stripe", paymentRequest)
    if err != nil {
        log.Fatal("Payment failed:", err)
    }

    fmt.Printf("Payment Status: %s\n", response.Status)
    fmt.Printf("Payment ID: %s\n", response.PaymentID)
    fmt.Printf("Transaction ID: %s\n", response.TransactionID)
    fmt.Printf("Amount: $%.2f %s\n", response.Amount, response.Currency)

    if response.Success {
        fmt.Println("Payment completed successfully!")
    } else {
        fmt.Printf("❌ Payment failed: %s\n", response.Message)
    }
}
```

## Integration Testing

### Test Environment Setup

The integration tests are configured to use **real Stripe test API keys** directly in the code, following the pattern used by other providers in the project. This ensures reliable testing without environment variable dependencies.

1. **Automatic Test Configuration**: Tests use embedded test API keys (safe for Stripe test environment)
2. **Real API Testing**: Tests make actual calls to Stripe's test environment
3. **Comprehensive Coverage**: All payment flows are tested with real responses

### Running Integration Tests

```bash
# Run all integration tests
go test -v ./provider/stripe/ -run Integration

# Run specific tests
go test -v ./provider/stripe/ -run TestStripeIntegration_DirectPayment
go test -v ./provider/stripe/ -run TestStripeIntegration_3DSecure
go test -v ./provider/stripe/ -run TestStripeIntegration_DeclinedPayment

# Run all tests with coverage
go test -v -cover ./provider/stripe/
```

### Example Test Output

```bash
=== RUN   TestStripeIntegration_DirectPayment
    stripe_integration_test.go:66: Payment Response: &{Success:true Status:successful Message:Payment successful TransactionID:ch_3Rh7Rp2x6R10KRrh01q4fqSE PaymentID:pi_3Rh7Rp2x6R10KRrh0anvkfzW Amount:25.99 Currency:USD}
    stripe_integration_test.go:93: Status Response: &{Success:true Status:successful Message:Payment successful}
--- PASS: TestStripeIntegration_DirectPayment (2.36s)
PASS
```

## Production Checklist

- [ ] Replace test API keys with live API keys
- [ ] Set `environment` to `"production"`
- [ ] Configure webhook endpoints in Stripe Dashboard
- [ ] Test webhook signature validation
- [ ] Implement proper error handling
- [ ] Set up monitoring and logging
- [ ] Configure rate limiting
- [ ] Test with real payment methods
- [ ] Implement proper security measures
- [ ] Review Stripe's security guidelines

## Security Considerations

1. **API Key Security**: Never expose secret keys in client-side code
2. **Webhook Validation**: Always validate webhook signatures
3. **HTTPS Only**: Use HTTPS for all API communications
4. **PCI Compliance**: Follow PCI DSS requirements
5. **Data Encryption**: Encrypt sensitive payment data
6. **Audit Logging**: Log all payment transactions
7. **Rate Limiting**: Implement API rate limiting
8. **Error Handling**: Don't expose sensitive error details
9. **Modern API**: Uses latest Stripe Go library with security improvements

## Performance and Reliability

- **Connection Pooling**: Efficient HTTP connection management
- **Retry Logic**: Built-in retry mechanisms for transient failures
- **Timeout Handling**: Proper timeout configuration
- **Error Recovery**: Graceful error handling and recovery
- **Test Coverage**: Comprehensive integration test suite

## Support and Documentation

- **Stripe Documentation**: [https://stripe.com/docs](https://stripe.com/docs)
- **API Reference**: [https://stripe.com/docs/api](https://stripe.com/docs/api)
- **Dashboard**: [https://dashboard.stripe.com](https://dashboard.stripe.com)
- **Status Page**: [https://status.stripe.com](https://status.stripe.com)
- **Community**: [https://github.com/stripe](https://github.com/stripe)

## Limitations

- **Card Storage**: This implementation creates single-use payment methods
- **Subscriptions**: Subscription handling not implemented in this version
- **Connect**: Stripe Connect features not included
- **Radar**: Advanced fraud detection features not configured

## Changelog

### v1.1.0 (2025-01-04)

- **Major Update**: Migrated to modern Stripe Go library v82
- **Deprecation Fix**: Replaced deprecated `client.API` with `stripe.Client`
- **API Modernization**: Updated all API calls to use new client methods
- **Parameter Updates**: Migrated to `*CreateParams` types for all operations
- **Test Integration**: Verified with real Stripe test API
- **Performance**: Improved error handling and response mapping
- **Documentation**: Updated with modern implementation details

### v1.0.0 (Previous)

- Initial Stripe provider implementation
- Support for direct and 3D Secure payments
- Payment status tracking
- Cancellation and refund support
- Webhook validation framework

## Migration Notes

If you're updating from a previous version, note these changes:

1. **No Configuration Changes**: Provider configuration remains the same
2. **API Compatibility**: All public APIs remain unchanged
3. **Improved Reliability**: Better error handling and response processing
4. **Deprecation Resolved**: No more deprecation warnings from Stripe library
5. **Test Coverage**: Enhanced integration testing with real API calls

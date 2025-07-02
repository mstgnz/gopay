# GoPay OzanPay Provider

OzanPay payment gateway integration for GoPay library. This provider supports all major payment operations including 3D Secure payments, refunds, cancellations, and webhook validation.

## Features

- ✅ **Standard Card Payments** - Direct card payments without 3D Secure
- ✅ **3D Secure Payments** - Enhanced security with 3D authentication
- ✅ **Payment Status Query** - Real-time payment status checking
- ✅ **Refund Operations** - Full and partial refunds
- ✅ **Payment Cancellation** - Cancel pending/authorized payments
- ✅ **Webhook Validation** - Secure webhook signature validation
- ✅ **Multi-Currency Support** - Support for multiple currencies
- ✅ **Test Environment** - Comprehensive sandbox testing
- ✅ **Error Handling** - Detailed error codes and messages
- ✅ **Request Validation** - Input validation before API calls

## Installation

The OzanPay provider is included with the main GoPay library. No additional installation required.

```bash
go get github.com/mstgnz/gopay
```

## Configuration

### Environment Variables

Set the following environment variables for OzanPay configuration:

```bash
# OzanPay Credentials
OZANPAY_API_KEY=your_api_key_here
OZANPAY_SECRET_KEY=your_secret_key_here
OZANPAY_MERCHANT_ID=your_merchant_id_here

# Environment (sandbox or production)
OZANPAY_ENVIRONMENT=sandbox

# Your application's base URL for callbacks
APP_URL=https://yourdomain.com
```

### Programmatic Configuration

```go
package main

import (
    "log"
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/ozanpay" // Import for side-effect registration
)

func main() {
    // Create payment service
    paymentService := provider.NewPaymentService()

    // Configure OzanPay
    ozanpayConfig := map[string]string{
        "apiKey":        "your_api_key",
        "secretKey":     "your_secret_key",
        "merchantId":    "your_merchant_id",
        "environment":   "sandbox", // or "production"
        "gopayBaseURL":  "https://yourdomain.com",
    }

    // Add OzanPay provider
    err := paymentService.AddProvider("ozanpay", ozanpayConfig)
    if err != nil {
        log.Fatal("Failed to add OzanPay provider:", err)
    }

    // Set as default provider
    err = paymentService.SetDefaultProvider("ozanpay")
    if err != nil {
        log.Fatal("Failed to set default provider:", err)
    }
}
```

## Usage Examples

### Basic Payment

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mstgnz/gopay/provider"
)

func createBasicPayment(paymentService *provider.PaymentService) {
    request := provider.PaymentRequest{
        Amount:   100.50,
        Currency: "USD",
        Customer: provider.Customer{
            ID:      "customer_123",
            Name:    "John",
            Surname: "Doe",
            Email:   "john.doe@example.com",
            Address: provider.Address{
                Country: "US",
                City:    "New York",
                Address: "123 Main St",
                ZipCode: "10001",
            },
        },
        CardInfo: provider.CardInfo{
            CardHolderName: "John Doe",
            CardNumber:     "4111111111111111", // Test card
            CVV:            "123",
            ExpireMonth:    "12",
            ExpireYear:     "2030",
        },
        Description: "Test payment",
    }

    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "ozanpay", request)
    if err != nil {
        log.Printf("Payment failed: %v", err)
        return
    }

    if response.Success {
        fmt.Printf("Payment successful! ID: %s\n", response.PaymentID)
    } else {
        fmt.Printf("Payment failed: %s\n", response.Message)
    }
}
```

### 3D Secure Payment

```go
func create3DPayment(paymentService *provider.PaymentService) {
    request := provider.PaymentRequest{
        Amount:   200.00,
        Currency: "USD",
        Customer: provider.Customer{
            Name:    "Jane",
            Surname: "Smith",
            Email:   "jane.smith@example.com",
        },
        CardInfo: provider.CardInfo{
            CardHolderName: "Jane Smith",
            CardNumber:     "4000000000000044", // 3D test card
            CVV:            "123",
            ExpireMonth:    "12",
            ExpireYear:     "2030",
        },
        CallbackURL: "https://yourdomain.com/payment/callback",
        Use3D:       true,
    }

    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "ozanpay", request)
    if err != nil {
        log.Printf("3D Payment initiation failed: %v", err)
        return
    }

    if response.Status == provider.StatusPending && response.RedirectURL != "" {
        fmt.Printf("Redirect user to: %s\n", response.RedirectURL)
        // User will be redirected to 3D secure page
        // After authentication, they'll return to your callback URL
    }
}
```

### Complete 3D Payment

```go
func complete3DPayment(paymentService *provider.PaymentService, paymentID string, callbackData map[string]string) {
    ctx := context.Background()

    response, err := paymentService.Complete3DPayment(ctx, "ozanpay", paymentID, "", callbackData)
    if err != nil {
        log.Printf("3D Payment completion failed: %v", err)
        return
    }

    if response.Success {
        fmt.Printf("3D Payment completed successfully! ID: %s\n", response.PaymentID)
    } else {
        fmt.Printf("3D Payment failed: %s\n", response.Message)
    }
}
```

### Payment Status Check

```go
func checkPaymentStatus(paymentService *provider.PaymentService, paymentID string) {
    ctx := context.Background()

    response, err := paymentService.GetPaymentStatus(ctx, "ozanpay", paymentID)
    if err != nil {
        log.Printf("Status check failed: %v", err)
        return
    }

    fmt.Printf("Payment Status: %s\n", response.Status)
    fmt.Printf("Success: %v\n", response.Success)
    if response.Message != "" {
        fmt.Printf("Message: %s\n", response.Message)
    }
}
```

### Refund Payment

```go
func refundPayment(paymentService *provider.PaymentService, paymentID string) {
    refundRequest := provider.RefundRequest{
        PaymentID:      paymentID,
        RefundAmount:   50.00, // Partial refund
        Reason:         "Customer request",
        Description:    "Refund for order #12345",
        ConversationID: "refund_conv_123",
    }

    ctx := context.Background()
    response, err := paymentService.RefundPayment(ctx, "ozanpay", refundRequest)
    if err != nil {
        log.Printf("Refund failed: %v", err)
        return
    }

    if response.Success {
        fmt.Printf("Refund successful! Refund ID: %s\n", response.RefundID)
    } else {
        fmt.Printf("Refund failed: %s\n", response.Message)
    }
}
```

### Cancel Payment

```go
func cancelPayment(paymentService *provider.PaymentService, paymentID string) {
    ctx := context.Background()

    response, err := paymentService.CancelPayment(ctx, "ozanpay", paymentID, "Customer cancellation")
    if err != nil {
        log.Printf("Cancellation failed: %v", err)
        return
    }

    if response.Success {
        fmt.Printf("Payment cancelled successfully!\n")
    } else {
        fmt.Printf("Cancellation failed: %s\n", response.Message)
    }
}
```

## Webhook Integration

### Webhook Validation

```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    // Parse webhook data
    var webhookData map[string]string
    if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Extract headers
    headers := make(map[string]string)
    for key, values := range r.Header {
        if len(values) > 0 {
            headers[key] = values[0]
        }
    }

    // Validate webhook
    ctx := context.Background()
    valid, paymentData, err := paymentService.ValidateWebhook(ctx, "ozanpay", webhookData, headers)
    if err != nil {
        log.Printf("Webhook validation failed: %v", err)
        http.Error(w, "Validation failed", http.StatusBadRequest)
        return
    }

    if !valid {
        log.Println("Invalid webhook signature")
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Process webhook
    paymentID := paymentData["paymentId"]
    status := paymentData["status"]

    fmt.Printf("Webhook received for payment %s with status %s\n", paymentID, status)

    // Update your database, send notifications, etc.

    w.WriteHeader(http.StatusOK)
}
```

## Test Cards

### Successful Payment Cards

| Card Number      | Description        |
| ---------------- | ------------------ |
| 4111111111111111 | Visa Success       |
| 5555555555554444 | Mastercard Success |

### Test Failure Cards

| Card Number      | Error Type             |
| ---------------- | ---------------------- |
| 4000000000000002 | Insufficient Funds     |
| 4000000000000341 | Card Declined          |
| 4000000000000069 | Expired Card           |
| 4000000000000127 | Invalid Card           |
| 4000000000000259 | Fraudulent Transaction |

### 3D Secure Test Cards

| Card Number      | Description        |
| ---------------- | ------------------ |
| 4000000000000044 | 3D Secure Redirect |

## API Reference

### Payment Statuses

- `pending` - Payment is being processed
- `processing` - Payment is in progress
- `successful` - Payment completed successfully
- `failed` - Payment failed
- `cancelled` - Payment was cancelled
- `refunded` - Payment was refunded

### Error Codes

- `INSUFFICIENT_FUNDS` - Card has insufficient funds
- `INVALID_CARD` - Invalid card number or details
- `EXPIRED_CARD` - Card has expired
- `FRAUDULENT_TRANSACTION` - Transaction flagged as fraudulent
- `CARD_DECLINED` - Card declined by issuer

## Testing

### Unit Tests

```bash
go test ./provider/ozanpay/... -v
```

### Integration Tests

```bash
go test ./provider/ozanpay/... -v -tags=integration
```

### Test Coverage

```bash
go test ./provider/ozanpay/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Security Considerations

1. **Never log sensitive data** - Card numbers, CVV, or API keys
2. **Use HTTPS** - Always use secure connections
3. **Validate webhooks** - Always validate webhook signatures
4. **Environment separation** - Use different credentials for sandbox/production
5. **API key rotation** - Regularly rotate your API keys

## Support

For OzanPay specific issues:

- OzanPay Documentation: [Official Docs](https://developer.ozan.com)
- OzanPay Support: support@ozan.com

For GoPay library issues:

- GitHub Issues: [Create an issue](https://github.com/mstgnz/gopay/issues)

## License

This provider is part of the GoPay library and follows the same license terms.

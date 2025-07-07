# PayU Turkey Payment Provider

This document provides comprehensive information about integrating PayU Turkey payments with GoPay.

[PayU API Documentation](https://docs.payu.in/docs/introduction)

## Overview

PayU Turkey is a leading payment processing platform in Turkey, offering secure and reliable payment solutions for businesses. This provider implementation supports:

- ✅ **Direct Payments**: Non-3D secure card payments
- ✅ **3D Secure Payments**: Enhanced security with SCA compliance
- ✅ **Payment Status Inquiry**: Real-time payment status tracking
- ✅ **Payment Cancellation**: Cancel pending payments
- ✅ **Refunds**: Full and partial refund support
- ✅ **Webhook Validation**: Secure webhook handling with signature verification
- ✅ **Multi-currency Support**: TRY, USD, EUR and other major currencies
- ✅ **Turkish Market Focus**: Optimized for Turkish payment ecosystem

## Configuration

### Environment Variables

Add these configuration values to your `.env` file:

```bash
# PayU Turkey Configuration
PAYU_MERCHANT_ID=your_payu_merchant_id
PAYU_SECRET_KEY=your_payu_secret_key
PAYU_ENVIRONMENT=sandbox  # or "production"

# GoPay Configuration
APP_URL=https://your-gopay-domain.com
```

### Provider Registration

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/payu"  // Import to register
)

// Configure PayU Turkey provider
payuConfig := map[string]string{
    "merchantId":   "your_payu_merchant_id",
    "secretKey":    "your_payu_secret_key",
    "environment":  "sandbox", // or "production"
    "gopayBaseURL": "https://your-gopay-domain.com",
}

paymentService := provider.NewPaymentService()
err := paymentService.AddProvider("payu", payuConfig)
```

## API Usage

### 1. Direct Payment (Non-3D)

```bash
curl -X POST "http://localhost:9999/v1/payments/payu" \
-H "Content-Type: application/json" \
-d '{
    "amount": 150.75,
    "currency": "TRY",
    "description": "PayU Test Payment",
    "customer": {
        "name": "Ahmet",
        "surname": "Yılmaz",
        "email": "ahmet.yilmaz@example.com",
        "phoneNumber": "+905551234567",
        "address": {
            "address": "Atatürk Caddesi No:123",
            "city": "İstanbul",
            "country": "TR",
            "zipCode": "34000"
        }
    },
    "cardInfo": {
        "cardHolderName": "Ahmet Yılmaz",
        "cardNumber": "5528790000000008",
        "expireMonth": "12",
        "expireYear": "2030",
        "cvv": "123"
    },
    "use3D": false
}'
```

### 2. 3D Secure Payment

```bash
curl -X POST "http://localhost:9999/v1/payments/payu" \
-H "Content-Type: application/json" \
-d '{
    "amount": 250.00,
    "currency": "TRY",
    "description": "PayU 3D Secure Payment",
    "callbackUrl": "https://yoursite.com/payment/callback",
    "customer": {
        "name": "Mehmet",
        "surname": "Demir",
        "email": "mehmet.demir@example.com",
        "phoneNumber": "+905559876543"
    },
    "cardInfo": {
        "cardHolderName": "Mehmet Demir",
        "cardNumber": "4059030000000009",
        "expireMonth": "06",
        "expireYear": "2029",
        "cvv": "456"
    },
    "use3D": true
}'
```

### 3. Payment Status Inquiry

```bash
curl -X GET "http://localhost:9999/v1/payments/payu/{paymentId}" \
-H "Content-Type: application/json"
```

### 4. Refund Payment

```bash
curl -X POST "http://localhost:9999/v1/payments/payu/{paymentId}/refund" \
-H "Content-Type: application/json" \
-d '{
    "refundAmount": 50.00,
    "reason": "Customer request",
    "description": "Partial refund for order cancellation"
}'
```

### 5. Cancel Payment

```bash
curl -X POST "http://localhost:9999/v1/payments/payu/{paymentId}/cancel" \
-H "Content-Type: application/json" \
-d '{
    "reason": "Customer cancellation"
}'
```

## Response Examples

### Successful Payment Response

```json
{
  "success": true,
  "status": "successful",
  "paymentId": "payu_abc123xyz789",
  "transactionId": "txn_987654321",
  "amount": 150.75,
  "currency": "TRY",
  "message": "Payment processed successfully",
  "systemTime": "2024-01-15T10:30:00Z",
  "providerResponse": {
    "status": "SUCCESS",
    "paymentId": "payu_abc123xyz789",
    "transactionId": "txn_987654321",
    "orderId": "order_456789",
    "amount": 150.75,
    "currency": "TRY",
    "timestamp": 1705317000
  }
}
```

### 3D Secure Payment Response

```json
{
  "success": true,
  "status": "pending",
  "paymentId": "payu_def456uvw012",
  "amount": 250.0,
  "currency": "TRY",
  "redirectUrl": "https://secure.payu.tr/3dsecure?token=abc123...",
  "message": "Redirecting to 3D Secure authentication",
  "systemTime": "2024-01-15T10:35:00Z"
}
```

### Failed Payment Response

```json
{
  "success": false,
  "status": "failed",
  "amount": 150.75,
  "currency": "TRY",
  "errorCode": "INSUFFICIENT_FUNDS",
  "message": "Insufficient funds on card",
  "systemTime": "2024-01-15T10:40:00Z"
}
```

## Test Cards

### For Sandbox Environment

| Card Number      | CVV | Expiry | Expected Result    |
| ---------------- | --- | ------ | ------------------ |
| 5528790000000008 | 123 | 12/30  | Success            |
| 4059030000000009 | 456 | 06/29  | Success (3D)       |
| 4111111111111111 | 789 | 08/28  | Success            |
| 5555555555554444 | 321 | 10/27  | Declined           |
| 4000000000000002 | 654 | 04/26  | Insufficient Funds |

### Test 3D Secure Scenarios

- **OTP Code for Success**: `123456`
- **OTP Code for Failure**: `000000`

## Error Codes

| Error Code                 | Description                     | Action Required        |
| -------------------------- | ------------------------------- | ---------------------- |
| `INSUFFICIENT_FUNDS`       | Not enough balance on card      | Try different card     |
| `INVALID_CARD`             | Invalid card number or details  | Check card information |
| `EXPIRED_CARD`             | Card has expired                | Use valid card         |
| `FRAUDULENT_TRANSACTION`   | Transaction flagged as fraud    | Contact PayU support   |
| `CARD_DECLINED`            | Card declined by bank           | Try different card     |
| `SYSTEM_ERROR`             | PayU system error               | Retry later            |
| `INVALID_AMOUNT`           | Invalid transaction amount      | Check amount format    |
| `3D_AUTHENTICATION_FAILED` | 3D secure authentication failed | Retry with valid OTP   |

## Webhook Integration

### Webhook Endpoint Setup

Configure your webhook endpoint in PayU Turkey merchant panel:

```
POST https://your-domain.com/v1/callback/payu
```

### Webhook Payload Example

```json
{
  "paymentId": "payu_abc123xyz789",
  "transactionId": "txn_987654321",
  "status": "SUCCESS",
  "amount": 150.75,
  "currency": "TRY",
  "timestamp": 1705317000,
  "signature": "a1b2c3d4e5f6..."
}
```

### Webhook Validation

PayU Turkey webhooks are validated using HMAC-SHA256 signature:

```go
func validateWebhook(payload, signature, secretKey string) bool {
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(payload))
    expectedSignature := hex.EncodeToString(h.Sum(nil))
    return signature == expectedSignature
}
```

## Currency Support

| Currency      | Code | Supported |
| ------------- | ---- | --------- |
| Turkish Lira  | TRY  | ✅        |
| US Dollar     | USD  | ✅        |
| Euro          | EUR  | ✅        |
| British Pound | GBP  | ✅        |
| Swiss Franc   | CHF  | ✅        |
| Japanese Yen  | JPY  | ✅        |

## Security Features

### SHA-256 Signature Verification

All requests include HMAC-SHA256 signatures for security:

```go
signatureData := merchantId + "|" + amount + "|" + orderId + "|" + secretKey
hash := sha256.Sum256([]byte(signatureData))
signature := hex.EncodeToString(hash[:])
```

### 3D Secure Support

- **Full 3D Secure**: Complete authentication with SMS OTP
- **Half 3D Secure**: Liability shift without OTP (fallback)
- **Frictionless**: Fast authentication for low-risk transactions

### PCI DSS Compliance

- All card data encrypted in transit
- No card storage on merchant servers
- PCI DSS Level 1 compliant infrastructure

## Production Environment

### Going Live Checklist

1. **Update Environment**: Change `environment` to `production`
2. **Production Credentials**: Use live merchantId and secretKey
3. **SSL Certificate**: Ensure HTTPS for all endpoints
4. **Webhook URLs**: Update to production callback URLs
5. **Test Thoroughly**: Verify all payment flows in production

### Live Configuration

```go
payuConfig := map[string]string{
    "merchantId":   "your_live_merchant_id",
    "secretKey":    "your_live_secret_key",
    "environment":  "production",
    "gopayBaseURL": "https://your-production-domain.com",
}
```

## Rate Limits

| Operation        | Limit       | Window       |
| ---------------- | ----------- | ------------ |
| Payment Creation | 100 req/min | Per merchant |
| Status Inquiry   | 200 req/min | Per merchant |
| Refunds          | 50 req/min  | Per merchant |
| Webhooks         | N/A         | Event-driven |

## Support and Documentation

- **PayU Turkey Documentation**: https://docs.payu.tr
- **PayU Turkey Support**: support@payu.tr
- **Developer Portal**: https://developer.payu.tr
- **Status Page**: https://status.payu.tr

## Troubleshooting

### Common Issues

1. **Invalid Signature Error**

   - Verify secretKey is correct
   - Check signature calculation order
   - Ensure UTF-8 encoding

2. **3D Secure Redirect Issues**

   - Verify callbackUrl is accessible
   - Check HTTPS certificate
   - Ensure proper URL encoding

3. **Payment Failures**
   - Check test card numbers
   - Verify merchant configuration
   - Review error codes and messages

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
payuConfig["debug"] = "true"
```

This will log all API requests and responses for debugging purposes.

## Examples

### Complete Integration Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/payu"
)

func main() {
    // Create payment service
    paymentService := provider.NewPaymentService()

    // Configure PayU Turkey
    payuConfig := map[string]string{
        "merchantId":   "your_merchant_id",
        "secretKey":    "your_secret_key",
        "environment":  "sandbox",
        "gopayBaseURL": "https://yourdomain.com",
    }

    // Add PayU provider
    err := paymentService.AddProvider("payu", payuConfig)
    if err != nil {
        log.Fatal("Failed to add PayU provider:", err)
    }

    // Set as default provider
    err = paymentService.SetDefaultProvider("payu")
    if err != nil {
        log.Fatal("Failed to set default provider:", err)
    }

    // Create payment request
    paymentRequest := provider.PaymentRequest{
        Amount:   199.99,
        Currency: "TRY",
        Customer: provider.Customer{
            Name:        "Ali",
            Surname:     "Veli",
            Email:       "ali.veli@example.com",
            PhoneNumber: "+905551234567",
        },
        CardInfo: provider.CardInfo{
            CardHolderName: "Ali Veli",
            CardNumber:     "5528790000000008",
            ExpireMonth:    "12",
            ExpireYear:     "2030",
            CVV:            "123",
        },
        Use3D: true,
        CallbackURL: "https://yourdomain.com/payment/callback",
    }

    // Process payment
    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "payu", paymentRequest)
    if err != nil {
        log.Fatal("Payment failed:", err)
    }

    fmt.Printf("Payment Status: %s\n", response.Status)
    fmt.Printf("Payment ID: %s\n", response.PaymentID)

    if response.RedirectURL != "" {
        fmt.Printf("3D Secure URL: %s\n", response.RedirectURL)
    }
}
```

This completes the PayU Turkey integration documentation with comprehensive examples, test data, and production guidelines.

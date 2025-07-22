# PayTR Payment Provider

## Overview

PayTR is one of Turkey's leading payment processing platforms, providing virtual POS and payment solutions for businesses. This provider implementation supports PayTR's iFrame and Direct API integration methods.

[PayTR API Documentation](https://dev.paytr.com/)

## Features

- **iFrame Payments**: Secure payment form within iframe
- **3D Secure Support**: Enhanced security with SCA compliance
- **Payment Status Inquiry**: Real-time payment status tracking
- **Refunds**: Full and partial refund support
- **Webhook Validation**: Secure webhook handling with MD5 signatures
- **Multi-Currency**: Support for TL, USD, EUR
- **Installment Payments**: Flexible installment options
- **Test Mode**: Comprehensive testing environment

## Configuration

### Required Parameters

```go
config := map[string]string{
    "merchantId":     "your-merchant-id",
    "merchantKey":    "your-merchant-key",
    "merchantSalt":   "your-merchant-salt",
    "environment":    "sandbox", // or "production"
    "gopayBaseURL":   "https://your-gopay-instance.com",
}
```

### Environment Variables

```bash
# PayTR Configuration
PAYTR_MERCHANT_ID=your_merchant_id
PAYTR_MERCHANT_KEY=your_merchant_key
PAYTR_MERCHANT_SALT=your_merchant_salt
PAYTR_ENVIRONMENT=sandbox

# GoPay Base URL for callbacks
APP_URL=https://your-gopay-domain.com
```

## Usage Examples

### Basic Integration

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/paytr"
)

// Initialize PayTR provider
paytrConfig := map[string]string{
    "merchantId":     "your-merchant-id",
    "merchantKey":    "your-merchant-key",
    "merchantSalt":   "your-merchant-salt",
    "environment":    "sandbox",
    "gopayBaseURL":   "https://your-domain.com",
}

provider := paytr.NewProvider()
err := provider.Initialize(paytrConfig)
if err != nil {
    log.Fatal(err)
}
```

### iFrame Payment (3D Secure)

```go
// Create payment request
request := provider.PaymentRequest{
    Amount:   100.50,
    Currency: "TL",
    Customer: provider.Customer{
        Name:    "Ahmet",
        Surname: "Yılmaz",
        Email:   "ahmet@example.com",
        PhoneNumber: "+905551234567",
        IPAddress: "192.168.1.1",
    },
    Items: []provider.Item{
        {
            Name:     "Product 1",
            Price:    100.50,
            Quantity: 1,
        },
    },
    CallbackURL: "https://yourapp.com/payment-callback",
    ClientIP:    "192.168.1.1",
}

// Process 3D secure payment
response, err := provider.Create3DPayment(ctx, request)
if err != nil {
    log.Printf("Payment failed: %v", err)
    return
}

if response.Success {
    // Display iframe or redirect to payment page
    log.Printf("Payment initiated: %s", response.PaymentID)
    log.Printf("Redirect URL: %s", response.RedirectURL)

    // If HTML iframe is provided, you can embed it directly
    if response.HTML != "" {
        fmt.Printf("Embed this HTML: %s", response.HTML)
    }
}
```

### Direct Payment (Non-3D)

```go
// For direct payments (currently redirects to 3D secure in PayTR)
response, err := provider.CreatePayment(ctx, request)
if err != nil {
    log.Printf("Payment failed: %v", err)
    return
}

log.Printf("Payment response: %+v", response)
```

### Payment Status Inquiry

```go
// Check payment status
status, err := provider.GetPaymentStatus(ctx, "payment-id")
if err != nil {
    log.Printf("Status inquiry failed: %v", err)
    return
}

log.Printf("Payment Status: %s", status.Status)
log.Printf("Success: %t", status.Success)
```

### Refund Payment

```go
// Process refund
refundRequest := provider.RefundRequest{
    PaymentID:    "payment-id",
    RefundAmount: 50.25, // Partial refund
    Reason:       "Customer request",
    Description:  "Partial refund for order #12345",
}

refundResponse, err := provider.RefundPayment(ctx, refundRequest)
if err != nil {
    log.Printf("Refund failed: %v", err)
    return
}

if refundResponse.Success {
    log.Printf("Refund successful: %s", refundResponse.RefundID)
}
```

## API Endpoints

| Method | Endpoint                         | Description        |
| ------ | -------------------------------- | ------------------ |
| `POST` | `/v1/payments/paytr`             | Process payment    |
| `GET`  | `/v1/payments/paytr/{paymentID}` | Get payment status |
| `POST` | `/v1/payments/paytr/refund`      | Process refund     |
| `POST` | `/v1/callback/paytr`             | Payment callback   |
| `POST` | `/v1/webhooks/paytr`             | Webhook endpoint   |

## Test Environment

### Test Cards

PayTR provides specific test card numbers for testing:

#### Successful Test Cards

| Card Number        | Description                       |
| ------------------ | --------------------------------- |
| `4355084355084358` | Visa test card (successful)       |
| `5406675406675403` | MasterCard test card (successful) |
| `9792030394440796` | Troy test card (successful)       |

#### Card Details for Testing

- **Expiry Date**: Any future date (e.g., 12/25)
- **CVV**: Any 3-digit number (e.g., 000)
- **Card Holder Name**: Any name (e.g., "PAYTR TEST")

### Test Configuration

```go
testConfig := map[string]string{
    "merchantId":     "your-test-merchant-id",
    "merchantKey":    "your-test-merchant-key",
    "merchantSalt":   "your-test-merchant-salt",
    "environment":    "sandbox",
    "gopayBaseURL":   "https://test.yourapp.com",
}
```

## Webhook Integration

PayTR sends webhook notifications for payment status changes.

### Webhook Handler

```go
func handlePayTRWebhook(w http.ResponseWriter, r *http.Request) {
    // Parse webhook data
    var webhookData map[string]string
    err := json.NewDecoder(r.Body).Decode(&webhookData)
    if err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Get headers
    headers := make(map[string]string)
    for key, values := range r.Header {
        if len(values) > 0 {
            headers[key] = values[0]
        }
    }

    // Validate webhook
    valid, paymentInfo, err := provider.ValidateWebhook(ctx, webhookData, headers)
    if err != nil {
        http.Error(w, "Webhook validation failed", http.StatusBadRequest)
        return
    }

    if valid {
        log.Printf("Payment %s status: %s", paymentInfo["paymentId"], paymentInfo["status"])

        // Process payment status change
        switch paymentInfo["status"] {
        case "success":
            // Payment successful
            processSuccessfulPayment(paymentInfo["paymentId"])
        case "failed":
            // Payment failed
            processFailedPayment(paymentInfo["paymentId"], paymentInfo["errorMessage"])
        }

        w.WriteHeader(http.StatusOK)
    } else {
        http.Error(w, "Invalid webhook", http.StatusBadRequest)
    }
}
```

### Webhook Signature Verification

PayTR uses MD5 signatures for webhook validation:

```
Hash = MD5(merchant_oid + merchant_salt + status + total_amount)
```

## Supported Currencies

| Currency     | Code | Name                 |
| ------------ | ---- | -------------------- |
| Turkish Lira | TL   | Turkish Lira         |
| US Dollar    | USD  | United States Dollar |
| Euro         | EUR  | Euro                 |

## Response Codes

### Payment Status Mapping

| PayTR Status | GoPay Status | Description                    |
| ------------ | ------------ | ------------------------------ |
| `success`    | `successful` | Payment completed successfully |
| `failed`     | `failed`     | Payment failed                 |
| `waiting`    | `pending`    | Payment waiting for completion |
| `pending`    | `pending`    | Payment pending                |

### Common Error Codes

| Error Code            | Description        | Action                       |
| --------------------- | ------------------ | ---------------------------- |
| `YETERSIZ_BAKIYE`     | Insufficient funds | Contact card issuer          |
| `GECERSIZ_KART`       | Invalid card       | Check card details           |
| `SURESI_GECMIS_KART`  | Expired card       | Use valid expiry date        |
| `SAHTEKARLIK_SUPTESI` | Fraud suspicion    | Contact PayTR support        |
| `KART_REDDEDILDI`     | Card declined      | Try different payment method |
| `SISTEM_HATASI`       | System error       | Retry payment                |

## Security Considerations

### Authentication

PayTR uses MD5 hash-based authentication for all API calls:

```
Token = MD5(merchant_id + user_ip + merchant_oid + email + payment_amount + user_basket + no_installment + max_installment + currency + test_mode + merchant_salt)
```

### Best Practices

1. **Always validate webhooks** before processing payment status changes
2. **Use HTTPS** for all production environments
3. **Store credentials securely** using environment variables
4. **Implement retry logic** for network failures
5. **Log all transactions** for audit purposes
6. **Monitor failed payments** and implement alerts

### Hash Calculation

PayTR requires different hash calculations for different operations:

#### Token Hash (iFrame)

```
merchant_id + user_ip + merchant_oid + email + payment_amount + user_basket + no_installment + max_installment + currency + test_mode + merchant_salt
```

#### Status Query Hash

```
merchant_id + merchant_oid + merchant_salt
```

#### Refund Hash

```
merchant_id + merchant_oid + return_amount + merchant_salt
```

#### Webhook Hash

```
merchant_oid + merchant_salt + status + total_amount
```

## Integration Flow

### 3D Secure Payment Flow

1. **Initialize Payment**: Call `Create3DPayment` with payment details
2. **Get iFrame Token**: Receive iframe URL or HTML for payment form
3. **Display Payment Form**: Show PayTR payment iframe to customer
4. **Customer Payment**: Customer enters card details and completes 3D secure
5. **Receive Callback**: PayTR redirects to your callback URL
6. **Webhook Notification**: PayTR sends webhook with final payment status
7. **Verify Payment**: Use `GetPaymentStatus` to confirm payment status

### Error Handling

```go
response, err := provider.Create3DPayment(ctx, request)
if err != nil {
    // Handle API errors
    log.Printf("API Error: %v", err)
    return
}

if !response.Success {
    // Handle payment errors
    log.Printf("Payment failed: %s (Code: %s)", response.Message, response.ErrorCode)

    switch response.ErrorCode {
    case "YETERSIZ_BAKIYE":
        // Handle insufficient funds
    case "GECERSIZ_KART":
        // Handle invalid card
    default:
        // Handle other errors
    }
}
```

## Troubleshooting

### Common Issues

#### Hash Mismatch

- Verify all parameters are included in hash calculation
- Check parameter order matches PayTR documentation
- Ensure no extra spaces or encoding issues

#### Connection Errors

- Verify PayTR endpoints are accessible
- Check firewall settings
- Ensure SSL/TLS configuration is correct

#### Invalid Credentials

- Verify merchant ID, key, and salt are correct
- Check if credentials are for correct environment (test/production)
- Contact PayTR support if credentials are not working

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
// Add detailed logging
log.Printf("Payment request: %+v", request)
log.Printf("Payment response: %+v", response)
```

## Production Deployment

### Environment Configuration

```bash
# Production settings
PAYTR_MERCHANT_ID=prod_merchant_id
PAYTR_MERCHANT_KEY=prod_merchant_key
PAYTR_MERCHANT_SALT=prod_merchant_salt
PAYTR_ENVIRONMENT=production

# SSL/TLS Configuration
APP_URL=https://secure.yourapp.com
```

### Monitoring

- Monitor payment success rates
- Set up alerts for high failure rates
- Track response times and timeouts
- Monitor webhook delivery success

## Support

- **PayTR Documentation**: https://dev.paytr.com
- **PayTR Support**: Contact through PayTR merchant panel
- **Technical Issues**: Check PayTR developer documentation
- **Integration Help**: Refer to PayTR API documentation

For PayTR-specific issues, contact PayTR support directly through their merchant panel or support channels.

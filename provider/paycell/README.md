# GoPay Paycell Integration

This package provides a complete integration between GoPay and Paycell (TPay), Turkcell's payment processing solution. Paycell offers secure payment processing for Turkish market with comprehensive tPay REST API support.

## API URLs

- **Test Environment**: `https://tpay-test.turkcell.com.tr`
- **Production Environment**: `https://tpay.turkcell.com.tr`
- **3D Secure Management (Test)**: `https://omccstb.turkcell.com.tr`
- **3D Secure Management (Prod)**: `https://secure.paycell.com.tr`

## Table of Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [API Methods](#api-methods)
  - [Regular Payment](#regular-payment)
  - [3D Secure Payment](#3d-secure-payment)
  - [Payment Status](#payment-status)
  - [Cancel Payment](#cancel-payment)
  - [Refund Payment](#refund-payment)
- [Test Cards](#test-cards)
- [Test Scenarios](#test-scenarios)
- [Error Codes](#error-codes)
- [Webhook Integration](#webhook-integration)
- [Security Considerations](#security-considerations)
- [API Reference](#api-reference)
- [Troubleshooting](#troubleshooting)

## Installation

```bash
go get github.com/mstgnz/gopay
```

## Configuration

Add the following environment variables to your `.env` file:

```bash
# Paycell Configuration
PAYCELL_USERNAME=your_paycell_username
PAYCELL_PASSWORD=your_paycell_password
PAYCELL_MERCHANT_ID=your_merchant_id
PAYCELL_TERMINAL_ID=your_terminal_id
PAYCELL_ENVIRONMENT=sandbox  # or 'production'

# GoPay Base URL (for 3D Secure callbacks)
APP_URL=https://your-gopay-domain.com
```

## Quick Start

### Using GoPay Service

```go
package main

import (
    "context"
    "log"

    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/paycell" // Import to register provider
)

func main() {
    // Create payment service
    paymentService := provider.NewPaymentService()

    // Configure Paycell
    paycellConfig := map[string]string{
        "username":     "your_username",
        "password":     "your_password",
        "merchantId":   "your_merchant_id",
        "terminalId":   "your_terminal_id",
        "environment":  "sandbox",
        "gopayBaseURL": "https://your-gopay-domain.com",
    }

    err := paymentService.AddProvider("paycell", paycellConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Create payment request
    request := provider.PaymentRequest{
        Amount:   100.50,
        Currency: "TRY",
        Customer: provider.Customer{
            Name:        "Ahmet",
            Surname:     "Yılmaz",
            Email:       "ahmet@example.com",
            PhoneNumber: "+90555123456",
            Address: provider.Address{
                Country: "Turkey",
                City:    "Istanbul",
                Address: "Ataşehir",
                ZipCode: "34750",
            },
        },
        CardInfo: provider.CardInfo{
            CardNumber:     "5528790000000008",
            ExpireMonth:    "12",
            ExpireYear:     "2030",
            CVV:            "123",
            CardHolderName: "AHMET YILMAZ",
        },
        Description: "Test payment",
    }

    // Process payment
    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "paycell", request)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Payment successful: %v", response.Success)
    log.Printf("Payment ID: %s", response.PaymentID)
}
```

### Direct Provider Usage

```go
package main

import (
    "context"
    "log"

    "github.com/mstgnz/gopay/provider/paycell"
)

func main() {
    // Create provider instance
    provider := paycell.NewProvider()

    // Initialize with configuration
    config := map[string]string{
        "username":     "your_username",
        "password":     "your_password",
        "merchantId":   "your_merchant_id",
        "terminalId":   "your_terminal_id",
        "environment":  "sandbox",
        "gopayBaseURL": "https://your-gopay-domain.com",
    }

    err := provider.Initialize(config)
    if err != nil {
        log.Fatal(err)
    }

    // Use provider methods directly...
}
```

## API Methods

### Regular Payment

Process a standard payment without 3D Secure authentication:

```go
request := provider.PaymentRequest{
    Amount:   100.50,
    Currency: "TRY",
    Customer: provider.Customer{
        Name:    "Ahmet",
        Surname: "Yılmaz",
        Email:   "ahmet@example.com",
    },
    CardInfo: provider.CardInfo{
        CardNumber:  "5528790000000008",
        ExpireMonth: "12",
        ExpireYear:  "2030",
        CVV:         "123",
    },
    Description: "Regular payment",
}

response, err := provider.CreatePayment(ctx, request)
if err != nil {
    log.Fatal(err)
}

if response.Success {
    log.Printf("Payment successful: %s", response.PaymentID)
} else {
    log.Printf("Payment failed: %s", response.Message)
}
```

### 3D Secure Payment

Process a payment with 3D Secure authentication:

```go
request := provider.PaymentRequest{
    Amount:      100.50,
    Currency:    "TRY",
    CallbackURL: "https://yourapp.com/payment-callback",
    Customer: provider.Customer{
        Name:    "Ahmet",
        Surname: "Yılmaz",
        Email:   "ahmet@example.com",
    },
    CardInfo: provider.CardInfo{
        CardNumber:  "5528790000000057", // 3D test card
        ExpireMonth: "12",
        ExpireYear:  "2030",
        CVV:         "123",
    },
    Description: "3D Secure payment",
}

response, err := provider.Create3DPayment(ctx, request)
if err != nil {
    log.Fatal(err)
}

if response.Status == provider.StatusPending {
    // Redirect user to 3D authentication
    log.Printf("Redirect URL: %s", response.RedirectURL)
    log.Printf("HTML Form: %s", response.HTML)
}
```

### Payment Status

Check the status of a payment:

```go
paymentID := "pay_1234567890"
response, err := provider.GetPaymentStatus(ctx, paymentID)
if err != nil {
    log.Fatal(err)
}

log.Printf("Payment Status: %s", response.Status)
log.Printf("Amount: %.2f %s", response.Amount, response.Currency)
```

### Cancel Payment

Cancel a pending or authorized payment:

```go
paymentID := "pay_1234567890"
reason := "Customer cancellation"

response, err := provider.CancelPayment(ctx, paymentID, reason)
if err != nil {
    log.Fatal(err)
}

if response.Success {
    log.Printf("Payment cancelled successfully")
} else {
    log.Printf("Cancellation failed: %s", response.Message)
}
```

### Refund Payment

Issue a full or partial refund:

```go
// Full refund
refundRequest := provider.RefundRequest{
    PaymentID:      "pay_1234567890",
    Reason:         "Customer return",
    Description:    "Full refund for order #12345",
    ConversationID: "conv_67890",
}

// Partial refund
refundRequest := provider.RefundRequest{
    PaymentID:      "pay_1234567890",
    RefundAmount:   50.25, // Partial amount
    Currency:       "TRY",
    Reason:         "Partial return",
    Description:    "Partial refund for damaged item",
    ConversationID: "conv_67890",
}

response, err := provider.RefundPayment(ctx, refundRequest)
if err != nil {
    log.Fatal(err)
}

if response.Success {
    log.Printf("Refund successful: %s", response.RefundID)
    log.Printf("Refunded amount: %.2f", response.RefundAmount)
} else {
    log.Printf("Refund failed: %s", response.Message)
}
```

## Test Cards

### Success Test Cards

| Card Number        | Description     | Expected Result |
| ------------------ | --------------- | --------------- |
| `5528790000000008` | Successful card | SUCCESS         |
| `4111111111111111` | Visa test card  | SUCCESS         |
| `4000000000000002` | Visa test card  | SUCCESS         |

### Error Test Cards

| Card Number        | Error Type         | Expected Result    |
| ------------------ | ------------------ | ------------------ |
| `5528790000000016` | Insufficient funds | INSUFFICIENT_FUNDS |
| `5528790000000024` | Invalid card       | INVALID_CARD       |
| `5528790000000032` | Expired card       | EXPIRED_CARD       |
| `5528790000000040` | Declined by bank   | CARD_DECLINED      |

### 3D Secure Test Cards

| Card Number        | Description       | Expected Result         |
| ------------------ | ----------------- | ----------------------- |
| `5528790000000057` | 3D redirect card  | Redirects to 3D auth    |
| `4000000000003220` | 3D challenge card | Requires authentication |

## Test Scenarios

### Amount-Based Test Scenarios

| Amount  | Description          | Expected Behavior         |
| ------- | -------------------- | ------------------------- |
| 999.99  | Timeout simulation   | Request timeout after 35s |
| 0.01    | Minimum amount       | SUCCESS                   |
| 1000.00 | Standard test amount | SUCCESS                   |
| 9999.99 | Maximum test amount  | SUCCESS                   |

### Merchant Credentials

For testing, use these sandbox credentials:

```bash
PAYCELL_USERNAME=test_merchant
PAYCELL_PASSWORD=test_password123
PAYCELL_MERCHANT_ID=TEST_MERCHANT_001
PAYCELL_TERMINAL_ID=TEST_TERMINAL_001
PAYCELL_ENVIRONMENT=sandbox
```

## Error Codes

### Paycell Error Codes

| Error Code               | Description               | Action Required          |
| ------------------------ | ------------------------- | ------------------------ |
| `INSUFFICIENT_FUNDS`     | Insufficient card balance | Use different card       |
| `INVALID_CARD`           | Invalid card number/data  | Check card information   |
| `EXPIRED_CARD`           | Card has expired          | Use valid card           |
| `CARD_DECLINED`          | Bank declined transaction | Contact card issuer      |
| `FRAUDULENT_TRANSACTION` | Suspected fraud           | Contact Paycell support  |
| `SYSTEM_ERROR`           | Internal system error     | Retry or contact support |

### HTTP Status Codes

| Status Code | Description           | Meaning              |
| ----------- | --------------------- | -------------------- |
| 200         | OK                    | Request successful   |
| 400         | Bad Request           | Invalid request data |
| 401         | Unauthorized          | Invalid credentials  |
| 403         | Forbidden             | Access denied        |
| 500         | Internal Server Error | Paycell system error |

## Webhook Integration

Paycell sends webhook notifications for payment status changes:

### Webhook Handler

```go
func handlePaycellWebhook(w http.ResponseWriter, r *http.Request) {
    // Parse webhook data
    var webhookData map[string]string
    json.NewDecoder(r.Body).Decode(&webhookData)

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
        w.WriteHeader(http.StatusOK)
    } else {
        http.Error(w, "Invalid webhook", http.StatusBadRequest)
    }
}
```

### Webhook Signature Verification

Paycell uses MD5 signatures for webhook validation:

```
Signature = MD5(webhookData)
Header: X-Paycell-Signature
```

## Security Considerations

### Authentication

Paycell uses signature-based authentication:

```
Signature = MD5(METHOD|PATH|BODY|TIMESTAMP|PASSWORD)
Headers:
- X-Paycell-Username: merchant_username
- X-Paycell-Timestamp: unix_timestamp
- X-Paycell-Signature: calculated_signature
```

### Best Practices

1. **Always validate webhooks** before processing payment status changes
2. **Use HTTPS** for all production environments
3. **Store credentials securely** using environment variables
4. **Implement retry logic** for network failures
5. **Log all transactions** for audit purposes
6. **Monitor failed payments** and implement alerts

### PCI Compliance

- **Never store** card numbers, CVV, or expiry dates
- **Use tokenization** for recurring payments
- **Implement proper encryption** for sensitive data transmission
- **Follow PCI DSS guidelines** for payment processing

## API Reference

### Provision Services Endpoints

| Method | Endpoint                                                                | Description                |
| ------ | ----------------------------------------------------------------------- | -------------------------- |
| POST   | `/tpay/provision/services/restful/getCardToken/provision/`              | Create regular payment     |
| POST   | `/tpay/provision/services/restful/getCardToken/getThreeDSession/`       | Create 3D secure session   |
| POST   | `/tpay/provision/services/restful/getCardToken/inquire/`                | Get payment status         |
| POST   | `/tpay/provision/services/restful/getCardToken/reverse/`                | Cancel/Reverse payment     |
| POST   | `/tpay/provision/services/restful/getCardToken/refund/`                 | Create refund              |
| POST   | `/tpay/provision/services/restful/getCardToken/getThreeDSessionResult/` | Complete 3D secure payment |

### Payment Management Endpoints (3D Secure)

| Method | Endpoint                                     | Description              |
| ------ | -------------------------------------------- | ------------------------ |
| POST   | `/paymentmanagement/rest/getCardTokenSecure` | Get secure card token    |
| POST   | `/paymentmanagement/rest/threeDSecure`       | 3D Secure authentication |

### Request Headers

```
Content-Type: application/json
X-Paycell-Username: merchant_username
X-Paycell-Timestamp: unix_timestamp
X-Paycell-Signature: calculated_signature
```

### Request Format (Example)

```json
{
  "orderId": "gopay_12345",
  "merchantId": "your_merchant_id",
  "terminalId": "your_terminal_id",
  "amount": "100.50",
  "currency": "TRY",
  "description": "Payment description",
  "timestamp": 1699876543,
  "cardNumber": "5528790000000008",
  "expireMonth": "12",
  "expireYear": "2030",
  "cvv": "123",
  "cardHolderName": "AHMET YILMAZ",
  "customerName": "Ahmet Yılmaz",
  "customerEmail": "ahmet@example.com",
  "customerPhone": "+90555123456",
  "successUrl": "https://your-app.com/success",
  "failureUrl": "https://your-app.com/failure",
  "secure3d": "true"
}
```

### Response Format

#### Regular Payment Response

```json
{
  "success": true,
  "status": "SUCCESS",
  "orderId": "gopay_12345",
  "paymentId": "pay_1234567890",
  "transactionId": "txn_0987654321",
  "amount": "100.50",
  "currency": "TRY",
  "message": "Payment successful",
  "responseCode": "00",
  "responseMessage": "Success",
  "provisionResponse": "approved"
}
```

#### 3D Secure Payment Response

```json
{
  "success": true,
  "status": "PENDING",
  "orderId": "gopay_12345",
  "threeDSessionId": "3ds_session_123",
  "threeDUrl": "https://omccstb.turkcell.com.tr/paymentmanagement/rest/threeDSecure",
  "amount": "100.50",
  "currency": "TRY",
  "message": "3D Secure authentication required",
  "responseCode": "3D",
  "responseMessage": "3D Secure Required"
}
```

## Troubleshooting

### Common Issues

#### Connection Errors

```
Error: dial tcp: lookup test.paycell.com.tr: no such host
```

**Solution:** Check network connectivity and DNS resolution.

#### Authentication Errors

```
Error: paycell: invalid signature
```

**Solution:** Verify credentials and signature calculation.

#### Invalid Amount Errors

```
Error: paycell: invalid payment request: amount must be greater than 0
```

**Solution:** Ensure amount is positive and properly formatted.

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
import "log"

// Enable detailed logging
log.SetFlags(log.LstdFlags | log.Lshortfile)

// Log request/response details
response, err := provider.CreatePayment(ctx, request)
log.Printf("Request: %+v", request)
log.Printf("Response: %+v", response)
log.Printf("Error: %v", err)
```

### Testing Connection

Test your Paycell configuration:

```go
func testPaycellConnection() {
    provider := paycell.NewProvider()
    config := map[string]string{
        "username":    "test_username",
        "password":    "test_password",
        "merchantId":  "test_merchant",
        "terminalId":  "test_terminal",
        "environment": "sandbox",
    }

    err := provider.Initialize(config)
    if err != nil {
        log.Printf("Configuration error: %v", err)
        return
    }

    // Test with minimal payment
    request := provider.PaymentRequest{
        Amount:   1.00,
        Currency: "TRY",
        Customer: provider.Customer{
            Name:    "Test",
            Surname: "User",
            Email:   "test@example.com",
        },
        CardInfo: provider.CardInfo{
            CardNumber:  "5528790000000008",
            ExpireMonth: "12",
            ExpireYear:  "2030",
            CVV:         "123",
        },
    }

    response, err := provider.CreatePayment(context.Background(), request)
    if err != nil {
        log.Printf("Connection test failed: %v", err)
        return
    }

    log.Printf("Connection test successful: %v", response.Success)
}
```

### Support

For technical support:

- **GoPay Issues:** Create an issue on the GitHub repository
- **Paycell Issues:** Contact Paycell technical support
- **Integration Help:** Check the documentation or create a GitHub discussion

### Environment URLs

| Environment            | URL                               |
| ---------------------- | --------------------------------- |
| Sandbox (tPay)         | https://tpay-test.turkcell.com.tr |
| Production (tPay)      | https://tpay.turkcell.com.tr      |
| Sandbox (3D Secure)    | https://omccstb.turkcell.com.tr   |
| Production (3D Secure) | https://secure.paycell.com.tr     |

## Contributing

Contributions are welcome! Please read the contributing guidelines and submit pull requests for any improvements.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

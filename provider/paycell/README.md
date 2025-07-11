# GoPay Paycell Integration

This package provides a complete integration between GoPay and Paycell (TPay), Turkcell's payment processing solution. Paycell offers secure payment processing for Turkish market with comprehensive tPay REST API support.

[Paycell API Documentation](https://apiportal.paycell.com.tr/paycellapi)

## API URLs

- **Test Environment**: `https://tpay-test.turkcell.com.tr`
- **Production Environment**: `https://tpay.turkcell.com.tr`
- **3D Secure Management (Test)**: `https://omccstb.turkcell.com.tr`
- **3D Secure Management (Prod)**: `https://epayment.turkcell.com.tr`

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
- [Security](#security)
- [API Reference](#api-reference)
- [Troubleshooting](#troubleshooting)

## Installation

```bash
go get github.com/mstgnz/gopay
```

## Configuration

Required configuration for test environment:

```bash
# Paycell Test Configuration
PAYCELL_USERNAME=PAYCELLTEST
PAYCELL_PASSWORD=PaycellTestPassword
PAYCELL_MERCHANT_ID=9998
PAYCELL_TERMINAL_ID=17
PAYCELL_SECURE_CODE=PAYCELL12345
PAYCELL_ENVIRONMENT=sandbox

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
        "username":     "PAYCELLTEST",
        "password":     "PaycellTestPassword",
        "merchantId":   "9998",
        "terminalId":   "17",
        "secureCode":   "PAYCELL12345",
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
            Name:        "Test",
            Surname:     "Customer",
            Email:       "test@example.com",
            PhoneNumber: "5551234567", // 10 digits without country code
            Address: provider.Address{
                Country: "Turkey",
                City:    "Istanbul",
                Address: "Test Address",
                ZipCode: "34000",
            },
        },
        CardInfo: provider.CardInfo{
            CardNumber:     "5528790000000008",
            ExpireMonth:    "12",
            ExpireYear:     "26",
            CVV:            "001",
            CardHolderName: "TEST CUSTOMER",
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
    log.Printf("Transaction ID: %s", response.TransactionID)
    log.Printf("Message: %s", response.Message)
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
        "username":     "PAYCELLTEST",
        "password":     "PaycellTestPassword",
        "merchantId":   "9998",
        "terminalId":   "17",
        "secureCode":   "PAYCELL12345",
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
        Name:        "Test",
        Surname:     "Customer",
        Email:       "test@example.com",
        PhoneNumber: "5551234567", // 10 digits, no country code
    },
    CardInfo: provider.CardInfo{
        CardNumber:  "5528790000000008",
        ExpireMonth: "12",
        ExpireYear:  "26",
        CVV:         "001",
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
    log.Printf("Error code: %s", response.ErrorCode)
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
        Name:        "Test",
        Surname:     "User",
        Email:       "test@example.com",
        PhoneNumber: "5551234567",
    },
    CardInfo: provider.CardInfo{
        CardNumber:  "4355084355084358", // 3D test card
        ExpireMonth: "12",
        ExpireYear:  "26",
        CVV:         "000",
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
paymentID := "17516132740638762000"
response, err := provider.GetPaymentStatus(ctx, paymentID)
if err != nil {
    log.Fatal(err)
}

log.Printf("Payment Status: %s", response.Status)
log.Printf("Amount: %.2f %s", response.Amount, response.Currency)
log.Printf("Message: %s", response.Message)
```

### Cancel Payment

Cancel a pending or authorized payment:

```go
paymentID := "17516132740638762000"
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
    PaymentID:      "17516132740638762000",
    RefundAmount:   100.50, // Full amount
    Currency:       "TRY",
    Reason:         "Customer return",
    Description:    "Full refund for order #12345",
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

| Card Number        | Description          | Expected Result |
| ------------------ | -------------------- | --------------- |
| `5528790000000008` | Successful test card | SUCCESS         |
| `4111111111111111` | Visa test card       | SUCCESS         |

### 3D Secure Test Cards

| Card Number        | Description      | CVV | Expected Result         |
| ------------------ | ---------------- | --- | ----------------------- |
| `4355084355084358` | 3D test card     | 000 | Redirects to 3D auth    |
| `5528790000000057` | 3D redirect card | 123 | Requires authentication |

### Test Card Details

```go
// For successful payment
CardInfo: provider.CardInfo{
    CardNumber:  "5528790000000008",
    ExpireMonth: "12",
    ExpireYear:  "26",
    CVV:         "001",
}

// For 3D Secure test
CardInfo: provider.CardInfo{
    CardNumber:  "4355084355084358",
    ExpireMonth: "12",
    ExpireYear:  "26",
    CVV:         "000",
}
```

## Test Scenarios

### Amount-Based Test Scenarios

| Amount  | Description          | Expected Behavior |
| ------- | -------------------- | ----------------- |
| 1.00    | Minimum test amount  | SUCCESS           |
| 100.50  | Standard test amount | SUCCESS           |
| 1000.00 | High test amount     | SUCCESS           |

### Test Customer Information

```go
Customer: provider.Customer{
    Name:        "Test",
    Surname:     "Customer",
    Email:       "test@example.com",
    PhoneNumber: "5551234567", // 10 digits, no leading zero
    Address: provider.Address{
        Country: "Turkey",
        City:    "Istanbul",
        Address: "Test Address",
        ZipCode: "34000",
    },
}
```

## Error Codes

### Paycell Error Codes

| Error Code | Description          | Solution                  |
| ---------- | -------------------- | ------------------------- |
| `0`        | Operation successful | -                         |
| `4000`     | Bank error           | Try different card        |
| `1`        | General error        | Check transaction details |

### HTTP Status Codes

| Status Code | Description           | Meaning              |
| ----------- | --------------------- | -------------------- |
| 200         | OK                    | Request successful   |
| 400         | Bad Request           | Invalid request data |
| 401         | Unauthorized          | Invalid credentials  |
| 500         | Internal Server Error | Paycell system error |

## Security

### Hash Generation

Paycell uses SHA-256 based hash validation:

```go
// Hash generation process (automatic)
// 1. SecurityData = hash(applicationPwd + applicationName)
// 2. HashData = hash(applicationName + transactionId + transactionDateTime + secureCode + securityData)
```

### Security Best Practices

1. **Use HTTPS** for all production environments
2. **Store credentials securely** using environment variables
3. **Implement retry logic** for network failures
4. **Log all transactions** for audit purposes
5. **Monitor failed payments** and implement alerts

### PCI Compliance

- **Never store** card numbers, CVV, or expiry dates
- **Use tokenization** for recurring payments
- **Implement proper encryption** for sensitive data transmission
- **Follow PCI DSS guidelines** for payment processing

## API Reference

### Provision Services Endpoints

| Method | Endpoint                                                          | Description              |
| ------ | ----------------------------------------------------------------- | ------------------------ |
| POST   | `/tpay/provision/services/restful/getCardToken/provision/`        | Create regular payment   |
| POST   | `/tpay/provision/services/restful/getCardToken/getThreeDSession/` | Create 3D secure session |
| POST   | `/tpay/provision/services/restful/getCardToken/inquire/`          | Get payment status       |
| POST   | `/tpay/provision/services/restful/getCardToken/reverse/`          | Cancel/Reverse payment   |
| POST   | `/tpay/provision/services/restful/getCardToken/refund/`           | Create refund            |

### Payment Management Endpoints (3D Secure)

| Method | Endpoint                                     | Description              |
| ------ | -------------------------------------------- | ------------------------ |
| POST   | `/paymentmanagement/rest/getCardTokenSecure` | Get secure card token    |
| POST   | `/paymentmanagement/rest/threeDSecure`       | 3D Secure authentication |

### Request Format

```json
{
  "requestHeader": {
    "applicationName": "PAYCELLTEST",
    "applicationPwd": "PaycellTestPassword",
    "clientIPAddress": "127.0.0.1",
    "transactionDateTime": "20250704101456464",
    "transactionId": "66620250704101456464"
  },
  "cardToken": "4f9f4204-a23d-4825-947e-48dfebf1288a",
  "merchantCode": "9998",
  "msisdn": "5551234567",
  "referenceNumber": "REF_1733312096464823000",
  "amount": "10050",
  "paymentType": "SALE",
  "eulaId": "17"
}
```

### Response Format

#### Regular Payment Response

```json
{
  "responseHeader": {
    "transactionId": "17516132740638762000",
    "responseDateTime": "20250704101433406",
    "responseCode": "4000",
    "responseDescription": "Bank error"
  },
  "amount": "100",
  "currency": "TRY"
}
```

## Troubleshooting

### Common Issues

#### Connection Errors

```
Error: failed to send request: dial tcp: lookup tpay-test.turkcell.com.tr: no such host
```

**Solution:** Check network connectivity and DNS resolution.

#### Authentication Errors

```
Error: card token error: 1 - Authentication failed
```

**Solution:** Verify test credentials and hash calculation.

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
        "username":    "PAYCELLTEST",
        "password":    "PaycellTestPassword",
        "merchantId":  "9998",
        "terminalId":  "17",
        "secureCode":  "PAYCELL12345",
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
            Name:        "Test",
            Surname:     "Customer",
            Email:       "test@example.com",
            PhoneNumber: "5551234567",
        },
        CardInfo: provider.CardInfo{
            CardNumber:  "5528790000000008",
            ExpireMonth: "12",
            ExpireYear:  "26",
            CVV:         "001",
        },
    }

    response, err := provider.CreatePayment(context.Background(), request)
    if err != nil {
        log.Printf("Connection test failed: %v", err)
        return
    }

    log.Printf("Connection test successful - Transaction ID: %s", response.TransactionID)
}
```

### Running Tests

```bash
# Run all tests
go test ./provider/paycell/ -v

# Run only integration tests
go test ./provider/paycell/ -v -run "RealAPI"

# Run specific test
go test ./provider/paycell/ -v -run "TestPaycellProvider_RealAPI_CreatePayment"
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
| Production (3D Secure) | https://epayment.turkcell.com.tr  |

---

**Note:** This documentation is based on real test results and current API implementation. Integration tests are successfully working and ready for production use.

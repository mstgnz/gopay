# GoPay Papara Provider

This document explains how to use the Papara payment provider within GoPay.

## Features

- ✅ **Standard Payment**: Direct card payment
- ✅ **3D Secure Payment**: Secure payment with 3D authentication
- ✅ **Payment Status Query**: Check payment status
- ✅ **Refund Operations**: Full and partial refunds
- ✅ **Cancel Operations**: Payment cancellation (as refund)
- ✅ **Webhook Validation**: Secure webhook notifications
- ✅ **Test and Live Environment**: Sandbox and production support

## Configuration

### Required Configuration

The following configuration parameters are required for Papara integration:

```bash
# In .env file
PAPARA_API_KEY=your_papara_api_key_here
PAPARA_ENVIRONMENT=sandbox  # or production
APP_URL=https://your-domain.com  # For GoPay callback URLs
```

### Configuration Parameters

| Parameter      | Required | Description                                  |
| -------------- | -------- | -------------------------------------------- |
| `apiKey`       | ✅       | API key obtained from Papara merchant panel  |
| `environment`  | ❌       | `sandbox` or `production` (default: sandbox) |
| `gopayBaseURL` | ❌       | GoPay's own base URL (for callbacks)         |

## API Endpoints

### Test Environment (Sandbox)

- **Base URL**: `https://merchant.test.papara.com`

### Live Environment (Production)

- **Base URL**: `https://merchant.papara.com`

### Used Endpoints

- `POST /api/v1/payments` - Create payment
- `GET /api/v1/payments/{paymentId}` - Query payment status
- `POST /api/v1/refunds` - Process refund

## Usage Examples

### 1. Simple Payment

```bash
curl -X POST http://localhost:9999/v1/payments/papara \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "referenceId": "order-123",
    "description": "Test payment",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john@example.com"
    }
  }'
```

### 2. 3D Secure Payment

```bash
curl -X POST http://localhost:9999/v1/payments/papara \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 250.00,
    "currency": "TRY",
    "use3D": true,
    "callbackUrl": "https://yourapp.com/payment-callback",
    "referenceId": "order-456",
    "description": "3D Secure payment",
    "customer": {
      "name": "Jane",
      "surname": "Smith",
      "email": "jane@example.com"
    }
  }'
```

### 3. Payment Status Query

```bash
curl -X GET http://localhost:9999/v1/payments/papara/payment_id_here \
  -H "Authorization: Bearer your_api_key"
```

### 4. Refund Operation

```bash
curl -X POST http://localhost:9999/v1/payments/papara/refund \
  -H "Authorization: Bearer your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "payment_id_here",
    "refundAmount": 50.00,
    "reason": "Customer request",
    "description": "Partial refund"
  }'
```

### 5. Payment Cancellation

```bash
curl -X DELETE http://localhost:9999/v1/payments/papara/payment_id_here \
  -H "Authorization: Bearer your_api_key"
```

## Usage with Go Code

### Library Usage

```go
package main

import (
    "context"
    "log"

    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/papara"
)

func main() {
    // Create payment service
    paymentService := provider.NewPaymentService()

    // Add Papara provider
    paparaConfig := map[string]string{
        "apiKey":      "your-papara-api-key",
        "environment": "sandbox",
    }
    paymentService.AddProvider("papara", paparaConfig)

    // Create payment request
    paymentRequest := provider.PaymentRequest{
        Amount:      100.50,
        Currency:    "TRY",
        ReferenceID: "order-123",
        Description: "Test payment",
        Customer: provider.Customer{
            Name:    "John",
            Surname: "Doe",
            Email:   "john@example.com",
        },
    }

    // Process payment
    ctx := context.Background()
    response, err := paymentService.CreatePayment(ctx, "papara", paymentRequest)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Payment ID: %s", response.PaymentID)
    log.Printf("Status: %s", response.Status)
}
```

### 3D Secure Payment Example

```go
// 3D Secure payment request
paymentRequest := provider.PaymentRequest{
    Amount:      250.00,
    Currency:    "TRY",
    Use3D:       true,
    CallbackURL: "https://yourapp.com/payment-callback",
    ReferenceID: "order-456",
    Description: "3D Secure payment",
    Customer: provider.Customer{
        Name:    "Jane",
        Surname: "Smith",
        Email:   "jane@example.com",
    },
}

response, err := paymentService.Create3DPayment(ctx, "papara", paymentRequest)
if err != nil {
    log.Fatal(err)
}

// Returns redirect URL or HTML for 3D Secure
if response.RedirectURL != "" {
    log.Printf("3D Secure URL: %s", response.RedirectURL)
}
```

## Test Cards

Test cards you can use in Papara test environment:

| Card Number      | CVV | Expiry Date | Description          |
| ---------------- | --- | ----------- | -------------------- |
| 5528790000000008 | 123 | 12/2030     | Successful test card |
| 5528790000000016 | 123 | 12/2030     | Insufficient funds   |
| 5528790000000024 | 123 | 12/2030     | Invalid card         |

## Webhook Management

### Setting Webhook URL

Set your webhook URL in Papara merchant panel:

```
https://yourdomain.com/v1/callback/papara
```

### Webhook Validation

GoPay automatically validates webhook signatures:

```go
// Webhook handler example
func paparaWebhookHandler(w http.ResponseWriter, r *http.Request) {
    // GoPay automatically validates and processes the webhook
    // Signature validation is done with the API key
}
```

## Error Codes

### Papara Error Codes

| Code                     | Description            |
| ------------------------ | ---------------------- |
| `INSUFFICIENT_FUNDS`     | Insufficient funds     |
| `INVALID_CARD`           | Invalid card details   |
| `EXPIRED_CARD`           | Expired card           |
| `CARD_DECLINED`          | Card declined          |
| `FRAUDULENT_TRANSACTION` | Fraudulent transaction |

### HTTP Status Codes

| Code | Description               |
| ---- | ------------------------- |
| 200  | Successful                |
| 400  | Invalid request parameter |
| 401  | Authorization error       |
| 404  | Resource not found        |
| 500  | Server error              |

## Security

### API Key Security

- Store your API key securely
- Use environment variables
- Do not hard-code the API key in your code
- Regularly rotate your API key

### Webhook Security

- Webhook signatures are automatically validated
- HMAC-SHA256 algorithm is used
- API key is used for signature generation

## Troubleshooting

### Common Issues

1. **"Invalid API Key" Error**

   - Ensure your API key is correct
   - Check environment (sandbox/production) setting

2. **"Payment Not Found" Error**

   - Ensure the Payment ID is correct
   - Verify the payment was created in the same environment

3. **Webhook Validation Error**
   - Ensure webhook URL is configured correctly
   - Verify API key is used for webhook signing

### Running in Debug Mode

```bash
# Enable debug logs
export LOG_LEVEL=debug
go run ./cmd/main.go
```

### Test Commands

```bash
# Run unit tests
go test ./provider/papara/

# Run integration tests (API key required)
PAPARA_API_KEY=your_test_api_key go test ./provider/papara/ -v -run Integration
```

## Limitations

- Minimum payment amount: 1.00 TRY
- Maximum payment amount: 500,000.00 TRY
- Supported currency: TRY
- Refund operations only possible for completed payments
- Partial refunds are supported

## Support

For issues with Papara integration:

- Papara Merchant Support: [support@papara.com](mailto:support@papara.com)
- GoPay Issues: [GitHub Issues](https://github.com/mstgnz/gopay/issues)
- Papara Documentation: [https://docs.papara.com](https://docs.papara.com)

## Release Notes

### v1.0.0

- ✅ Initial release
- ✅ Basic payment operations
- ✅ 3D Secure support
- ✅ Refund and cancel operations
- ✅ Webhook validation

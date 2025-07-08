# Nkolay Payment Integration Guide

## Overview

Nkolay is a Turkish payment gateway provider that offers secure payment processing solutions. This integration allows you to process payments through Nkolay's API using GoPay's unified interface.

[Nkolay API Documentation](https://paynkolay.com.tr/entegrasyon)

## Features

- ✅ **Non-3D Payments**: Direct payment processing without 3D Secure
- ✅ **3D Secure Payments**: Enhanced security with 3D authentication
- ✅ **Payment Status Inquiry**: Real-time payment status checking
- ✅ **Refund Operations**: Full and partial refund support
- ✅ **Payment Cancellation**: Cancel authorized payments
- ✅ **Webhook Validation**: Secure webhook notification handling
- ✅ **Multi-currency Support**: Support for TRY and other currencies

## Configuration

### Environment Variables

Add these configuration variables to your `.env` file:

```bash
# Nkolay API credentials
NKOLAY_API_KEY=your_nkolay_api_key
NKOLAY_SECRET_KEY=your_nkolay_secret_key
NKOLAY_MERCHANT_ID=your_merchant_id
NKOLAY_ENVIRONMENT=sandbox  # or 'production'

# GoPay base URL for callbacks
APP_URL=https://your-gopay-domain.com
```

### Provider Configuration

```go
import (
    "github.com/mstgnz/gopay/provider"
    _ "github.com/mstgnz/gopay/provider/nkolay" // Import to register provider
)

// Create payment service
paymentService := provider.NewPaymentService()

// Add Nkolay provider
nkolayConfig := map[string]string{
    "apiKey":      "your-api-key",
    "secretKey":   "your-secret-key",
    "merchantId":  "your-merchant-id",
    "environment": "sandbox", // or "production"
}

err := paymentService.AddProvider("nkolay", nkolayConfig)
if err != nil {
    log.Fatal("Failed to add Nkolay provider:", err)
}
```

## API Endpoints

| Method   | Endpoint                          | Description        |
| -------- | --------------------------------- | ------------------ |
| `POST`   | `/v1/payments/nkolay`             | Process payment    |
| `GET`    | `/v1/payments/nkolay/{paymentID}` | Get payment status |
| `DELETE` | `/v1/payments/nkolay/{paymentID}` | Cancel payment     |
| `POST`   | `/v1/payments/nkolay/refund`      | Process refund     |
| `POST`   | `/v1/callback/nkolay`             | 3D Secure callback |
| `POST`   | `/v1/webhooks/nkolay`             | Webhook endpoint   |

## Usage Examples

### 1. Non-3D Payment

```bash
curl -X POST http://localhost:9999/v1/payments/nkolay \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john@example.com",
      "phoneNumber": "+905551234567"
    },
    "cardInfo": {
      "cardHolderName": "John Doe",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    }
  }'
```

### 2. 3D Secure Payment

```bash
curl -X POST http://localhost:9999/v1/payments/nkolay \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "use3D": true,
    "callbackUrl": "https://yourapp.com/payment-callback",
    "customer": {
      "name": "John",
      "surname": "Doe",
      "email": "john@example.com",
      "phoneNumber": "+905551234567",
      "ipAddress": "192.168.1.1"
    },
    "cardInfo": {
      "cardHolderName": "John Doe",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    }
  }'
```

### 3. Payment Status Inquiry

```bash
curl -X GET http://localhost:9999/v1/payments/nkolay/{paymentID} \
  -H "Authorization: Bearer your_jwt_token"
```

### 4. Refund Payment

```bash
curl -X POST http://localhost:9999/v1/payments/nkolay/refund \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "payment_123",
    "refundAmount": 50.25,
    "reason": "Customer request",
    "currency": "TRY"
  }'
```

### 5. Cancel Payment

```bash
curl -X DELETE http://localhost:9999/v1/payments/nkolay/{paymentID} \
  -H "Authorization: Bearer your_jwt_token" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "Customer cancellation"
  }'
```

## Response Format

### Successful Payment Response

```json
{
  "success": true,
  "status": "successful",
  "paymentId": "nkolay_payment_123",
  "transactionId": "txn_456789",
  "amount": 100.5,
  "currency": "TRY",
  "message": "Payment successful",
  "systemTime": "2024-01-15T10:30:00Z"
}
```

### 3D Secure Payment Response

```json
{
  "success": true,
  "status": "processing",
  "paymentId": "nkolay_payment_123",
  "redirectUrl": "https://3dsecure.nkolay.com/auth?token=abc123",
  "html": "<form action='...' method='post'>...</form>",
  "amount": 100.5,
  "currency": "TRY",
  "message": "3D authentication required"
}
```

### Error Response

```json
{
  "success": false,
  "status": "failed",
  "errorCode": "INVALID_CARD",
  "message": "Invalid card number",
  "amount": 100.5,
  "currency": "TRY"
}
```

## Error Codes

| Error Code               | Description                       |
| ------------------------ | --------------------------------- |
| `INSUFFICIENT_FUNDS`     | Insufficient funds in card        |
| `INVALID_CARD`           | Invalid card number or details    |
| `EXPIRED_CARD`           | Card has expired                  |
| `FRAUDULENT_TRANSACTION` | Transaction flagged as fraudulent |
| `CARD_DECLINED`          | Card declined by issuer           |
| `SYSTEM_ERROR`           | Nkolay system error               |

## Test Cards

For sandbox environment, you can use these test cards:

| Card Number      | Type       | 3D Secure | Expected Result    |
| ---------------- | ---------- | --------- | ------------------ |
| 5528790000000008 | MasterCard | Yes       | Success            |
| 4508034508034509 | Visa       | Yes       | Success            |
| 4157920000000002 | Visa       | No        | Success            |
| 5528790000000016 | MasterCard | Yes       | Insufficient Funds |
| 4508034508034517 | Visa       | Yes       | Invalid Card       |

**Common Test Card Details:**

- Expiry: Any future date (e.g., 12/2030)
- CVV: Any 3-digit number (e.g., 123)
- Cardholder Name: Any name

## Webhook Configuration

Nkolay will send webhook notifications to your configured endpoint. Configure the webhook URL in your Nkolay merchant panel:

```
https://your-gopay-domain.com/v1/webhooks/nkolay
```

### Webhook Payload Example

```json
{
  "paymentId": "nkolay_payment_123",
  "transactionId": "txn_456789",
  "status": "SUCCESS",
  "amount": 100.5,
  "currency": "TRY",
  "timestamp": 1642248600,
  "signature": "webhook_signature_hash"
}
```

## Security

### Authentication

- API requests are authenticated using API Key and Secret Key
- HMAC-SHA256 signature verification for all requests
- Timestamp validation to prevent replay attacks

### Webhook Validation

- All webhooks are signed with HMAC-SHA256
- Signature validation prevents unauthorized notifications
- Timestamp validation prevents replay attacks

## Integration Testing

Run integration tests with your Nkolay credentials:

```bash
# Set test credentials
export NKOLAY_API_KEY="test_api_key"
export NKOLAY_SECRET_KEY="test_secret_key"
export NKOLAY_MERCHANT_ID="test_merchant"
export NKOLAY_ENVIRONMENT="sandbox"

# Run Nkolay integration tests
make test-nkolay
```

## Production Checklist

Before going live with Nkolay:

- [ ] Obtain production API credentials from Nkolay
- [ ] Set `NKOLAY_ENVIRONMENT=production`
- [ ] Configure production webhook URLs
- [ ] Test with small amounts first
- [ ] Implement proper error handling
- [ ] Set up monitoring and logging
- [ ] Verify 3D Secure flow works correctly

## Support

For Nkolay-specific issues:

- Nkolay Documentation: [Nkolay Developer Portal](https://developer.nkolay.com)
- Nkolay Support: support@nkolay.com

For GoPay integration issues:

- GoPay Documentation: [GoPay GitHub](https://github.com/mstgnz/gopay)
- Create an issue on GitHub for bugs or feature requests

## API Version

This implementation is compatible with:

- **Nkolay API Version**: v1
- **GoPay Version**: v1.0+
- **Go Version**: 1.21+

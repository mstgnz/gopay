# GoPay Examples

This directory contains comprehensive examples for testing and integrating with GoPay payment providers.

## 🏢 Multi-Tenant Support

GoPay supports **multi-tenant architecture** where different tenants (projects/companies) can use different payment provider configurations. Each tenant can have their own İyzico, OzanPay, or Paycell credentials.

### How Multi-Tenant Works:

1. **Tenant Registration**: Each tenant sets their provider credentials via API
2. **Request Headers**: Include `X-Tenant-ID` header in payment requests
3. **Automatic Routing**: GoPay automatically uses tenant-specific provider configuration
4. **Persistence**: Configurations are stored in SQLite and persist across restarts

### Multi-Tenant Examples:

- `multi_tenant_setup.sh` - Complete multi-tenant setup examples
- `tenant_curl_examples.sh` - Multi-tenant payment examples with different tenants
- `multi_tenant/example.go` - Go integration example with multi-tenant support

## 📁 Available Examples

### Go Code Examples

- `iyzico_example.go` - Complete Go integration example for İyzico provider
- `multi_tenant/example.go` - Multi-tenant Go integration example with different tenants

### Curl Examples (REST API)

- `iyzico_curl_examples.sh` - İyzico provider curl examples
- `ozanpay_curl_examples.sh` - OzanPay provider curl examples
- `paycell_curl_examples.sh` - Paycell provider curl examples

## 🚀 Quick Start

### Prerequisites

1. **Start GoPay Server**

   ```bash
   cd /path/to/gopay
   go run cmd/main.go
   ```

   Default server runs on `http://localhost:9999`

2. **Configure Environment Variables**
   Create a `.env` file in the project root with your provider credentials:

   ```bash
   # İyzico Configuration
   IYZICO_API_KEY=your-iyzico-api-key
   IYZICO_SECRET_KEY=your-iyzico-secret-key
   IYZICO_ENVIRONMENT=sandbox

   # OzanPay Configuration
   OZANPAY_API_KEY=your-ozanpay-api-key
   OZANPAY_SECRET_KEY=your-ozanpay-secret-key
   OZANPAY_MERCHANT_ID=your-ozanpay-merchant-id
   OZANPAY_ENVIRONMENT=sandbox

   # Paycell Configuration
   PAYCELL_USERNAME=your-paycell-username
   PAYCELL_PASSWORD=your-paycell-password
   PAYCELL_MERCHANT_ID=your-paycell-merchant-id
   PAYCELL_TERMINAL_ID=your-paycell-terminal-id
   PAYCELL_ENVIRONMENT=sandbox
   ```

## 📖 Running Examples

### Go Example

```bash
cd examples
go run iyzico_example.go
```

### Curl Examples

```bash
# İyzico provider examples
./iyzico_curl_examples.sh

# OzanPay provider examples
./ozanpay_curl_examples.sh

# Paycell provider examples
./paycell_curl_examples.sh
```

## 🧪 Test Scenarios Covered

Each curl example file includes comprehensive test scenarios:

### 1. **Basic Payment Operations**

- Regular payment (without 3D Secure)
- 3D Secure payment
- Payment status check
- Payment cancellation
- Partial refund
- Full refund

### 2. **Advanced Scenarios**

- Multiple items payment
- Installment payments
- Marketplace payments (OzanPay)
- Recurring payments (OzanPay)
- Mobile payments (Paycell)
- Currency conversion (USD, EUR)

### 3. **Error Testing**

- Insufficient funds
- Invalid card numbers
- Expired cards
- Timeout scenarios
- Network errors

## 💳 Test Cards

### İyzico Test Cards

| Card Number        | Type      | Expected Result    |
| ------------------ | --------- | ------------------ |
| `5528790000000008` | Success   | SUCCESS            |
| `5528790000000016` | Error     | INSUFFICIENT_FUNDS |
| `5528790000000024` | Error     | INVALID_CARD       |
| `5528790000000032` | Error     | EXPIRED_CARD       |
| `5528790000000065` | 3D Secure | 3D_AUTH_REQUIRED   |

### OzanPay Test Cards

| Card Number        | Type           | Expected Result    |
| ------------------ | -------------- | ------------------ |
| `4111111111111111` | Visa Success   | SUCCESS            |
| `5555555555554444` | Master Success | SUCCESS            |
| `4000000000003220` | 3D Secure      | 3D_AUTH_REQUIRED   |
| `4000000000000002` | Declined       | CARD_DECLINED      |
| `4000000000009995` | Insufficient   | INSUFFICIENT_FUNDS |

### Paycell Test Cards

| Card Number        | Type         | Expected Result    |
| ------------------ | ------------ | ------------------ |
| `5528790000000008` | Success      | SUCCESS            |
| `5528790000000057` | 3D Secure    | 3D_AUTH_REQUIRED   |
| `5528790000000016` | Insufficient | INSUFFICIENT_FUNDS |
| `5528790000000024` | Invalid      | INVALID_CARD       |
| `5528790000000032` | Expired      | EXPIRED_CARD       |
| `5528790000000040` | Declined     | CARD_DECLINED      |

## 🔧 API Endpoints

GoPay provides both provider-specific and generic endpoints:

### Provider-Specific Endpoints

```
POST   /payments/{provider}              - Create payment
GET    /payments/{provider}/{paymentID}  - Get payment status
DELETE /payments/{provider}/{paymentID}  - Cancel payment
POST   /payments/{provider}/refund       - Refund payment
POST   /callback/{provider}              - 3D Secure callback
POST   /webhooks/{provider}              - Webhook notifications
```

### Generic Endpoints (Uses Default Provider)

```
POST   /payments                - Create payment
GET    /payments/{paymentID}    - Get payment status
DELETE /payments/{paymentID}    - Cancel payment
POST   /payments/refund         - Refund payment
POST   /callback                - 3D Secure callback
```

## 🛠 Customizing Examples

### Modifying Request Data

Each curl example can be customized by editing the JSON payload:

```bash
curl -X POST "http://localhost:9999/payments/iyzico" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.00,           # Change amount
    "currency": "TRY",          # Change currency
    "customer": {
      "name": "Your Name",      # Change customer details
      "email": "your@email.com"
    },
    "cardInfo": {
      "cardNumber": "5528790000000008"  # Use different test cards
    }
  }'
```

### Changing Base URL

If your GoPay server runs on a different port or host:

```bash
# Edit the BASE_URL variable in any .sh file
BASE_URL="http://localhost:8080"  # or your custom URL
```

### Adding Authentication

If you have API authentication enabled:

```bash
curl -X POST "${BASE_URL}/payments/iyzico" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{ ... }'
```

## 📊 Response Examples

### Successful Payment Response

```json
{
  "success": true,
  "status": "successful",
  "message": "Payment completed successfully",
  "data": {
    "success": true,
    "status": "successful",
    "paymentId": "pay_123456789",
    "transactionId": "txn_987654321",
    "amount": 100.5,
    "currency": "TRY",
    "systemTime": "2024-01-01T12:00:00Z"
  }
}
```

### 3D Secure Payment Response

```json
{
  "success": true,
  "status": "pending",
  "message": "3D Secure authentication required",
  "data": {
    "success": false,
    "status": "pending",
    "paymentId": "pay_123456789",
    "redirectUrl": "https://3d-secure-bank.com/auth",
    "html": "<form>...3D form...</form>"
  }
}
```

### Error Response

```json
{
  "success": false,
  "status": "failed",
  "message": "Payment failed",
  "data": {
    "success": false,
    "status": "failed",
    "errorCode": "INSUFFICIENT_FUNDS",
    "message": "Insufficient card balance"
  }
}
```

## 🔍 Debugging Tips

### 1. Check Server Logs

```bash
# Terminal where you started GoPay server
tail -f /path/to/gopay/logs/app.log
```

### 2. Verify Configuration

```bash
# Check if environment variables are loaded
curl http://localhost:9999/health
```

### 3. Test Provider Registration

```bash
# Check which providers are registered
curl http://localhost:9999/providers
```

### 4. Validate Request Format

Use JSON validators or tools like `jq`:

```bash
echo '{"amount": 100}' | jq .
```

## 📝 Notes

1. **Payment IDs**: Replace `PAYMENT_ID` in examples with actual payment IDs from responses
2. **Environment**: All examples default to sandbox/test environments
3. **Rate Limits**: Some providers have rate limits for testing
4. **3D Secure**: 3D Secure flows require browser interaction in real scenarios
5. **Webhooks**: Webhook examples require publicly accessible URLs

## 🆘 Troubleshooting

### Common Issues

1. **"Provider not found"**

   - Check environment variables are set correctly
   - Verify provider registration in server logs

2. **"Invalid credentials"**

   - Double-check API keys/credentials
   - Ensure using correct environment (sandbox vs production)

3. **"Connection refused"**

   - Make sure GoPay server is running
   - Check if port 9999 is available

4. **"Validation error"**
   - Verify JSON format is correct
   - Check required fields are present

### Getting Help

- Check the [main README](../README.md) for more information
- Review provider-specific documentation in `provider/{provider}/README.md`
- Look at server logs for detailed error messages

## 🎯 Next Steps

After running the examples:

1. **Integrate into your application** using the request/response patterns
2. **Implement webhook handling** for payment notifications
3. **Add error handling** based on the error scenarios
4. **Test in production** environment with real credentials
5. **Monitor payment flows** using the logging and stats features

Happy coding! 🚀

## 🏢 Multi-Tenant Architecture

GoPay supports multi-tenant architecture where different tenants (projects/companies) can use different payment provider configurations.

### Quick Multi-Tenant Setup

1. **Configure Tenants**

```bash
# Setup multiple tenants with their own credentials
./multi_tenant_setup.sh
```

2. **Use Tenant-Specific Payments**

```bash
# ABC tenant payment with their İyzico credentials
curl -X POST "${BASE_URL}/payments/iyzico" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC" \
  -d '{ "amount": 100.00, ... }'

# XYZ tenant payment with their OzanPay credentials
curl -X POST "${BASE_URL}/payments/ozanpay" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: XYZ" \
  -d '{ "amount": 200.00, ... }'
```

3. **Run Multi-Tenant Examples**

```bash
# Complete multi-tenant payment examples
./tenant_curl_examples.sh

# Go integration example
go run examples/multi_tenant/example.go
```

### Multi-Tenant Management

```bash
# Check tenant configuration
curl -X GET "${BASE_URL}/config/tenant-config?provider=iyzico" \
  -H "X-Tenant-ID: ABC"

# Get system statistics
curl -X GET "${BASE_URL}/stats"

# Update tenant configuration
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC" \
  -d '{ "IYZICO_ENVIRONMENT": "production" }'

# Delete tenant configuration
curl -X DELETE "${BASE_URL}/config/tenant-config?provider=iyzico" \
  -H "X-Tenant-ID: ABC"
```

### Multi-Tenant Benefits

- **🔒 Isolation**: Each tenant uses their own provider credentials
- **🔄 Flexibility**: Different tenants can use different payment providers
- **💾 Persistence**: Configurations survive application restarts
- **📈 Scalability**: Supports unlimited number of tenants
- **🔐 Security**: Tenant credentials are stored securely and separately

### Use Cases

- **SaaS Platforms**: Different customers use their own payment gateways
- **Marketplace Applications**: Multiple vendors with separate payment processing
- **White-label Solutions**: Different brands with their own payment configurations
- **Multi-region Deployments**: Different regions with localized payment providers

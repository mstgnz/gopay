# Payten Payment Provider

https://merchantsafeunipay.com/msu/api/v2/doc

This provider implements payment processing for Payten Payment Gateway using Direct Post integration method.

## Configuration

Required configuration parameters:

- `merchant`: Payten Merchant ID
- `merchantUser`: Payten Merchant User
- `merchantPassword`: Payten Merchant Password
- `secretKey`: Payten Secret Key (Store Key) for hash calculation
- `environment`: Either "sandbox" or "production"

## Test Credentials

For testing purposes, you need to obtain test credentials from Payten. Contact Payten support for test merchant credentials.

## Features

- ✅ Non-3D payments (Direct Post Non 3D)
- ✅ 3D Secure payments (Direct Post 3D)
- ✅ Payment cancellation (VOID)
- ✅ Refund processing
- ⚠️ Payment status inquiry (requires additional API)

## API Endpoints

### API v2

- Sandbox & Production: `https://merchantsafeunipay.com/msu/api/v2`

### 3D Secure Gateway

- Gateway: `https://merchantsafeunipay.com/msu/3dgate` (both sandbox and production)

## Authentication

Uses SHA512 hash calculation with form parameters (ver3 format):

1. All form parameters (except `hash` and `encoding`) are sorted alphabetically (case-insensitive)
2. Parameter values are escaped (`|` → `\|`, `\` → `\\`)
3. Parameters are joined with `|` separator
4. Secret key (Store Key) is appended (also escaped)
5. SHA512 hash is calculated and converted to hex
6. Hex string is packed to bytes and base64 encoded

## Payment Actions

- **SALE**: Direct sale transaction
- **PREAUTH**: Pre-authorization (not yet implemented)
- **VOID**: Cancel/void transaction
- **REFUND**: Refund transaction

## 3D Secure Flow

1. **Create3DPayment**: Generates HTML form with all payment parameters and hash
2. **User Authentication**: User is redirected to Payten 3D Secure page
3. **Callback**: Payten redirects back to GoPay callback URL with payment result
4. **Complete3DPayment**: Validates hash and processes payment result

## Notes

- All amounts are sent with 2 decimal places (e.g., "100.50")
- Currency code for TRY is "TRY"
- Merchant Payment ID is automatically generated if not provided
- 3D Secure uses form-based POST submission
- Hash algorithm: ver3 (SHA512)

## Documentation

Full API documentation: https://merchantsafeunipay.com/msu/api/v2/doc

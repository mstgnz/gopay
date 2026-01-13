# Ziraat Payment Provider

https://merchantsafeunipay.com/msu/api/v2/doc (Ziraat Bankası Payten altyapısını kullanıyor)

This provider implements payment processing for Ziraat Virtual POS using Payten infrastructure.

## Configuration

Required configuration parameters:

- `merchantSafeId`: Merchant Safe ID provided by Ziraat (also used as clientid for 3D Secure)
- `terminalSafeId`: Terminal Safe ID provided by Ziraat
- `secretKey`: Security Key for HMAC authentication and 3D Secure hash calculation
- `environment`: Either "sandbox" or "production"

## Test Credentials

For testing purposes, use the following credentials:

```
merchantSafeId: 2025100217305644994AAC1BF57EC29B
terminalSafeId: 202510021730564616275A2A52298FCF
secretKey: 323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532
```

## Features

- ✅ Non-3D payments
- ✅ 3D Secure payments (form-based Payten flow)
- ✅ Payment cancellation
- ✅ Refund processing
- ⚠️ Payment status inquiry (requires additional API)

## API Endpoints

### Non-3D Payments
- Sandbox: `https://apipre.ziraat.com/api/v1/payment/virtualpos/transaction/process`
- Production: `https://api.ziraat.com/api/v1/payment/virtualpos/transaction/process`

### 3D Secure Payments
- Gateway: `https://merchantsafeunipay.com/msu/3dgate` (both sandbox and production)

## Authentication

### Non-3D Payments
Uses HMAC-SHA512 authentication with the entire JSON request body. The hash is sent in the `auth-hash` header.

### 3D Secure Payments
Uses SHA512 hash calculation with form parameters:
1. All form parameters (except `hash` and `encoding`) are sorted alphabetically (case-insensitive)
2. Parameter values are escaped (`|` → `\|`, `\` → `\\`)
3. Parameters are joined with `|` separator
4. Secret key is appended (also escaped)
5. SHA512 hash is calculated and converted to hex
6. Hex string is packed to bytes and base64 encoded

## 3D Secure Flow

1. **Create3DPayment**: Generates HTML form with all payment parameters and hash
2. **User Authentication**: User is redirected to Payten 3D Secure page
3. **Callback**: Payten redirects back to GoPay callback URL with payment result
4. **Complete3DPayment**: Validates hash and processes payment result

## Notes

- All amounts are sent in kuruş (Turkish cents). Multiply by 100 before sending.
- Currency code for TRY is 949
- Order IDs are automatically generated in the format: YY + MONTH_NAME + DAY_NAME + SECONDS
- 3D Secure uses form-based POST submission (Payten standard)
- Card type is automatically detected (1=Visa, 2=MasterCard) based on first digit

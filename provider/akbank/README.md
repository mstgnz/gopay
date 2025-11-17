# Akbank Payment Provider

https://sanalpos-prep.akbank.com/#entry

This provider implements payment processing for Akbank Virtual POS.

## Configuration

Required configuration parameters:

- `merchantSafeId`: Merchant Safe ID provided by Akbank
- `terminalSafeId`: Terminal Safe ID provided by Akbank
- `secretKey`: Security Key for HMAC authentication
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
- ✅ Payment cancellation
- ✅ Refund processing
- ⚠️ 3D Secure payments (in progress)
- ⚠️ Payment status inquiry (requires additional API)

## API Endpoint

- Production & Sandbox: `https://api.akbank.com/api/v1/payment/virtualpos/transaction/process`

## Authentication

Uses HMAC-SHA512 authentication with the entire JSON request body. The hash is sent in the `auth-hash` header.

## Notes

- All amounts are sent in kuruş (Turkish cents). Multiply by 100 before sending.
- Currency code for TRY is 949
- Order IDs are automatically generated in the format: YY + MONTH + DAY + SECONDS
- The API endpoint is the same for both sandbox and production environments

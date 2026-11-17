# iyzico in GoPay: HTTP contract

iyzico payments go through iyzico's hosted **Checkout Form**: the customer types the card on
iyzico's page, so card data never reaches GoPay or the tenant. **Marketplace** (sub-merchants,
item approval) is supported. Amounts are exact decimal strings on every route below.

Verified against docs.iyzico.com and the official Java, PHP and Node SDKs on 2026-11-17 (there
is no official Go SDK). Requests are signed with IYZWSv2 (HMAC-SHA256).

## Setup

1. A GoPay admin stores the tenant's iyzico key pair (merchant panel, Settings > Company Settings):

   ```http
   POST /v1/config/tenant
   Authorization: Bearer <admin JWT>

   {"tenantId": 5, "provider": "iyzico", "environment": "sandbox",
    "configs": [{"key": "apiKey", "value": "sandbox-..."}, {"key": "secretKey", "value": "sandbox-..."}]}
   ```

2. Every call below carries the tenant's JWT (`Authorization: Bearer ...`) and
   `?environment=sandbox|production`. A missing or unknown environment is `400`.

## Response envelope and failure kinds

Every answer is `{"code", "success", "message", "error", "data"}`.

| HTTP | `data.kind` | Meaning | What the caller does |
|---|---|---|---|
| 200 | | The call was done (for a result lookup: see `outcome`). | Act on `data`. |
| 400 | | GoPay refused the request before calling iyzico (`error` names the field). | Fix the request. |
| 422 | `refused` | iyzico answered and refused. `providerErrorCode`, `providerErrorMessage`, `retryable`. | Same call unchanged will fail; see the codes below. |
| 502 | `unavailable` | No answer from iyzico: network, timeout, HTTP 5xx or 429, rate limit (50000), an unreadable answer, or an initialize/refund answer whose signature does not match. | **Outcome unknown.** Read the state again later; never record it as failed. |
| 500 | `internal` | GoPay error (for example, no iyzico keys for this tenant). | **Outcome unknown.** Same as 502. |

Only `200` and `422` are answers from iyzico.

## Payment flow

1. Server-side, the tenant calls `POST /checkout` and redirects the customer to `paymentPageUrl`.
2. The customer pays on iyzico's page. iyzico then POSTs **only `token`** (form field) to the
   tenant's `callbackUrl`, through the customer's browser. That POST proves nothing: anyone can
   send it.
3. The tenant reads the verdict server-side with `GET /checkout/{token}` and acts only on
   `outcome`. A reconcile job repeats the lookup for payments left at `no_verdict`.

### Start: `POST /v1/payments/iyzico/checkout`

```json
{
  "conversationId": "bk-9-1",
  "basketId": "BK-9",
  "price": "450.50",
  "currency": "TRY",
  "callbackUrl": "https://tenant.example/odeme/sonuc",
  "locale": "tr",
  "buyer": {
    "id": "u-7", "name": "Ada", "surname": "Kaya", "identityNumber": "12345678901",
    "email": "ada@example.com", "gsmNumber": "+905551112233",
    "address": "Moda Cd. 1", "city": "Istanbul", "country": "Turkey", "zipCode": "34710",
    "ip": "85.34.78.112"
  },
  "billingAddress": {"contactName": "Ada Kaya", "address": "Moda Cd. 1", "city": "Istanbul", "country": "Turkey"},
  "items": [
    {"id": "BK-9", "name": "Saha 1, 20:00", "category": "Saha kiralama", "itemType": "VIRTUAL",
     "price": "450.50", "subMerchantKey": "smk-...", "subMerchantPrice": "405.45"}
  ]
}
```

- Optional: `conversationId` (generated when absent, returned), `currency` (only `TRY`), `locale`
  (`tr` default, `en`), `buyer.country` and address countries (`Turkey` default), `zipCode`,
  `billingAddress` (defaults to the buyer's name and address), `shippingAddress` (required only when
  an item is `PHYSICAL`; leave it out for `VIRTUAL` baskets).
- Amounts: strings, at most 2 fraction digits, no exponent (`"450.50"`, `"450.5"`, `"450"`). A JSON
  number is refused.
- `price` must equal the sum of `items[].price` exactly. `subMerchantKey` and `subMerchantPrice` come
  together; `0 < subMerchantPrice <= price` of the item. The customer is charged `price`, in one
  installment.
- `buyer.ip` is the customer's IP (not the tenant server's). `buyer.identityNumber` is the TCKN
  (5 to 20 characters). `gsmNumber` is E.164.
- `callbackUrl` must be `https`; `http` is accepted only with `environment=sandbox`.
- Unknown fields are refused (`400`).

`200`:

```json
{"token": "4f3...", "paymentPageUrl": "https://sandbox-cpp.iyzipay.com?token=4f3...",
 "tokenExpireSeconds": 1800, "conversationId": "bk-9-1"}
```

### Result: `GET /v1/payments/iyzico/checkout/{token}?conversationId=bk-9-1`

`conversationId` is optional (the token is used when absent). `200` for every outcome; the
envelope's `success` is `true` only for `paid`.

```json
{
  "outcome": "paid",
  "readable": true,
  "paymentStatus": "SUCCESS",
  "fraudStatus": 1,
  "paymentId": "28157797",
  "conversationId": "bk-9-1",
  "basketId": "BK-9",
  "price": "450.5",
  "paidPrice": "450.5",
  "currency": "TRY",
  "itemTransactions": [
    {"itemId": "BK-9", "paymentTransactionId": "30113131", "transactionStatus": 1,
     "price": "450.5", "paidPrice": "450.5", "subMerchantKey": "smk-...", "subMerchantPrice": "405.45"}
  ]
}
```

Amounts come back exact with trailing zeros removed (`"450.5"`, `"0"`): compare them as decimals,
not as strings.

| `outcome` | When | Tenant action |
|---|---|---|
| `paid` | `paymentStatus` SUCCESS, `fraudStatus` 1, every item has a `paymentTransactionId`, answer signed by iyzico | Confirm. Store `paymentId` (cancel) and each `paymentTransactionId` (refund, approval). |
| `in_review` | SUCCESS with `fraudStatus` 0 | Hold; read again later. Do not deliver. |
| `declined` | `paymentStatus` FAILURE, or `fraudStatus` -1 | Release the booking once the callback for this token has arrived or `tokenExpireSeconds` has passed: while the page is open the customer may still retry with another card (unverified at iyzico). `errorCode` may carry the bank code (e.g. 10051 no funds). |
| `no_verdict` | iyzico error answer (`readable: false`, `errorCode` set), page or 3-D Secure still open (`INIT_THREEDS`, `CALLBACK_THREEDS`), or an answer GoPay cannot trust | Keep the payment open; read again later. See "Abandoned pages" below. |

A verdict needs iyzico's response signature. On a success answer it must be present and match. iyzico
documents no signature on error answers, so a FAILURE in an error answer counts unsigned, but a
signature sent there must match.

`errorCode` values that GoPay itself sets start with `GOPAY_`:
`GOPAY_SIGNATURE_MISMATCH` (the signature was missing or did not match; never acted on) and
`GOPAY_UNREADABLE_RESULT` (SUCCESS without a fraud status, a payment id or item lines).

### Abandoned pages (open item, settle in the sandbox before go-live)

A customer who opens the page and leaves never produces a verdict, and iyzico does not document what
the retrieve answers after the token expires. Until the sandbox run settles it, GoPay keeps every such
answer at `no_verdict`, and the tenant decides when an unpaid booking is abandoned. A rule of the form
"past `tokenExpireSeconds`, an error answer means not paid" is safe only if iyzico still answers a
**paid** token after expiry; check that first:

1. Start a checkout, pay it with `5528790000000008`, wait more than `tokenExpireSeconds`, retrieve.
2. Start a checkout, leave the page, wait more than `tokenExpireSeconds`, retrieve.
3. Record both answers (`errorCode`, `paymentStatus`) here.

`transactionStatus` per item (iyzico): `0` fraud review, `-1` rejected, `1` approved and, in a
marketplace, waiting for item approval, `2` item approval given.

The token's lifetime after the page is paid is not documented by iyzico; read the result soon
after the callback and keep `paymentId` for later lookups through `GET /v1/payments/iyzico/{paymentId}`.

## Returns

The same day: **cancel** the whole payment. A later day, up to 365 days: **refund** per item. iyzico
does not say which day the bank has closed, so:

- cancel refused on a later day: refund instead (iyzico documents no code for this; treat any `422` on
  cancel as "refund instead" after checking the payment's day);
- refund refused with **10223** ("End of day required for the POS") on the payment's day: cancel instead.

### Cancel: `POST /v1/payments/iyzico/checkout/cancel`

```json
{"paymentId": "28157797", "ip": "85.34.78.112", "conversationId": "c-1", "reason": "BUYER_REQUEST", "description": "Customer cancelled"}
```

`ip` is required (the IP of whoever asked for the cancel). `reason` is optional, one of `OTHER`,
`FRAUD`, `BUYER_REQUEST`, `DOUBLE_PAYMENT`; `description` needs a `reason`.
`200`: `{"paymentId": "28157797", "price": "450.5", "currency": "TRY", "conversationId": "c-1"}`.

### Refund: `POST /v1/payments/iyzico/checkout/refund`

```json
{"paymentTransactionId": "30113131", "price": "100.00", "ip": "85.34.78.112", "conversationId": "r-1", "reason": "BUYER_REQUEST"}
```

`price` at most the item's `paidPrice` minus what was refunded already; several partial refunds are
allowed. `200`: `{"paymentId", "paymentTransactionId", "price", "currency", "conversationId"}`.

**iyzico does not deduplicate refunds.** After a `502` or `500` the refund may have happened. Do not
send it again blindly: check the merchant panel or wait for reconciliation first.

## Marketplace

### Create a sub-merchant: `POST /v1/payments/iyzico/submerchants`

```json
{"externalId": "fac-1", "type": "PERSONAL", "name": "Moda Spor", "email": "info@moda.example",
 "gsmNumber": "+905551112233", "address": "Moda Cd. 1", "iban": "TR330006100519786457841326",
 "contactName": "Ada", "contactSurname": "Kaya", "identityNumber": "12345678901", "conversationId": "c"}
```

Required by `type` (iyzico):

| `type` | Required besides `externalId`, `email`, `gsmNumber`, `address` |
|---|---|
| `PERSONAL` | `contactName`, `contactSurname`, `identityNumber` |
| `PRIVATE_COMPANY` | `taxOffice`, `legalCompanyTitle` |
| `LIMITED_OR_JOINT_STOCK_COMPANY` | `taxOffice`, `taxNumber`, `legalCompanyTitle` |

`iban` may wait until before the first item approval. iyzico refuses item approval for a
sub-merchant created without `identityNumber` (5024). `200`: `{"subMerchantKey": "...", "conversationId": "c"}`.
A duplicate `externalId` is `422` with `2002`. A `502` may mean the sub-merchant was created: look it
up by `externalId` before creating again.

### Update: `PUT /v1/payments/iyzico/submerchants/{subMerchantKey}`

Same fields as create without `externalId` and `type` (iyzico does not let them change). `email`,
`gsmNumber`, `address` and `iban` are required; iyzico also requires the identity fields of the type.
`200`: `{"subMerchantKey": "..."}`.

### Look up: `GET /v1/payments/iyzico/submerchants/{externalId}`

`200`: `{"subMerchantKey", "externalId", "type", "name", "email", "gsmNumber", "address", "iban",
"currency", "taxOffice", "legalCompanyTitle"}`. Identity and tax numbers are not returned.

### Item approval

`POST /v1/payments/iyzico/items/{paymentTransactionId}/approve` releases a paid item's
sub-merchant share; `/disapprove` withdraws it. The body is optional: `{"conversationId": "c"}`.
`200`: `{"paymentTransactionId": "30113131", "conversationId": "c"}`. Already approved: `422` with `5064`.
There is no panel action for approval at iyzico; only this call.

## Common routes for iyzico

| Route | Behaviour |
|---|---|
| `GET /v1/payments/iyzico/{paymentId}` | `/payment/detail` by iyzico paymentId. `status`: `successful` (SUCCESS + fraud 1), `failed` (FAILURE or fraud -1), otherwise `pending`; an iyzico error answer is `pending`, and so is a verdict without a matching signature (`errorCode` `GOPAY_SIGNATURE_MISMATCH`). Amounts are float64 here. |
| `DELETE /v1/payments/iyzico/{paymentId}` | Cancel without a buyer IP. Prefer `POST /checkout/cancel`. |
| `POST /v1/payments/iyzico/refund` | `paymentId` is the item's `paymentTransactionId`; `refundAmount` is a float. Prefer `POST /checkout/refund`. |
| `POST /v1/payments/iyzico` (card), installments, commission, webhooks | Not supported: card data does not pass through GoPay for iyzico. |

## Logging

Every call is logged in the `iyzico` table. `identityNumber`, `iban` and `taxNumber` are masked
there. Failed calls carry `error_code` `REFUSED_<iyzico code>`, `PROVIDER_UNAVAILABLE` or
`INVALID_REQUEST`. The `payment_id` column stays empty on these rows, so
`GET /v1/logs/iyzico/payment/{id}` does not find them; search the request JSON by token or basket id.

## Tests

```bash
go test ./provider/iyzico/   # httptest, no network
IYZICO_TEST_API_KEY=... IYZICO_TEST_SECRET_KEY=... go test -tags integration -run TestIntegration ./provider/iyzico/
```

Sandbox test cards: docs.iyzico.com "Test Cards" (`5528790000000008` succeeds, `4111111111111129`
has no funds). `postman.json` here is iyzico's own collection, kept as a reference.

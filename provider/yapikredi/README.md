# Yapı Kredi (Posnet) provider

Yapı Kredi virtual POS over the Posnet XML service, with 3D Secure. Provider name and log table:
`yapikredi`.

Bank documents: "POSNET XML Servisleri Entegrasyon Dokümanı" 2.1.1.4 and "POSNET 3D Secure
Entegrasyonu" 2.0.1.6, from yapikredipos.com.tr, "Sanal POS Entegrasyon Dokümanları".

## Tenant config

| Key          | Value                                                        |
| ------------ | ------------------------------------------------------------ |
| `merchantId` | MID, 10 digits                                               |
| `terminalId` | TID, 8 characters (test TIDs can contain letters)            |
| `posnetId`   | Posnet ID, up to 16 digits                                   |
| `encKey`     | ENCKEY for the 3D MAC; fixed in the test system, 15-35 chars live |

The bank accepts requests only from IPs it whitelisted for the MID/TID; anything else answers
`0148`. Test IPs are added by mail to posnet.support@yapikredi.com.tr, live IPs in the corporate
internet banking (Üye İşyeri - Posnet - Bilgilerim - IP İşlemleri).

## Flows

- **Non-3D sale** (`use3D: false`): one `sale` call. `paymentId` is a 24-char orderID.
- **3D sale** (`use3D: true`): `oosRequestData` encrypts the payment; the answer is an HTML form that
  posts the payer to the bank's 3D page. `paymentId` is a 20-char XID. The bank posts
  `MerchantPacket`, `BankPacket` and `Sign` to the GoPay callback, which calls
  `oosResolveMerchantData` and, only when mdStatus is 1, xid/amount/currency match and the bank MAC
  verifies, `oosTranData` to charge. The browser's own fields are never trusted.
- **Cancel** (`DELETE`): `reverse` of the sale, same day only. After the bank's day end (00:00-00:05)
  the bank answers `0211`; use refund.
- **Refund**: `return`, partial and repeated allowed up to the sale amount. Before the day end the
  bank answers `0411`; use cancel. GoPay never switches between the two on its own.
- **Status**: the `agreement` service, by orderID (`TDS_` + XID for 3D). Cancel and refund read the
  sale's hostLogKey from it too, so they do not depend on GoPay's log rows.

Only TRY is enabled (sent as `TL`). Installments are sent as given (`00` for a single payment);
there is no installment or commission inquiry.

## Status

| Bank answer                                   | Status                              |
| --------------------------------------------- | ----------------------------------- |
| Sale row, txnStatus 1                         | successful                          |
| + successful Sale_Reverse                     | cancelled                           |
| + more successful Return than Return_Reverse  | refunded                            |
| Sale row, txnStatus 0                         | failed                              |
| A row with a state or txnStatus GoPay does not know, or none | pending, logged as a warning |
| No record (E219), 3D, callback page open      | pending                             |
| No record, 3D, callback logged failed         | failed                              |
| No record, 3D, no callback logged, expired    | failed (`3D_SESSION_EXPIRED`, or the start's own error) |
| No record, any other case                     | error: outcome unknown, query again |

A cancel the bank declines answers `success: false` with the payment's unchanged status
(`successful`) and the bank's code, e.g. `0211`.

A 3D finance call that got no answer, or whose MAC does not verify, answers `pending` with
`VERIFICATION_UNAVAILABLE`; the status query settles it.

## Card data

The XML request carries the PAN and CVV and is never logged or returned. Log rows get only
non-card fields (`saleRequest`, `oosRequest`, `*Response` keys). The 3D form's `posnetData2` is the
bank-encrypted card; the log sanitizer blanks it. Bank messages pass through `safeBankText`: a schema
rejection (which echoes the rejected value) becomes a generic text, and PAN-length digit runs are masked.

## Not verified yet (verify in the bank's test system)

- The agreement `state` names. The code expects the transaction type names of the XML document
  (Sale, Sale_Reverse, Return, Return_Reverse, Capture); anything else reads pending.
- The agreement status element and its failure value. The sample says `txnStatus`, the field table
  `txstatus`; all spellings are read. Only `1` is documented; `0` is assumed to mean failed.
- The charset of real answers (iso-8859-9 is handled, and utf-16 with or without a BOM).
- That the bank keeps the `?state=` query of `merchantReturnURL` when it posts back.
- That the bank's response MAC uses the same amount format GoPay sends.

`yapikredi_integration_test.go` (`-tags integration`) runs a sale, status and cancel against the
test system and logs the agreement rows; it only works from a whitelisted IP.

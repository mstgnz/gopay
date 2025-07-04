# Paycell Error Codes

This document contains all error codes returned by the Paycell API system.

## General Error Codes (1000-1999)

| Code | Description                                |
| ---- | ------------------------------------------ |
| 1002 | Internal error                             |
| 1003 | MSISDN and creditcard ID is not matched    |
| 1004 | Only default card is expected              |
| 1005 | CardId is not found                        |
| 1018 | Account not found                          |
| 1021 | Token not found                            |
| 1022 | 3D session not found                       |
| 1023 | 3D session already used                    |
| 1024 | An error occurred during 3D authentication |
| 1026 | Merchant 3D authentication url not found   |
| 1027 | Merchant not found                         |
| 1028 | Signature is invalid                       |
| 1029 | Hash is invalid                            |
| 1030 | Hashparams not found                       |
| 1031 | Posnet API request error                   |
| 1032 | Acs enrollment request error               |
| 1033 | Payment method not found                   |
| 1034 | Callback url is invalid                    |

## Request Validation Errors (2000-2999)

| Code | Description                                               |
| ---- | --------------------------------------------------------- |
| 2000 | Request header is invalid                                 |
| 2001 | MSISDN parameter is empty                                 |
| 2002 | CardToken parameter is empty                              |
| 2003 | IsDefault parameter is empty                              |
| 2004 | EulaId parameter is empty                                 |
| 2005 | CardId parameter is empty                                 |
| 2006 | MerchantCode parameter is empty                           |
| 2007 | ReferenceNumber parameter is empty                        |
| 2008 | Amount parameter is empty                                 |
| 2009 | PaymentType parameter is empty                            |
| 2010 | OriginalReferanceNumber parameter is empty                |
| 2011 | Merchant is not found                                     |
| 2012 | Referance number is not unique                            |
| 2013 | Order is not found                                        |
| 2014 | Transaction is already reversed                           |
| 2015 | Total refund amount is greater than original amount       |
| 2016 | ReconciliationDate parameter is empty                     |
| 2017 | TotalSaleAmount parameter is empty                        |
| 2018 | TotalReverseAmount parameter is empty                     |
| 2019 | TotalRefundAmount parameter is empty                      |
| 2020 | TotalSaleCount parameter is empty                         |
| 2021 | TotalReverseCount parameter is empty                      |
| 2022 | TotalRefundCount parameter is empty                       |
| 2023 | Application is invalid                                    |
| 2025 | EulaId is invalid                                         |
| 2026 | EulaId is not found                                       |
| 2027 | Original sale transaction is refunded                     |
| 2028 | Refund transaction can not be reversed                    |
| 2029 | Transaction can not be reversed after reconciliation date |
| 2030 | Amount parameter is invalid                               |
| 2031 | There was a missing parameter                             |
| 2032 | There was a invalid parameter                             |
| 2041 | Provision transaction entity not found                    |
| 2042 | Application Not Found                                     |
| 2043 | Original reference number must be empty                   |
| 2044 | Payment reference number mismatch                         |
| 2045 | Postauth amount greater than preauth amount               |
| 2046 | 3D Validation Provision Amount Mismatch                   |
| 2047 | 3D Session Id Must Be Empty                               |
| 2048 | Transaction is not suitable for refund                    |
| 2049 | 3d validation is required for payment card                |

## Card and Customer Errors (3000-3999)

| Code | Description                                                     |
| ---- | --------------------------------------------------------------- |
| 3001 | Credit card is not valid                                        |
| 3002 | Max number of card for MSISDN is exceeded                       |
| 3003 | Card is used by another application                             |
| 3005 | Credit card is already registered                               |
| 3006 | Customer is not suitable for provision                          |
| 3007 | Customer limit is exceeded for provision                        |
| 3008 | Transaction can not be done because of reconciliation           |
| 3009 | Max number of MSISDN for card is exceeded                       |
| 3010 | Card is already registered for different National ID            |
| 3011 | Tenure check error                                              |
| 3012 | MSISDN in chargeback blacklist                                  |
| 3013 | MSISDN in fraud list                                            |
| 3014 | Payment method not found                                        |
| 3015 | There is an error while sending service booster message         |
| 3017 | Provision timeout                                               |
| 3019 | Amex Payment method found but not supported                     |
| 3020 | Customer is corporate customer                                  |
| 3021 | Customer is personal customer                                   |
| 3022 | TCKN value is not found for the customer                        |
| 3023 | Transaction is being processed                                  |
| 3025 | Account suspended                                               |
| 3032 | 3D authentication is not successful                             |
| 3033 | Payment method is not validated                                 |
| 3034 | Token expired                                                   |
| 3035 | 3D session expired                                              |
| 3036 | User is not payment capable on Zubizu                           |
| 3037 | EulaId is not valid                                             |
| 3038 | Application is not found for this Channel                       |
| 3039 | Terms of Services not found for application                     |
| 3048 | Merchant applicationId and request applicationId mismatch       |
| 3060 | User is not payment capable on Shell                            |
| 3061 | Submerchantkey is mandatory                                     |
| 3062 | Debit card registration is not allowed for this channel         |
| 3063 | Paycell card registration is not allowed for this channel       |
| 3064 | Paycell card deletion is not allowed for this channel           |
| 3065 | International card registration is not allowed for this channel |
| 3068 | Credit card is already canceled                                 |
| 3101 | Invalid Payment Approval Method                                 |
| 3102 | Loyalty Card Is Already Registered                              |
| 3103 | Plate Is Already Registered                                     |

## Bank and Payment Errors (4000-4999)

| Code | Description                              |
| ---- | ---------------------------------------- |
| 4000 | Bank error                               |
| 4001 | Insufficient balance                     |
| 4002 | Expired card                             |
| 4003 | Provision is restricted for card holder  |
| 4004 | Provision is restricted                  |
| 4015 | Card info is empty                       |
| 4016 | Credit card is not allowed for ecommerce |
| 4050 | Record Not Found                         |
| 4052 | Payment Not Found                        |

## GetCardTokenSecure Specific Errors (80000-90000)

| Code  | Description (Turkish)                        | Description (English)                          |
| ----- | -------------------------------------------- | ---------------------------------------------- |
| 90063 | Input degerleri dogrulanamadi                | Input values could not be validated            |
| 80003 | header bos olamaz                            | Header cannot be empty                         |
| 80003 | applicationName alani bos olamaz             | ApplicationName field cannot be empty          |
| 80003 | transactionDateTime bos olamaz               | TransactionDateTime cannot be empty            |
| 80003 | Transaction Id bos ya da formati dogru degil | Transaction ID is empty or format is incorrect |
| 80003 | creditCardNo alani bos olamaz                | CreditCardNo field cannot be empty             |
| 80003 | expireDateMonth alani bos olamaz             | ExpireDateMonth field cannot be empty          |
| 80003 | expireDateYear alani bos olamaz              | ExpireDateYear field cannot be empty           |
| 80003 | hashData alani bos olamaz                    | HashData field cannot be empty                 |
| 90000 | Sistem hatasi                                | System error                                   |

## Common Success Codes

| Code | Description           |
| ---- | --------------------- |
| 0    | Success               |
| 200  | Success (alternative) |

## Error Handling Guidelines

### Authentication Errors

- **90000**: System error - usually indicates authentication failure
- **1029**: Hash is invalid - check hash generation algorithm
- **2023**: Application is invalid - verify credentials

### Transaction ID Errors

- **80003**: Transaction ID format error - ensure 20-digit numeric format
- **2012**: Reference number is not unique - use unique transaction IDs

### Card Token Errors

- **1021**: Token not found - regenerate card token
- **3034**: Token expired - get new card token

### 3D Secure Errors

- **1022**: 3D session not found
- **1023**: 3D session already used
- **3032**: 3D authentication is not successful
- **3035**: 3D session expired

### Common Integration Issues

1. **Hash Generation**: Ensure proper SHA-256 + base64 encoding with uppercase conversion
2. **Transaction ID Format**: Must be exactly 20 digits
3. **DateTime Format**: Use `YYYYMMddHHmmssSSS` (17 characters)
4. **Expire Year**: Use 2-digit format (e.g., "26" not "2026")

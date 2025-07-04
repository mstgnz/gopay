# Paycell REST Service List

## Test Environment Endpoints

### Provision Services

| Service                   | Method                     | Test URL                                                                                                     |
| ------------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Get Cards                 | `getCards`                 | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getCards/                 |
| Register Card             | `registerCard`             | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/registerCard/             |
| Update Card               | `updateCard`               | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/updateCard/               |
| Delete Card               | `deleteCard`               | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/deleteCard/               |
| Provision                 | `provision`                | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/provision/                |
| Inquire                   | `inquire`                  | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/inquire/                  |
| Reverse                   | `reverse`                  | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/reverse/                  |
| Refund                    | `refund`                   | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/refund/                   |
| Summary Reconciliation    | `summaryReconciliation`    | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/summaryReconciliation/    |
| Get 3D Session            | `getThreeDSession`         | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getThreeDSession/         |
| Get 3D Session Result     | `getThreeDSessionResult`   | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getThreeDSessionResult/   |
| Get Provision History     | `getProvisionHistory`      | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getProvisionHistory       |
| Provision for Marketplace | `provisionForMarketPlace`  | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/provisionForMarketPlace/  |
| Get Terms of Service      | `getTermsOfServiceContent` | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getTermsOfServiceContent/ |
| Get Card BIN Information  | `getCardBinInformation`    | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getCardBinInformation/    |
| Get Payment Methods       | `getPaymentMethods`        | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/getPaymentMethods/        |
| Open Mobile Payment       | `openMobilePayment`        | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/openMobilePayment/        |
| Send OTP                  | `sendOTP`                  | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/sendOTP/                  |
| Validate OTP              | `validateOTP`              | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/validateOTP/              |
| Provision All             | `provisionAll`             | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/provisionAll/             |
| Inquire All               | `inquireAll`               | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/inquireAll/               |
| Refund All                | `refundAll`                | https://tpay-test.turkcell.com.tr:443/tpay/provision/services/restful/getCardToken/refundAll/                |

### Payment Management

| Service                 | Method               | Test URL                                                                  |
| ----------------------- | -------------------- | ------------------------------------------------------------------------- |
| Get Card Token (Secure) | `getCardTokenSecure` | https://omccstb.turkcell.com.tr/paymentmanagement/rest/getCardTokenSecure |
| 3D Secure Redirect      | `threeDSecure`       | https://omccstb.turkcell.com.tr/paymentmanagement/rest/threeDSecure       |

---

## Production Environment Endpoints

### Provision Services

| Service                   | Method                     | Production URL                                                                                      |
| ------------------------- | -------------------------- | --------------------------------------------------------------------------------------------------- |
| Get Cards                 | `getCards`                 | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getCards/                 |
| Register Card             | `registerCard`             | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/registerCard/             |
| Update Card               | `updateCard`               | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/updateCard/               |
| Delete Card               | `deleteCard`               | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/deleteCard/               |
| Provision                 | `provision`                | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/provision/                |
| Inquire                   | `inquire`                  | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/inquire/                  |
| Reverse                   | `reverse`                  | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/reverse/                  |
| Refund                    | `refund`                   | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/refund/                   |
| Summary Reconciliation    | `summaryReconciliation`    | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/summaryReconciliation/    |
| Get 3D Session            | `getThreeDSession`         | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getThreeDSession/         |
| Get 3D Session Result     | `getThreeDSessionResult`   | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getThreeDSessionResult/   |
| Get Provision History     | `getProvisionHistory`      | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getProvisionHistory       |
| Provision for Marketplace | `provisionForMarketPlace`  | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/provisionForMarketPlace/  |
| Get Terms of Service      | `getTermsOfServiceContent` | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getTermsOfServiceContent/ |
| Get Card BIN Information  | `getCardBinInformation`    | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getCardBinInformation/    |
| Get Payment Methods       | `getPaymentMethods`        | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getPaymentMethods/        |
| Open Mobile Payment       | `openMobilePayment`        | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/openMobilePayment/        |
| Send OTP                  | `sendOTP`                  | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/sendOTP/                  |
| Validate OTP              | `validateOTP`              | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/validateOTP/              |
| Provision All             | `provisionAll`             | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/provisionAll/             |
| Inquire All               | `inquireAll`               | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/inquireAll/               |
| Refund All                | `refundAll`                | https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/refundAll/                |

### Payment Management

| Service                 | Method               | Production URL                                                             |
| ----------------------- | -------------------- | -------------------------------------------------------------------------- |
| Get Card Token (Secure) | `getCardTokenSecure` | https://epayment.turkcell.com.tr/paymentmanagement/rest/getCardTokenSecure |
| 3D Secure Redirect      | `threeDSecure`       | https://epayment.turkcell.com.tr/paymentmanagement/rest/threeDSecure       |

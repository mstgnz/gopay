# Paycell API Request & Response Examples

This document provides comprehensive examples for all Paycell API endpoints.

## Card Management

### getCards

**Request Example:**

```json
{
  "msisdn": "5380521479",
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181017103525997",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "eulaId": "16",
  "cardList": [
    {
      "cardId": "bd142ddd-9fa4-46a6-8024-895500274826",
      "maskedCardNo": "402277******4026",
      "alias": "CARDFINANS**26",
      "cardBrand": "CARDFINANS",
      "isDefault": true,
      "isExpired": false,
      "showEulaId": false,
      "isThreeDValidated": true,
      "isOTPValidated": false,
      "activationDate": "2018-10-10 10:06:29"
    },
    {
      "cardId": "4b20f20c-945b-46b7-8538-4ea22b5c8c79",
      "maskedCardNo": "545616******5454",
      "alias": "CARDFINANS**54",
      "cardBrand": "CARDFINANS",
      "isDefault": false,
      "isExpired": false,
      "showEulaId": false,
      "isThreeDValidated": true,
      "isOTPValidated": false,
      "activationDate": "2018-10-16 19:15:27"
    }
  ]
}
```

### registerCard

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "alias": "3dlicard",
  "cardToken": "0a2c1212-4107-4eee-8b4d-22ff984ee46b",
  "eulaId": "16",
  "isDefault": "true",
  "msisdn": "905322870886",
  "threeDSessionId": "f547e92e-0675-4329-90ba-a85bea0dbaf7"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181018101442892",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "cardId": "f97715a8-77db-4d3f-9dd6-497c87be82ed"
}
```

### updateCard

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309112423228",
    "transactionId": "12345678901234567891"
  },
  "alias": "YKBTest",
  "cardId": "01525e7d-e6c6-448d-aaab-cca5339f24c6",
  "eulaId": "16",
  "isDefault": " ",
  "msisdn": "905591111112",
  "threeDSessionId": " "
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567891",
    "responseDateTime": "20181017141035079",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null
}
```

### deleteCard

**Request Example:**

```json
{
  "cardId": "40760eb6-0941-4c5b-a4c0-3b02cfe4fdac",
  "msisdn": "905599999969",
  "requestHeader": {
    "applicationName": " XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "91.93.156.6",
    "transactionDateTime": "20181002182828017",
    "transactionId": "00000000000002187144"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "00000000000002187144",
    "responseDateTime": "20181017155605211",
    "responseCode": "0",
    "responseDescription": "Success"
  }
}
```

## Payment Operations

### provision

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "cardId": "bd142ddd-9fa4-46a6-8024-895500274826",
  "merchantCode": "2003",
  "msisdn": "5380521479",
  "referenceNumber": "12333374401234567892",
  "amount": "2351",
  "paymentType": "SALE",
  "acquirerBankCode": "111",
  "threeDSessionId": " "
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181017104734492",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "551952788718101710473129",
  "acquirerBankCode": "111",
  "issuerBankCode": "111",
  "approvalCode": "646174",
  "reconciliationDate": "20181017"
}
```

### inquire

**Request Example:**

```json
{
  "merchantCode": "2001",
  "msisdn": "905591111112",
  "originalReferenceNumber": "12345678901234567891",
  "referenceNumber": "12345678901234567892",
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309112423228",
    "transactionId": "12345678901234567890"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567890",
    "responseDateTime": "20181017131815713",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "orderId": "926197750916112311250814",
  "acquirerBankCode": "046",
  "status": "REVERSE",
  "provisionList": [
    {
      "provisionType": "REVERSE",
      "transactionId": "12345678901234567890",
      "amount": "2351",
      "approvalCode": "478341",
      "dateTime": "20161123125420528",
      "reconciliationDate": "20161123",
      "responseCode": "",
      "responseDescription": ""
    },
    {
      "provisionType": "SALE",
      "transactionId": "12345678901234567890",
      "amount": "2351",
      "approvalCode": "478341",
      "dateTime": "20161123112507261",
      "reconciliationDate": "20161123",
      "responseCode": "",
      "responseDescription": ""
    }
  ]
}
```

### reverse

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "cardId": "e14fa3bc-82df-4086-bae2-664b77ae8692",
  "merchantCode": "9998",
  "msisdn": "5380521479",
  "referenceNumber": "12333374401234666892",
  "originalReferenceNumber": "12333374401234667882",
  "amount": "2351",
  "paymentType": "SALE",
  "acquirerBankCode": "111",
  "threeDSessionId": " "
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181101101959745",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "reconciliationDate": "20181101",
  "approvalCode": "575533",
  "retryStatusCode": null,
  "retryStatusDescription": null
}
```

### refund

**Request Example:**

```json
{
  "amount": "1000",
  "merchantCode": "2003",
  "msisdn": "905599999969",
  "originalReferenceNumber": "12319332200000000000",
  "referenceNumber": "1263000000000000933",
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "91.93.156.6",
    "transactionDateTime": "20181002182828017",
    "transactionId": "00000000000002187144"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "00000000000002187144",
    "responseDateTime": "20181017143240726",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "reconciliationDate": "20181017",
  "approvalCode": "820013",
  "retryStatusCode": null,
  "retryStatusDescription": null
}
```

### summaryReconciliation

**Request Example:**

```json
{
  "merchantCode": "2003",
  "reconciliationDate": "20160404",
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "12312312",
    "transactionDateTime": "20160309112423228",
    "transactionId": "12345678901234567890"
  },
  "totalRefundAmount": "10",
  "totalRefundCount": "1",
  "totalReverseAmount": "0",
  "totalReverseCount": "0",
  "totalSaleAmount": "152080",
  "totalSaleCount": "5"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567890",
    "responseDateTime": "20181017134955617",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "reconciliationResult": "NOK",
  "reconciliationDate": "20160404",
  "totalSaleAmount": "0",
  "totalReverseAmount": "0",
  "totalRefundAmount": "0",
  "totalPreAuthAmount": "0",
  "totalPostAuthAmount": "0",
  "totalPreAuthReverseAmount": "0",
  "totalPostAuthReverseAmount": "0",
  "totalSaleCount": 0,
  "totalReverseCount": 0,
  "totalRefundCount": 0,
  "totalPreAuthCount": 0,
  "totalPostAuthCount": 0,
  "totalPreAuthReverseCount": 0,
  "totalPostAuthReverseCount": 0,
  "extraParameters": null
}
```

### getThreeDSession

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "amount": "2351",
  "cardToken": "8c0e9491-9149-4b8e-9d99-a40669486ebd",
  "installmentCount": 1,
  "merchantCode": "2005",
  "msisdn": "5380521479",
  "referenceNumber": " ",
  "target": "MERCHANT",
  "transactionType": "AUTH"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181018085925250",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "threeDSessionId": "fa3d0e81-e9c8-4329-b0b7-5837c332c71e"
}
```

### getThreeDSessionResult

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567890"
  },
  "merchantCode": "2005",
  "msisdn": "5380521479",
  "referenceNumber": " ",
  "threeDSessionId": "4e215e3a-ebe7-4800-b44c-21ba789fe3d5"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567890",
    "responseDateTime": "20181031083214159",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "currentStep": "3",
  "mdErrorMessage": "Authenticated",
  "mdStatus": "1",
  "threeDOperationResult": {
    "threeDResult": "0",
    "threeDResultDescription": "3D Dogrulama basarili"
  }
}
```

### provisionForMarketPlace

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309112423230",
    "transactionId": "12345678901234567890"
  },
  "acquirerBankCode": "046",
  "amount": "2351",
  "cardId": "813256de-a1a7-4988-b2d0-33282fcedd98",
  "currency": "TRY",
  "merchantCode": "2003",
  "msisdn": "905591111112",
  "paymentType": "SALE",
  "referenceNumber": "12345678961334373593"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567890",
    "responseDateTime": "20190130214033491",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "042802072919013021403141",
  "acquirerBankCode": "046",
  "issuerBankCode": "046",
  "approvalCode": "205876",
  "reconciliationDate": "20190130",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

### getProvisionHistory

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "merchantCode": "2003",
  "partitionNo": "1",
  "reconciliationDate": "20181018"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20181030220749159",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "nextPartitionNo": 2,
  "transactionList": [
    {
      "transactionDateTime": "20181018103818666",
      "orderId": "103422357118101810382015",
      "acquirerBankCode": "111",
      "approvalCode": "537935",
      "issuerBankCode": "111",
      "amount": 23.51,
      "netAmount": 23.51,
      "referenceNumber": "12333374409823467891",
      "transactionId": "12345678901234567893",
      "transactionParams": []
    },
    {
      "transactionDateTime": "20181018104734870",
      "orderId": "716242318518101810473518",
      "acquirerBankCode": "111",
      "approvalCode": "644171",
      "issuerBankCode": "111",
      "amount": 23.51,
      "netAmount": 23.51,
      "referenceNumber": "12333374409823468891",
      "transactionId": "12345678901234567893",
      "transactionParams": []
    }
  ]
}
```

### getTermsOfServiceContent

**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "11.111.111.111",
    "transactionDateTime": "20161125121627734",
    "transactionId": "12345678901234566808"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234566808",
    "responseDateTime": "20190128093135788",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "eulaId": 16,
  "termsOfServiceHtmlContentTR": "Sözleşme içeriğinin Türkçe versiyonu termsOfServiceHtmlContentEN: Sözleşmeiçeriğininİngilizceversiyonu"
}
```

### getCardBinInformationWithBKM

rest url : https://tpay-test.turkcell.com.tr/tpay/provision/services/restful/getCardToken/getCardBinInformationWithBKM

**Request Example:** :

```json
{
  "requestHeader": {
    "applicationName": "****",
    "applicationPwd": "****",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "binValue": "",
  "merchantCode": "****",
  "getType": "ALL"
}
```

**Response Example:** :

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20240606144934384",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "cardBinInformations": [
    {
      "bankCode": "067",
      "bankName": "YAPI KREDI BANKASI",
      "binRangeMax": 49213099,
      "binRangeMin": 49213000,
      "cardBrand": "WORLD",
      "cardOrganization": "VISA",
      "cardType": "Credit Card",
      "commercialType": "BIREYSEL",
      "typeId": 1
    }
  ]
}
```

Müşterilerin kart numarasının ilk 6 ya da 8 hanesinden oluşan bin bilgisi veya kart token bilgisi girilerek ya da kayıtlı tüm kartların listelenebileceği ALL seçeneği kullanılarak kart bilgilerinin görüntülenmesini sağlayan servistir. Kartın bankası ve kartın özelliklerini, bin aralığı verisi ile döner.

Request Parameters

Field Format Length (O)ptional/(M)andatory Description
token String 400 M getCardTokenSecure servisi çağrılarak elde edilen token değeridir. Bin.Value ya da GetType gönderildiğinde null gönderilebilir.
binValue String 19 M Kart numarasının ilk 6 ya da 8hanesidir. Token ya da GetType gönderildiğinde null gönderilebilir.
getType String 3 M ALL olarak gönderildiğinde tüm kayıtlı tüm kartların bilgileri listelenir. Token ya da BinValue gönderildiğinde null gönderilebilir.

Response Parameters

List Name Field Format Length (O)ptional/(M)andatory Description
responseHeader transactionId String M Transaction’a ait id bilgisidir.
responseHeader responseDateTime String M İşlemin tarih bilgisi
responseHeader responseCode String M Hata kodudur. 0 ise başarıldır, diğer tüm kodlarda hatalıdır.
responseHeader responseDescription String M Response code’a ait açıklamadır.
extraParameters key String O Optional olarak iletilen veriye ait label bilgisi
extraParameters value String O Optional olarak iletilen veriye ait değer bilgisi
cardBinInformations bankCode String 3 M Kart banka kodu
cardBinInformations bankName String 50 M Kart bankasının adı
cardBinInformations binRangeMax String 16 M Kart bilgilerine sahip bin aralığının max değeri
cardBinInformations binRangeMin String 16 M Kart bilgilerine sahip bin aralığının min değeri
cardBinInformations cardBrand String 16 M Kart brandi
cardBinInformations cardOrganization String 16 M Kart organizasyonu
cardBinInformations cardType String 16 M Kart tipi (Kredi kartı/debit)
cardBinInformations commercialType String 16 M Kartın ticari tipi
cardBinInformations typeId String 1 M Kart tipi id değeri

getPaymentMethods
ımage
Müşterinin Paycell’de tanımlı olan kartları (kredi/debit/prepaid/PAYE yemek kartı) ve mobil ödeme(faturana yansıt) yöntemi olmak üzere kullanabileceği ödeme yöntemlerinin sorgulanıp listelenmesi amacıyla kullanılır. Müşterinin, üye işyerinin uygulama ekranında ilk kez Paycell’de tanımlı kartlarının ve mobil ödemesinin sorgulanması durumunda öncelikle müşterinin veri paylaşım iznini uygulama üzerinde vermiş olması gerekmektedir. Müşterinin ödeme yöntemlerinin listelenmesine yönelik verdiği izin üye işyeri uygulamasında tutulmalıdır.
requestParameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
responseParameters

Field Format Length (O)ptional/(M)andatory Description
eulaID String 20 M
Müşterinin tanımlı kart bilgisinden bağımsız olarak Paycell sisteminde güncel olan kart sözleşme numarası bilgisini döner:

1.Yeni kart ekleme senaryosunda müşteriye gösterilmesi gereken sözleşme metni numarası gösterilir, bu ID registerCard methodu’nda input olarak kullanılır.

2. Mevcut tanımlı kartlar için ise showEulaId = true ise ilgili kart için müşteriye gösterilmesi gereken sözleşme metni numarası gösterilir. Sözleşme kabul edildikten sonra updateCard requesti ile sözleşme bilgisi güncellenir.

cardList Array O Müşterinin Paycell’de tanımlı kartları bulunması durumunda kart bilgileri liste olarak iletilir, tanımlı kart bulunmuyorsa boş dönülür.
mobilePayment Array O Müşterinin Mobil Ödeme sisteminde kaydı bulunuyorsa mobil ödeme bilgileri iletilir. Müşterinin Mobil Ödeme sisteminde kaydı yoksa boş dönülür.
cardList

Field Format Length (O)ptional/(M)andatory Description
cardId String 36 M Kart bilgisine ilişkin uygulamaya özel kart referans numarasıdır, tanımlı kart için yapılan işlemlerde bu değer ilgili method’larda input olarak kullanılır.
maskedCardNo String 16 M Kartın ilk 6 ve son 4 hanesi açık, aradaki değerler \*’lı olarak maskelenmiş numarasıdır.
alias String 20 O Müşteri tarafından kart ekleme veya kart güncelleme aşamasında belirlenen kartını ayırt etmeye yarayan tanım bilgisidir.
cardBrand String 20 O
Kartın BIN bilgisinden elde edilen Paycell’de tutulan marka bilgisidir."

cardBrand": "PAYE", olan kartlar PAYE yemek kartlarıdır.

isDefault Boolean M Kart "varsayılan kart olarak" tanımlı ise true, diğer durumda "false" dönülür; default kart bilgisi, müşteri tarafından kart ekleme veya kart güncelleme işlemi esnasında değiştirilebilir. Kullanım alanı örneği olarak, uygulama ödeme yapılacak kartları listelerken kartlar arasında bu değerin "true" olduğu kartı seçili olarak gösterebilir.
isExpired Boolean M Paycell’de tanımlı olan kartın son kullanım tarihi geçmiş ise "true", diğer durumda "false" dönülür. True olarak dönülen kart için uygulama ekranında kartın son kullanım tarihi geçtiğine dair bilgilendirme mesajı verilebilir, bu kart ödeme işlemlerinde kullanılamaz, kartın silinip yeni kartın eklenmesi gerekmektedir.
showEulaId Boolean M Paycell’de tanımlı olan kart güncel Paycell sözleşme numarasına sahip değilse "true", diğer durumda "false" dönülür. "True" olarak iletilen bir kart için ödeme işlemi yapılmak isteniyorsa öncelikle müşteriye bu kart için güncel sözleşme metni uygulamada gösterilip, güncel eulaID değeri için updateCard method’u ile kart bilgisi güncellenmelidir, güncel sözleşme bilgisine sahip olmayan bir kart için (showEulaId=true) işlem gönderilmemelidir.
activationDate String M Kartın Paycell’de tanımlandığı tarih bilgisidir.
isThreeDValidated Boolean M Kart Paycell’e 3D doğrulama yöntemi ile tanımlandı ise veya Paycell üzerinden 3D doğrulama yöntemi ile bir ödeme işlemi yapıldı ise "true", diğer durumda "false" dönülür.
isOTPValidated Boolean M İleride kullanılmak üzere ayrılmıştır, şu an için "false" dönülmektedir. Kartın OTP yöntemi ile doğrulanıp doğrulanmadığı bilgisidir.
mobilePayment

Field Format Length (O)ptional/(M)andatory Description
isDcbOpen Boolean M Müşterinin mobil ödemesi açık ise "true", değilse "false" dönülür. Bu alanın "false" dönülmesi halinde üye işyeri bir checkbox ile müşteriye mobil ödemesinin açmak isteyip istemediğini sorabilir.
isEulaExpired Boolean M Müşterinin daha önceden imzaladığı bir mobil ödeme sözleşmesi varsa ve bu sözleşme güncel değilse "true", güncelse "false" dönülür.
eulaId String 20 O Müşterinin mobil ödeme sözleşmesi güncel değilse bu alanda güncel sözleşme versiyonu numarası dönülür.
eulaUrl String 20 O Müşterinin mobil ödeme sözleşmesi güncel değilse bu alanda verilecek link ile müşteri güncel sözleşmeye yönlendirilir.
signedEulaId String 20 O Müşterinin en son imzaladığı mobil ödeme sözleşmesinin versiyonunu gösterir. Eğer müşterinin daha önceden imzaladığı bir sözleşme yok ise boş dönülür.
limit String 20 M Müşterinin aylık kullanabileceği mobil ödeme limitidir. Son iki hanesi küsüratı belirtir.
maxLimit String 20 M Müşterinin mobil ödeme işlemi için kullanabileceği maksimum limittir. Son iki hanesi küsüratı belirtir.
remainingLimit String 20 M Müşterinin mobil ödeme yapmak için kullanabileceği kalan limitini gösterir. Son iki hanesi küsüratı belirtir.
statementDate String 20 M Müşterinin mobil ödemeye kaydolup sözleşme imzaladıktan sonra sözleşmesinin aktifleneceği tarihtir.

**Request Example:**

```json
{
  "msisdn": "5305289290",
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  }
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20201121222034021",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "eulaId": "16",
  "cardList": [
    {
      "cardId": "6f08f86f-0343-4e9e-a3e2-656a3facf79b",
      "maskedCardNo": "454671******7894",
      "alias": "MAXIMUM",
      "cardBrand": "MAXIMUM",
      "isDefault": true,
      "isExpired": false,
      "showEulaId": false,
      "isThreeDValidated": true,
      "isOTPValidated": false,
      "activationDate": "2020-11-02 22:08:55",
      "cardType": "Credit"
    },
    {
      "cardId": "576a99b7-0cc3-4ecd-8e06-3613013e14a5",
      "maskedCardNo": "700001******3173",
      "alias": "payekart",
      "cardBrand": "PAYE",
      "isDefault": false,
      "isExpired": false,
      "showEulaId": false,
      "isThreeDValidated": false,
      "isOTPValidated": false,
      "activationDate": "2020-11-17 12:42:28",
      "cardType": "Debit"
    }
  ],
  "mobilePayment": {
    "remainingLimit": "70164",
    "limit": "75000",
    "maxLimit": "75000",
    "isDcbOpen": true,
    "statementDate": "20201125",
    "isEulaExpired": true,
    "signedEulaId": "188",
    "eulaId": "188",
    "eulaUrl": null
  }
}
```

### openMobilePayment

Müşteri ödeme yöntemlerini listeledikten sonra mobil ödemesinin kapalı olduğu bilgisi geldiğinde merchant müşteriye bir checkbox ile mobil ödemeyi aç seçeneği sunabilir. Yetkili merchantlara kart kayıt sözleşmesi ile birlikte mobil ödeme sözleşmesi de verilecektir. Müşterinin bu sözleşmeler için tek bir checkbox ile Paycell kart saklama sözleşmesi ve mobil ödeme sözleşmesini kabul ettiğine dair sözleşme onayı vermesi gerekir.
Müşteri bu ekranda mobil ödemesini açma seçeneğini işaretler ve merchantın yine checkboxta sunacağı mobil ödeme sözleşmesini onaylar ise merchant openMobilePayment servisini çağıracaktır. Bu servis ile müşterinin mobil ödemesi kullanıma açılabilecektir.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
eulaId String 20 O getPaymentMethods servisinde isEulaExpired alanı "true" dönmüşse bu alana güncel sözleşme numarası girilir, "false" dönmüşse bu alan kullanılmaz.
**Request Example:** (Sözleşmesi Güncel Olmayan Müşteri İçin)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5320612543",
  "eulaId": "185"
}
```

**Response Example:** (Sözleşmesi Güncel Olmayan Müşteri İçin)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526181351531",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null
}
```

**Request Example:** (Sözleşmesi Güncel Olan Müşteri İçin)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5325628808"
}
```

**Response Example:** (Sözleşmesi Güncel Olan Müşteri İçin)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526184658307",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null
}
```

## OTP

### sendOTP

Müşteri mobil ödeme yöntemiyle ödeme yapmak istediğinde, ödemeyi başlatmadan önce OTP ile doğrulama yapacaktır. sendOTP servisi tarafından müşteriye ilgili ödemeyi doğrulaması için SMS ile bir şifre gönderilecektir.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
amount String 20 M Mobil ödeme yapılacak işleme ait tutardır.Son iki hanesi küsürat olacak şekildedir.
referenceNumber String 20 M Üye işyeri uygulaması tarafından üretilecek unique numerik işlem referans numarası değeridir.
Response Parameters

Field Format Length (O)ptional/(M)andatory Description
token String 36 M Her şifre için unique olarak üretilen doğrulama değeri
expireDate String 17 M Şifrenin parametrik olarak ayarlanabilen geçerlilik süresidir. YYMMddHHmmssSSS formatındadır.
**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5332149727",
  "amount": "1",
  "referenceNumber": "1234567891234567800"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190524171455059",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "token": "4bba3893-dce6-4d44-a264-1a6429cf7a83",
  "expireDate": 20190524171754513
}
```

### validateOTP

sendOTP ile gönderilen şifrenin doğrulanması için kullanılacak servistir.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
amount String 20 M Mobil ödeme yapılacak işleme ait tutardır.Son iki hanesi küsürat olacak şekildedir.
referenceNumber String 20 M Üye işyeri uygulaması tarafından üretilecek unique numerik işlem referans numarası değeridir.
otp String 4 M sendOTP servisininçağrılması sonucu müşteriye SMS olarak gönderilen, işleme özel unique şifre
token String 36 M sendOTP servisinin response olarak döndüğü, bir otp ve referenceNumber’a özel token değeri
Response Parameters

Field Format Length (O)ptional/(M)andatory Description
remainingRetryCount String 1 O OTP’nin başarılı şekilde girilebilmesi için geriye kalan deneme hakkını belirten sayıdır. OTP başarılı şekilde girildiyse boş olarak kalır, başarısızsa kalan hak sayısını gösterir.
**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5332100016",
  "amount": "1",
  "otp": "1769",
  "token": "58a63efa-7688-4fd6-8bee-a45051f611d4",
  "referenceNumber": "1234567891234567810"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526202332720",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "remainingRetryCount": null
}
```

### provisionAll

Ödeme isteklerinin Paycell’e iletilmesi amacıyla kullanılır.
provisionAll servisi ile;

Paycell’de tanımlı olan bir kart kullanılarak (kredi/debit/prepaid/PAYE yemek kartı ile)
Kart numarası girilerek (kredi/debit/prepaid kartlar ile)
Mobil ödeme yöntemi(faturama yansıt) seçilerek
ödeme yapılabilir.
Ödeme alternatifleri ve serviste iletilecek cardId ve cardToken kullanımları aşağıdaki şekildeki gibidir.
Kayıtlı kart ile cvc girilmeden:
Sadece cardId gönderilir.PAYE yemek kartı ile ödeme seçeneği sadece kartın PAYCELL cüzdana kayıtlı olması durumunda kullanılabilmektedir.
Kayıtlı kart ile cvc girilerek:
cardId ve sadece cvc’nin input olarak gönderildiği getCardTokenSecure servisi çağrılarak elde edilen cardToken parametreleri iletilir.
Kredi kartı numarası ve son kullanım tarihi girilerek:
Kart numarası ve son kullanım tarihi’nin input olarak gönderildiği getCardTokenSecure servisi çağrılarak elde edilen cardToken parametresi iletilir.
Kredi kartı numarası, son kullanım tarihi, cvc bilgisi girilerek:
Kart numarası, son kullanım tarihi ve cvc’nin input olarak gönderildiği getCardTokenSecure servisi çağrılarak elde edilen cardToken parametresi iletilir.
Mobil ödeme seçilerek:
Mobil ödeme yapılacak MSISDN iletilir.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
cardId String 36 O Paycell’de tanımlı kart ile ödeme yapılmak istenmesi durumunda gönderilir.
referenceNumber String 20 M Üye işyeri uygulaması tarafından üretilecek unique numerik işlem referans numarası değeridir.
originalReferenceNumber String 20 O Ön otorizasyon kapama amaçlı paymentType = POSTAUTH işlemi gönderildiğinde kapatılacak olan ön otorizasyon işleminin referenece number değeridir. Diğer işlem tiplerinde gönderilmez.
merchantCode String 19 M Ödeme işleminin başlatıldığı Paycell’de tanımlı üye işyeri kodu bilgisi gönderilir. Entegrasyon sonrasında her tanımlanan yeni üye işyeri için Paycell tarafından merchantCode değeri paylaşılır.
amount String 12 M İşlem tutarıdır.
Son 2 hane KURUŞ’u ifade eder. Virgül kullanılmaz.

Örnekler:
1TL = 100
15,25TL = 1525
currency String 3 M İşlem döviz cinsini belirler.TRY, EUR, USD, vb.
installmentCount Integer 2 O Taksit sayısı bilgisidir. Taksitsiz işlemlerde 1 veya 0 olarak gönderilebilir. Tırnak işareti kullanılmamalıdır.
pointAmount String 12 O İleride kullanılmak üzere ayrılmıştır. Kart puan bilgisidir.
paymentType Enum M Ödeme işlem tipini belirtir, ön otorizasyon uygulaması söz konusu değilse SALE değeri gönderilir[SALE, PREAUTH, POSTAUTH].
PAYE kart ile ödeme sadece SALE ödeme tipini desteklemektedir.
paymentMethodType Enum M Ödeme yöntemi tipini belirtir. Mobil ödeme yapılcaksa "MOBILE_PAYMENT", kartla ödeme yapılacaksa "CREDIT_CARD" gönderilir.
acquirerBankCode String 4 O İleride kullanılmak üzere ayrılmıştır. Sanal Pos bankası kodu iletilir.
pin String 6 O İleride kullanılmak üzere ayrılmıştır. Paycell kullanıcısı PIN değeri iletilir.
threeDSessionId String 36 O Ödeme işleminin 3D doğrulama yöntemi ile yapılması durumunda getThreeDSession servisi cevabında alınan session ID bilgisidir.
cardToken String 36 O Kart numarası girilerek yapılmak istenen ödeme işlemlerinde getCardTokenSecure servisi alınan değer veya kayıtlı kart kullanımında cvc ile ödeme yapılmasına ilişkin getCardTokenSecure servisi ile cvc karşılığında alınan token değeri .
extraParameters Array O Ödeme işlemine ilişkin ek bir parametre değeri opsiyonel olarak iletilebilir.
Eğer işlem Yemek kartı – PAYE ile yapılırsa, ödemelerde ek olarak bu alanlar iletilecektir. Kredi Kartı, Debit ve Paycell Kart, Mobil Ödeme ile yapılan işlemlerde PAYE sahaları olmayacaktır.
Üye işyeri tanımında tüm işlemlerin sabit bir vergi oranıyla PAYE’ye iletilmesi istenirse bu alanlar gönderilmeyebilir.Eğer farklı vergi dilimleri ve tutarlarıyla tamamlanacaksa bu bilgiler zorunludur.Örneğin; %1 ve %8lik vergi diliminde toplam 3 TLlik bir işlem için:mealPer1Tax vergi dilimi %8 için 800mealPer1Amount vergi dilimindeki tutar 1 TL için 100mealPer1Tax vergi dilimi %1 için 100mealPer1Amount vergi dilimindeki tutar 2 TL için 200
Response Parameters

Field Format Length (O)ptional/(M)andatory Description
orderId String 32 O Kartla ödemede banka sisteminde iletilen sipariş numarasıdır. Mobil ödemede ise Mobil Ödeme tarafından iletilen sipariş numarasıdır.
acquirerBankCode String 3 O İşlemde kullanılan kartın bankasının EFT kodu numarasıdır. Mobil ödemede bu alan boş döner.PAYE ile tamamlanan işlemlerde 996 döner.
approvalCodeo String 6 O Banka sisteminden iletilen onay kodudur. Mobil ödemede bu alan boş döner.
reconciliationDate String 8 O İşlemin mutabakatı için PAYCELL sisteminde belirlenen tarih bilgisidir. YYYYMMDD formatında olacaktır.
issuerBankCode String 3 O İşlemde kullanılan kartın bankasının EFT kodu numarasıdır. Mobil ödemede bu alan boş döner.PAYE ile tamamlanan işlemlerde 996 döner.
**Request Example:** (Kart ile Ödeme)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "amount": "1",
  "cardId": "e43d5943-3e39-4f1c-8dc8-a52cc5828849",
  "currency": "TRY",
  "merchantCode": "2005",
  "msisdn": "5332109681",
  "paymentType": "SALE",
  "paymentMethodType": "CREDIT_CARD",
  "referenceNumber": "12333374401234568900"
}
```

**Response Example:** (Kart ile Ödeme)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526222621143",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "957460402819052622261919",
  "acquirerBankCode": "046",
  "issuerBankCode": "046",
  "approvalCode": "311918",
  "reconciliationDate": "20190526",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

**Request Example:** (Mobil Ödeme)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5332109727",
  "amount": "1",
  "currency": "TRY",
  "merchantCode": "1182",
  "paymentMethodType": "MOBILE_PAYMENT",
  "paymentType": "SALE",
  "referenceNumber": "123452106152"
}
```

**Response Example:** (Mobil Ödeme)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190525213956409",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "5131719",
  "acquirerBankCode": null,
  "issuerBankCode": null,
  "approvalCode": null,
  "reconciliationDate": "20190525",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

**Request Example:** (PAYE ile Ödeme)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "11246782101234561119"
  },
  "amount": "100",
  "cardId": "01f7b0dd-eb3f-4241-bbcf-92414aaa2c81",
  "currency": "TRY",
  "merchantCode": "9999",
  "msisdn": "905332108191",
  "paymentType": "SALE",
  "paymentMethodType": "CREDIT_CARD",
  "referenceNumber": "12313372402234568901",
  "extraParameters": [
    {
      "key": "mealPer1Tax",
      "value": "1800"
    },
    {
      "key": "mealPer1Amount",
      "value": "50"
    },
    {
      "key": "mealPer2Tax",
      "value": "100"
    },
    {
      "key": "mealPer2Amount",
      "value": "50"
    }
  ]
}
```

**Response Example:** (PAYE ile Ödeme)

```json
{
  "responseHeader": {
    "transactionId": "11246782101234561119",
    "responseDateTime": "20201121223653796",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "313302652820112122365221",
  "acquirerBankCode": "996",
  "issuerBankCode": "996",
  "approvalCode": null,
  "reconciliationDate": "20201121",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

Ön Otorizasyon – Finansallaştırma (Preauth – Postauth)
Ödeme işlemleri için satış (SALE) dışında PREAUTH (ön otorizasyon) ve POSTAUTH (finansallaştırma) olmak üzere iki farklı işlem tipi mevcuttur.
Eğer ödeme işleminde preauth kullanılıyorsa karttan ön provizyon alınır ve üye işyeri adına, kart limitine provizyon miktarı kadar bloke konulur. Provizyon işlemi işyeri tarafından onaylanmadıkça üye işyeri hesabına yansımaz. Ödemenin alınıp işlemin tamamlanması için preauth işleminin finansallaştırma aşamasına geçmesi gerekir.

Preauth yapılan işlemin finansallaştırılması için ise postauth işlem tipi kullanılır. Preauth aşamasında ön provizyon alınan işlem onaylanır ve finansallaştırma gerçekleştirilir.

Postauth işlemi preauth olmadan yapılamaz.
Postauth tutarı preauth tutarına eşit ya da preauth tutarından az olabilir ancak fazla olamaz.
Postauth işleminde kullanılan değeri kapatılacak olan preauth işleminin reference number değeridir. Diğer işlem tiplerinde (SALE,PREAUTH) bu değer gönderilmez.
Preauth işlemi finansallaştırılmadan iptal edilecekse "reverse" metodu kullanılabilir. Bu durumda işlem tipi "PREAUTH_REVERSE" olarak güncellenir.
Preauth işlemi finansallaştırma sonrasında iptal edilecekse "reverse" metodu kullanılabilir. Bu durumda işlem tipi "POSTAUTH_REVERSE" olarak güncellenir.
Preauth ve postauth işlemleri için reverse metodu çağırılırken, preauth işlemine ait reference number değeri originalReferenceNumber alanında iletilir.
Preauth ve postauth işlemlerinin iadesi (refund) bulunmamaktadır.
**Request Example:** (Preauth)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "amount": "1",
  "cardId": "e43d5943-3e39-4f1c-8dc8-a52cc5828849",
  "currency": "TRY",
  "merchantCode": "2005",
  "msisdn": "5332119826",
  "paymentType": "PREAUTH",
  "paymentMethodType": "CREDIT_CARD",
  "referenceNumber": "12333374401234569900"
}
```

**Response Example:** (Preauth)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526223934353",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "857391542719052622393217",
  "acquirerBankCode": "046",
  "issuerBankCode": "046",
  "approvalCode": "855982",
  "reconciliationDate": "20190526",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

**Request Example:** (Postauth)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "amount": "1",
  "cardId": "e43d5943-3e39-4f1c-8dc8-a52cc5828849",
  "currency": "TRY",
  "merchantCode": "2005",
  "msisdn": "5332119826",
  "paymentType": "POSTAUTH",
  "paymentMethodType": "CREDIT_CARD",
  "originalReferenceNumber": "12333374401234569900",
  "referenceNumber": "12333374401234569911"
}
```

**Response Example:** (Postauth)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526224332669",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "857391542719052622393217",
  "acquirerBankCode": "046",
  "issuerBankCode": "046",
  "approvalCode": "855982",
  "reconciliationDate": "20190526",
  "iyzPaymentId": null,
  "iyzPaymentTransactionId": null
}
```

### inquireAll

Yapılan ödemenin işlem sonucunun sorgulanması amacıyla kullanılır. Provision servisi senkron olarak işlem sonucunu dönmektedir, ancak provision servisine herhangi bir teknik arıza sebebiyle cevap dönülememesi sonrasında işlem timeout’a düştüğünde işlemin sonucu inquire ile sorgulanabilir. inquire servisi yapılan işleme ilişkin işlemin son durumunu ve işlemin tarihçe bilgisini iletir.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
originalReferanceNumber String 20 M Sorgulanacak işlemin "referenceNumber" değeridir.
referanceNumber String 20 M Üye işyeri uygulaması tarafından üretilecek unique numerik işlem referans numarası değeridir. İlk 3 hanesi uygulama bazında unique’dir, bu değer entegrasyon aşamasında Paycell tarafından bildirilecektir.
merchantCode String 19 M Ödeme işleminin başlatıldığı Paycell’de tanımlı üye işyeri kodu bilgisi gönderilir. Entegrasyon sonrasında her tanımlanan yeni üye işyeri için Paycell tarafından merchantCode değeri paylaşılır.
Response Parameters

Field Format Length (O)ptional/(M)andatory Description
orderId String 32 O Banka sisteminde iletilen sipariş numarasıdır.
acquirerBankCode String 3 O İşlemde kullanılan sanal pos bankasının EFT kodu numarasıdır.
status String 12 O İşlemin güncel durumudur:SALE, PREAUTH, POSTAUTH, PREAUTH_REVERSE, POSTAUTH_REVERSE, REVERSE, REFUND
paymentMethodType String 20 O Sorgulanan işlemin hangi ödeme yöntemi ile yapıldığını belirtir. Kartla yapılmış bir işlemse "CREDIT_CARD", mobil ödeme ile yapılmış bir işlemse "MOBILE_PAYMENT" değerini alır.
provisionList Array O İşleme ait tarihçe bilgisi iletilir.
provisionList

Field Format Length (O)ptional/(M)andatory Description
provisionType String 12 O İşlemin tipini belirtir:SALE, PREAUTH, POSTAUTH, PREAUTH_REVERSE, POSTAUTH_REVERSE, REVERSE, REFUND
transactionId String 20 O İlgili işlemin transactionId bilgisidir.
amount String 12 O İlgili işlem tutarıdır.
approvalCode String 6 O Banka sisteminden iletilen onay kodudur.
dateTime String 17 O İlgili işlemin gerçekleşme zamanıdır.
reconciliationDate String 8 O İşlemin mutabakatı için PAYCELL sisteminde belirlenen tarih bilgisidir. YYYYMMDD formatında olacaktır.
responseCode String 20 M İlgili işlemin sonuç bilgisidir.
0: Success, >0: Fail
responseDescription String 200 M İlgili işlemin sonuç açıklama bilgisidir.
**Request Example:** (Kart ile Yapılan İşlemler İçin)

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5332118747",
  "merchantCode": "2005",
  "originalReferenceNumber": "12333374401234568900",
  "referenceNumber": "123452116154"
}
```

**Response Example:** (Kart ile Yapılan İşlemler İçin)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526225105147",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "957460402819052622261919",
  "acquirerBankCode": "046",
  "status": "SALE",
  "paymentMethodType": "CREDIT_CARD",
  "provisionList": [
    {
      "provisionType": "SALE",
      "transactionId": "12345678901234567893",
      "amount": "001",
      "approvalCode": "311918",
      "dateTime": "20190526222621142",
      "reconciliationDate": "20190526",
      "responseCode": "0",
      "responseDescription": "Success"
    }
  ]
}
```

**Request Example:** (Mobil Ödeme ile Yapılan İşlemler İçin)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526225105147",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "957460402819052622261919",
  "acquirerBankCode": "046",
  "status": "SALE",
  "paymentMethodType": "CREDIT_CARD",
  "provisionList": [
    {
      "provisionType": "SALE",
      "transactionId": "12345678901234567893",
      "amount": "001",
      "approvalCode": "311918",
      "dateTime": "20190526222621142",
      "reconciliationDate": "20190526",
      "responseCode": "0",
      "responseDescription": "Success"
    }
  ]
}
```

**Response Example:** (Mobil Ödeme ile Yapılan İşlemler İçin)

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20190526225000167",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "orderId": "5131719",
  "acquirerBankCode": null,
  "status": "REFUND",
  "paymentMethodType": "MOBILE_PAYMENT",
  "provisionList": [
    {
      "provisionType": "REFUND",
      "transactionId": "12345678901234567893",
      "amount": "001",
      "approvalCode": null,
      "dateTime": "20190525214401462",
      "reconciliationDate": "20190525",
      "responseCode": "0",
      "responseDescription": "Success"
    },
    {
      "provisionType": "SALE",
      "transactionId": "0000000000000300433",
      "amount": "001",
      "approvalCode": null,
      "dateTime": "20190525213956408",
      "reconciliationDate": "20190525",
      "responseCode": "0",
      "responseDescription": "Success"
    }
  ]
}
```

### refundAll

Yapılan ödeme işleminin iade edilmesi amacıyla kullanılır.
Kart ile yapılan işlemler için:
İade, işlemin günsonu ardından ertesi günden itibaren iptal edilemesi veya belirli bir tutarın iade edilmesi amacıyla kullanılır. İptal işlemi iki şekilde çağırabilir. Provision servisine cevap alınamayarak timeout alınması durumunda, işlem mutabakatının sağlanması amacıyla şayet günsonu olmuş ise sistem tarafından refund gönderilebilir. Müşterinin iade talebi olması durumunda üye işyeri tarafından manuel olarak çağrılabilir. İade işlemi birden fazla sayıda çağrılabilir, iptal edilmiş bir işlem için iade işlemi gerçekleştirilemez, toplam iade tutarı işlem tutarının üzerinde olamaz.
Mobil ödeme ile yapılan işlemler için:
Mobil ödeme için iptal kurgusu bulunmayıp, tüm işlemler için iade servisi çağrılır. Dolayısıyla işlem için aynı gün içinde de iade servisi çağrılabilir. İade işlemi birden fazla sayıda çağrılabilir, toplam iade tutarı işlem tutarının üstünde olamaz.
Request Parameters

Field Format Length (O)ptional/(M)andatory Description
msisdn String 20 M Müşterinin uygulamaya login olduğu telefon numarası. Ülke kodu + Telefon No formatında iletilir.
originalReferanceNumber String 20 M İade edilecek işlemin "referenceNumber" değeridir.
referanceNumber String 20 M Üye işyeri uygulaması tarafından üretilecek unique numerik işlem referans numarası değeridir. İlk 3 hanesi uygulama bazında unique’dir, bu değer entegrasyon aşamasında Paycell tarafından bildirilecektir.
merchantCode String 19 M Ödeme işleminin başlatıldığı Paycell’de tanımlı üye işyeri kodu bilgisi gönderilir. Entegrasyon sonrasında her tanımlanan yeni üye işyeri için Paycell tarafından merchantCode değeri paylaşılır.
amount string 12 M İade edilmesi istenen işlem tutarıdır. Son 2 hane KURUŞ’u ifade eder. Virgül kullanılmaz.
Örnekler:
1TL = 100
15,25TL = 1525
pointAmount string 12 O İleride kullanılmak üzere ayrılmıştır. İade edilmesi istenen kart puan bilgisidir.
Response Parameters

Field Format Length (O)ptional/(M)andatory Description
approvalCode String 6 O Banka sisteminden iletilen onay kodudur.
reconciliationDate String 8 O İşlemin mutabakatı için PAYCELL sisteminde belirlenen tarih bilgisidir. YYYYMMDD formatında olacaktır.
**Request Example:**

```json
{
  "requestHeader": {
    "applicationName": "XXXX",
    "applicationPwd": "XXXX",
    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",
    "transactionId": "12345678901234567893"
  },
  "msisdn": "5332173763",
  "amount": "1",
  "merchantCode": "1182",
  "originalReferenceNumber": "123452106152",
  "referenceNumber": "123452116153"
}
```

**Response Example:**

```json
{
  "responseHeader": {
    "transactionId": "00000000000002187144",
    "responseDateTime": "20181017143240726",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "reconciliationDate": "20181017",
  "approvalCode": "820013",
  "retryStatusCode": null,
  "retryStatusDescription": null
}
```

### getCardTokenSecure

Kart numarası girilerek yapılan işlemlerde öncelikle kart bilgilerine ait token değeri alınmalıdır. Alınan token değeri, gerçekleştirilmesi istenen işlem tipi için çağrılan servise input olarak eklenmelidir. getCardTokenSecure çağrılarak alınan token değerinin input olarak kullanıldığı işlemler aşağıdaki gibidir.

Kart ekleme 3D doğrulama olmadan: getCardTokenSecure + registerCard

Kart ekleme 3D doğrulama ile: getCardTokenSecure + (getThreeDSession + registerCard)

Kart numarası girilerek yapılan 3D doğrulama olmadan ödeme: getCardTokenSecure + provision

Kart numarası girilerek yapılan 3D doğrulama ile ödeme: getCardTokenSecure + (getThreeDSession + provision)
Servis inputunda yer alan hashdata oluşturulmasında kullanılan parametreler "backend" tarafında tutulmalı ve hashdata "backend" üzerinde oluşturularak uygulama/client’a bildirilmelidir. getCardTokenSecure servisi doğrudan uygulama/client tarafından ilgili parametreler ile çağrılmalıdır.
Implementasyon kullanıcı arayüzü olarak web sayfası kullanıyorsa cross-origin hatasının alınmasınının engellenmesi için üye işyeri domain bilgileri Paycell’e iletilmelidir ve Paycell’de yetki tanımlaması yapılmalıdır. Kullanıcı arayüzü mobil uygulama için herhangi bir tanıma gerek bulunmamaktadır.
requestParameters
requestHeader

Field Format Length (O)ptional/(M)andatory Description
applicationName String 20 M Servisi çağıran uygulamaya özel belirlenmiş kullanıcı adı bilgisidir, entegrasyon aşamasında Paycell tarafından bildirilecektir.
transactionId String 20 M applicationName bazında unique transactionId bilgisidir. Uygulama tarafından üretilir.
transactionDateTime String 17 M YYYYMMddHHmmssSSS formatında işlem zamanı bilgisidir.
requestBody

Field Format Length (O)ptional/(M)andatory Description
creditCardNo String 16 O Kredi kartı numarası
expireDateMonth String 2 O Son kullanma tarihi ay bilgisi
01, 02, …
expireDateYear String 2 O Son kullanma tarihi yıl bilgisi
17, 18, …
cvcNo String 3 O Kartın CVC/CVV değeri
hashData String 50 M
PAYCELL tarafından iletilecek applicationPwd ve secureCode ile input parametreleri hash’lenir.
Hash data oluşturulmasında kullanılacak olan güvenlik parametreleri (applicationName, applicationPwd, secureCode) server tarafında tutulmalıdır, hash oluşturma işlemi server tarafında yapılmalıdır, ancak oluşan değerler uygulama/client tarafında iletilerek getCardTokenSecure servisi uygulama/client tarafından çağrılmalıdır.
hashData 2 aşamada oluşturulacaktır.

Her iki aşamada da ilgili parametreler büyük harfe dönüştürülerek data oluşturulmalıdır.
İlk aşamada securityData hashlenerek oluşturulur. securityData oluşturulurken applicationName ve applicationPwd değeri büyük harfe çevrilir. Oluşan securityData değeri ikinci aşamadaki hashData üretiminde kullanılmak üzere büyük harfe dönüştürülür.
İkinci aşamada, oluşturulan securityData ile diğer değerler büyük harfe çevrilerek birleştirilip elde edilen değer hashlenerek hashData oluşturulur.

securityData: applicationPwd+ applicationName
hashData: applicationName+ transactionId+ transactionDateTime+ secureCode + securityData
Java hash örneği aşağıdaki gibidir.
java.security.MessageDigest sha2 = java.security.MessageDigest.getInstance("SHA-256");
hash = Base64.encodeBase64String(sha2.digest(paramsVal.getBytes()));
;

responseParameters
responseHeader

Field Format Length (O)ptional/(M)andatory Description
transactionId String 20 M Request ile iletilen transactionID
responseDateTime String 17 M YYYYMMddHHmmssSSS
responseCode String 20 M 0: Success, >0: Fail
responseDescription string 200 M İşlem sonuç açıklaması
responseBody

Field Format Length (O)ptional/(M)andatory Description
cardToken String 36 O Alınan tokenize kart bilgisi
hashData String 50 M
responseBody’de dönülen hashData ile üye işyerinin oluşturacağı hashData eşit olmalıdır. Bu kontrol üye işyeri tarafından yapılır.
Üye işyerinin oluşturacağı hashData 2 aşamada oluşturulacaktır. İlk aşamada securityData hashlenerek oluşturulur. İkinci aşamada oluşturulan securityData ile diğer değerler birleştirilerek elde edilen değer hashlenerek hashData oluşturulur.

securityData: applicationPwd+ applicationName

hashData: applicationName+ transactionId+ responseDateTime + responseCode + cardToken + secureCode + securityData
Java hash örneği aşağıdaki gibidir.
java.security.MessageDigest sha2 = java.security.MessageDigest.getInstance("SHA-256");
hash = Base64.encodeBase64String(sha2.digest(paramsVal.getBytes()));

**Request Example:**

{ "header": { "applicationName":"XXXX", "transactionId":"13115080770554206495", "transactionDateTime":"20171015141735420" }, "creditCardNo":"4355084355084358", "expireDateMonth":"12", "expireDateYear":"18", "cvcNo":"", "hashData":"IkpBlQHJgptYSzQCzBJvmVzGEg4teoSbB8VeHe6VVXw=" }
**Response Example:**

{ "header": { "responseCode": "0", "responseDateTime": "20171015141735420", "responseDescription": "Islem basarili", "transactionId": "13115080770554206495" }, "cardToken": "49d55d0e-37ce-4dcc-a2ea-9b452461b157", "hashData": "3Edn4YMafNjwjyWzDp1olbRB3ycJsJn90leS3yH9VLA=" }

queryOrderVpf
ımage

1.  Servis Genel Bilgileri
    Servis istenen tarih aralığında, TPAY servislerinden geçmiş satış ve iade işlemlerine ait finansal verilerin listelenmesi amacıyla kullanılmaktadır.

Servis adı : queryOrderVpf  TPAY Servis url bilgileri :
Test : https://tpay-test.turkcell.com.tr/tpay/provision/services/restful/getCardToken/ queryOrderVpf
PRP : https://tpay-prp.turkcell.com.tr/tpay/provision/services/restful/getCardToken/ queryOrderVpf
Prod : https://tpay.turkcell.com.tr/tpay/provision/services/restful/getCardToken/ queryOrderVpf
Apigw üzerinden erişimde kullanılacak url bilgileri :
Test : https://apigwtst.turkcell.com.tr:8443/tpay/provision/services/restful/getCardToken/ queryOrderVpf
PRP : https://apigwprp.turkcell.com.tr:8443/tpay/provision/services/restful/getCardToken/ queryOrderVpf
Prod : https://apigwmain.turkcell.com.tr:8443/tpay/provision/services/restful/ getCardToken/queryOrderVpf 2. Servis Request Parametreleri
name M/O description
requestHeader applicationName m Servisi çağıran üye işyerine verilen applicationName
applicationPwd m Servisi çağıran üye işyerine verilen application’a ait password
transactionDateTime m
Servisin çağırıldığı tarih saat, örnek data :

20160309084056197

transactionId m Servisin her çağırımında unique olacak şekilde üretilerek gönderilmesi gereken id’dir.
Parameters TRANSACTION_START_DATE m Listelenmek istenen kayıtların tarih aralığına ait başlangıç tarihi
TRANSACTION_END_DATE m Listelenmek istenen kayıtların tarih aralığına ait bitiş tarihi
MERCHANT_CODE m Application’a bağlı, işlem verisi listelenmek istenen Merchant_code bilgisidir.
SIZE m Servis çağırımında listelenmesi istenen kayıt sayısı – Page sayısı size ve total_size göre belirlenir.
PAGE m Servis çağırımında listelenmesi istenen page numarası
BASKET_ID o Opsiyonel olarak provisionAll servisinde veri gönderilmişse, ödeme kaydına bu veri ilişkilendirilir. Bu dokümanda anlatılan serviste transaction için veri döner.
MERCHANT_SPECIAL_DATA o Opsiyonel olarak provisionAll servisinde veri gönderilmişse, ödeme kaydına bu veri ilişkilendirilir. Bu dokümanda anlatılan serviste transaction için veri döner.
MERCHANT_SPECIAL_DATA2 o Opsiyonel olarak provisionAll servisinde veri gönderilmişse, ödeme kaydına bu veri ilişkilendirilir. Bu dokümanda anlatılan serviste transaction için veri döner.
MERCHANT_SPECIAL_DATA3 o Opsiyonel olarak provisionAll servisinde veri gönderilmişse, ödeme kaydına bu veri ilişkilendirilir. Bu dokümanda anlatılan serviste transaction için veri döner.
ORDER_ID O Sorgulanmak istenen işleme ait, provisionAll servis dönüşünde yer alan Order_id bilgisidir. İşleme ait uniqueId’dir.
**Request Example:** :

```json
{
  "requestHeader": {
    "applicationName": "***************",

    "applicationPwd": "******************",

    "clientIPAddress": "10.252.187.81",
    "transactionDateTime": "20160309084056197",

    "transactionId": "12345678901234567893"
  },

  "transactionStartDate": "01-05-2024",

  "transactionEndDate": "30-05-2024",

  "merchantCode": "**************",

  "clientBasketId": "",

  "merchantSpecialData": "",

  "vposOrderId": "",

  "size": "12",

  "page": "0"
}
```

3.  Servis Response Parametreleri
    Name description
    responseHeader responseCode
    0 → success

<> 0 ise servis hata vermiş demektir.

responseDateTime Servisin response tarihi
responseDescription Servis hata mesaj açıklamasıdır
transactionId Servis çağırımına ait transactionId’dir. Listelenen işlemlere ait değil, queryOrderVpf servisinin çağırımına ait id’dir.
Parameters TOTAL_SIZE Sorgulanan tarih aralığındaki toplam kayıt sayısını verir.
CURRENT_PAGE Listelenen page’e ait numarayı verir.
TOTAL_PAGE Requestte gönderilen size ve total_size’a göre toplam kaç sayfada verinin çekileceğini gösterir. Page number 0’dan başlar. Örnek olarak total_page = 3 ise requestte sırasıyla 0, 1, 2 gönderilerek tüm kayıtlar alınabilir.
MERCHANT_LEGAL_NAME Üye işyerinin adı
TRANSACTION_TYPE ödeme tipi alabileceği değerler : SATIS, IADE
Merchant_code requestte gönderilen merhant code dönülür
Sub_merchant_code response’ta sub_dealer_code varsa dönülür
INSTALLMENT_COUNT taksit sayısı
TRANSACTION_DATE işlemin yapıldığı tarih
VALOR_DATE ödemenin yapılacağı tarih
PAYCELL_COMMISSION_RATE komisyon oranı
CALC_COMMISSION_AMOUNT komisyon tutarı
CC_NUMBER İşlem yapılan kartın ilk 4 hanesi
VPOS_ORDER_ID İşleme ait unique_id
CLIENT_BASKET_ID Opsiyonel bir bilgidir, ProvisionAll’da gönderilmiş ise bu serviste döner.
MERCHANT_SPECIAL_DATA Opsiyonel bir bilgidir, ProvisionAll’da gönderilmiş ise bu serviste döner.
MERCHANT_SPECIAL_DATA2 Opsiyonel bir bilgidir, ProvisionAll’da gönderilmiş ise bu serviste döner.
MERCHANT_SPECIAL_DATA3 Opsiyonel bir bilgidir, ProvisionAll’da gönderilmiş ise bu serviste döner.
CARD_TYPE Kartın tipini verir. Credit_Card, Debit_Card vb.
CARD_FAMILY
Kartın organizasyon bilgisini verir. VISA,

MASTERCARD vb.

BOLGE Bu alan opsiyonel olarak kullanılır, işlemin yapıldığı üye işyerine ait detay veri döner.
IBAN_NO Üye işyerinin hak edişinin ödeneceği IBAN bilgisidir.
FOREIGN_CARD İşlem yapılan kartın yabancı kart olup olmadığını döner.
**Response Example:** :

```json
{
  "responseHeader": {
    "transactionId": "12345678901234567893",
    "responseDateTime": "20240813161858955",
    "responseCode": "0",
    "responseDescription": "Success"
  },
  "extraParameters": null,
  "pageInfoResponse": {
    "totalSize": "10",
    "currentPage": "0",
    "totalPage": "5"
  },
  "items": [
    {
      "merchantLegalName": "TÖHAŞ ÖDEME ELEKTRONİK",
      "taxNumber": "8806546623",
      "subMerchantNumber": "201769",
      "terminalNumber": "60416",
      "subDealerCode": "100000511",
      "transactionType": "SATIS",
      "installmentCount": "1",
      "transactionDate": "20-05-2024",
      "valorDate": "31-05-2024",
      "paycellCommissionRate": "1",
      "calcCommissionAmount": "0.15",
      "grossAmount": "15",
      "calcNetAmount": "14.85",
      "ccNumber": "5200",
      "vPosOrderId": "664b2853dd81876c744d3b68",
      "applicationId": "1421",
      "clientBasketId": null,
      "merchantSpecialData": null,
      "merchantSpecialData2": null,
      "merchantSpecialData3": null,
      "cardType": "Credit_Card",
      "cardFamily": "VISA",
      "bolge": null,
      "ibanNo": "TR123456119012345678901276",
      "foreignCard": "0"
    },
    {
      "merchantLegalName": "TÖHAŞ ÖDEME ELEKTRONİK",
      "taxNumber": "8806546623",
      "subMerchantNumber": "201769",
      "terminalNumber": "60416",
      "subDealerCode": null,
      "transactionType": "SATIS",
      "installmentCount": "1",
      "transactionDate": "20-05-2024",
      "valorDate": "31-05-2024",
      "paycellCommissionRate": "1",
      "calcCommissionAmount": "0.2",
      "grossAmount": "20",
      "calcNetAmount": "19.8",
      "ccNumber": "520019",
      "vPosOrderId": "664b4809dd818721d24d3b84",
      "applicationId": "1421",
      "clientBasketId": null,
      "merchantSpecialData": null,
      "merchantSpecialData2": null,
      "merchantSpecialData3": null,
      "cardType": "Credit_Card",
      "cardFamily": "VISA",
      "bolge": null,
      "ibanNo": "TR123456119012345678901276",
      "foreignCard": "0"
    }
  ]
}
```

4.  Hata kodları
    Hata kodu Açıklama TPAY hata dönüşü
    0 Success İşlem Başarılı
    10509 En fazla 90 günlük veri listelenebilir. En fazla 90 günlük veri listelenebilir.
    10507
    Geçersiz Tarih Formatı. Olması gereken dd-

MM-yyyy

Geçersiz Tarih Formatı. Olması gereken dd-

MM-yyyy

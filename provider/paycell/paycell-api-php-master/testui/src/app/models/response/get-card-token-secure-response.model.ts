export interface GetCardTokenSecureResponse {
  header: ResponseHeader,
  cardToken: String,
  hashData: String,
}

export interface ResponseHeader {
  responseCode: String,
  responseDateTime: String,
  responseDescription: String,
  transactionId: String,
}
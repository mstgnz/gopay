import testCredentialConstants from 'src/app/constant/test-credential-constants';
import * as moment from 'moment';

export interface GetCardTokenSecureRequest {
  header?: RequestHeader,
  creditCardNo?: String,
  expireDateMonth?: String,
  expireDateYear?: String,
  cvcNo?: String,
  hashData?: String,
}

export interface RequestHeader {
  applicationName: String,
  transactionId: String,
  transactionDateTime: String,
}

export function createTestRequestHeader(): RequestHeader {
  let transactionDateTime: String = moment().format("YYYYMMDDHHmmssSSS");
  return {
    applicationName: testCredentialConstants.applicationName,
    transactionId: testCredentialConstants.transactionIdPrefix + transactionDateTime,
    transactionDateTime,
  }
}
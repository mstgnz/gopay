import { BaseInput } from './base-input.model';

export interface ValidateOtpRequest extends BaseInput {

  referenceNumber: String,
  msisdn: String,
  amount: String,
  otp: String,
  token: String,

}
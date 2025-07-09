import { BaseInput } from './base-input.model';

export interface SendOtpRequest extends BaseInput {

  msisdn: String,
  amount: String

}
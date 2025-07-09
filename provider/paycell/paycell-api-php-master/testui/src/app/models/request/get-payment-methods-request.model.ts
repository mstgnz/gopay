import { BaseInput } from './base-input.model';

export interface GetPaymentMethodsRequest extends BaseInput {
  //Ülke kodu + Telefon No
  msisdn: String;
}
import { BaseInput } from './base-input.model';

export interface GetCardsRequest extends BaseInput {
  //Ülke kodu + Telefon No
  msisdn: String;
}
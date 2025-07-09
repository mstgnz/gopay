import { BaseInput } from './base-input.model';

export interface GetThreeDSessionRequest extends BaseInput {
  msisdn: String,
  cardId?: String,
  cardToken?: String,
  referenceNumber?: String,
  amount?: String;
  installmentCount?: String,
}